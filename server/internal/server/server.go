package server

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"monitor-server/internal/model"

	_ "modernc.org/sqlite"
)

type Config struct {
	Token                string
	DatabasePath         string
	OfflineAfter         time.Duration
	LatencyThresholdMS   float64
	MemoryThresholdPct   float64
	DiskThresholdPct     float64
	HistoryRetentionDays int
	AuthEnabled          bool
	AuthUsername         string
	AuthPassword         string
	AgentTokens          map[string]string
}
type Handler struct {
	cfg       Config
	db        *sql.DB
	dashboard *template.Template
	detail    *template.Template
	login     *template.Template
	mu        sync.Mutex
	sessions  map[string]time.Time
}
type nodeView struct {
	model.Report
	Online   bool    `json:"online"`
	Alerts   []alert `json:"alerts"`
	NetRxBps float64 `json:"net_rx_bps"`
	NetTxBps float64 `json:"net_tx_bps"`
}
type alert struct {
	ID        int64     `json:"id"`
	NodeID    string    `json:"node_id"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
type groupView struct {
	Name  string
	Nodes []nodeView
}

// 默认分组DEFAULT,为了方便还是从agent提交吧
func groupNodes(v []nodeView) []groupView {
	byName := map[string][]nodeView{}
	for _, n := range v {
		g := strings.TrimSpace(n.Group)
		if g == "" {
			g = "DEFAULT"
		}
		byName[g] = append(byName[g], n)
	}
	names := make([]string, 0, len(byName))
	for g := range byName {
		names = append(names, g)
	}
	sort.Strings(names)
	out := make([]groupView, 0, len(names))
	for _, g := range names {
		out = append(out, groupView{Name: g, Nodes: byName[g]})
	}
	return out
}

func New(cfg Config) (*Handler, error) {
	if cfg.OfflineAfter <= 0 {
		cfg.OfflineAfter = 90 * time.Second
	}
	if cfg.LatencyThresholdMS <= 0 {
		cfg.LatencyThresholdMS = 500
	}
	if cfg.MemoryThresholdPct <= 0 {
		cfg.MemoryThresholdPct = 80
	}
	if cfg.DiskThresholdPct <= 0 {
		cfg.DiskThresholdPct = 80
	}
	if cfg.HistoryRetentionDays <= 0 {
		cfg.HistoryRetentionDays = 30
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "monitor.db"
	}
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS nodes(node_id TEXT PRIMARY KEY, hostname TEXT NOT NULL, last_seen INTEGER NOT NULL, report_json TEXT NOT NULL); CREATE TABLE IF NOT EXISTS metrics(node_id TEXT NOT NULL, collected_at INTEGER NOT NULL, cpu REAL NOT NULL, memory_percent REAL NOT NULL, disk_percent REAL NOT NULL, latency_ms REAL NOT NULL, rx_rate REAL NOT NULL, tx_rate REAL NOT NULL); CREATE INDEX IF NOT EXISTS idx_metrics_node_time ON metrics(node_id,collected_at); CREATE TABLE IF NOT EXISTS alerts(id INTEGER PRIMARY KEY AUTOINCREMENT,node_id TEXT NOT NULL,kind TEXT NOT NULL,message TEXT NOT NULL,active INTEGER NOT NULL DEFAULT 1,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,resolved_at INTEGER,UNIQUE(node_id,kind)); CREATE INDEX IF NOT EXISTS idx_alerts_active ON alerts(active,updated_at);`); err != nil {
		db.Close()
		return nil, err
	}
	// 数据库含完整资产数据，落盘后收紧权限（Windows 上无实际强制作用，仅尽力而为）。
	if runtime.GOOS != "windows" {
		_ = os.Chmod(cfg.DatabasePath, 0600)
	}
	funcs := template.FuncMap{
		"pct":        percent,
		"ago":        ago,
		"procChecks": func(c []model.Check) []model.Check { return checksByType(c, "process") },
		"svcChecks":  func(c []model.Check) []model.Check { return checksByType(c, "service") },
		"bytes":      humanBytes,
		"rate":       rate,
		"isUp":       isUp,
		"ipv4s":      ipv4s,
		"loadPct":    loadPct,
		"add":        func(a, b int) int { return a + b },
		"dur":        formatUptime,
		"sysTime":    sysTime,
	}
	t := template.Must(template.New("dashboard").Funcs(funcs).Parse(page))
	d := template.Must(template.New("detail").Funcs(funcs).Parse(detailPage2))
	l := template.Must(template.New("login").Parse(loginPage))
	return &Handler{cfg: cfg, db: db, dashboard: t, detail: d, login: l, sessions: map[string]time.Time{}}, nil
}
func (h *Handler) Close() error { return h.db.Close() }

// 还是用gin舒服,不想改了
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AuthEnabled && r.URL.Path != "/api/v1/reports" && r.URL.Path != "/login" && r.URL.Path != "/logout" && !h.isAuthed(r) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/reports":
		h.ingest(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/login":
		h.loginPage(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/login":
		h.handleLogin(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/logout":
		h.logout(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/nodes":
		h.json(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/nodes/") && strings.HasSuffix(r.URL.Path, "/history"):
		h.history(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/nodes/"):
		h.nodeDetail(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/":
		h.home(w, r)
	default:
		http.NotFound(w, r)
	}
}
func (h *Handler) isAuthed(r *http.Request) bool {
	c, err := r.Cookie("session")
	if err != nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	exp, ok := h.sessions[c.Value]
	return ok && time.Now().Before(exp)
}
func (h *Handler) newSession() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	tok := hex.EncodeToString(b)
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[tok] = now.Add(7 * 24 * time.Hour)
	for k, exp := range h.sessions {
		if now.After(exp) {
			delete(h.sessions, k)
		}
	}
	return tok
}
func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if h.isAuthed(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.login.Execute(w, struct{ Err bool }{r.URL.Query().Get("err") != ""})
}
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	u := r.FormValue("username")
	p := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(u), []byte(h.cfg.AuthUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(h.cfg.AuthPassword)) == 1 {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: h.newSession(), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 86400})
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/login?err=1", http.StatusFound)
}
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		h.mu.Lock()
		delete(h.sessions, c.Value)
		h.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}
func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var report model.Report
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&report) != nil || report.NodeID == "" {
		http.Error(w, "invalid report", 400)
		return
	}
	if !h.validAgentToken(report.NodeID, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
		http.Error(w, "unauthorized", 401)
		return
	}
	// The server clock is authoritative for freshness and offline evaluation.
	// This prevents an Agent with an incorrect local clock from looking stale.
	report.Timestamp = time.Now().UTC()
	data, _ := json.Marshal(report)
	_, err := h.db.Exec(`INSERT INTO nodes(node_id,hostname,last_seen,report_json) VALUES(?,?,?,?) ON CONFLICT(node_id) DO UPDATE SET hostname=excluded.hostname,last_seen=excluded.last_seen,report_json=excluded.report_json`, report.NodeID, report.Hostname, report.Timestamp.Unix(), string(data))
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	disk := 0.0
	if len(report.Resources.Disks) > 0 {
		disk = report.Resources.Disks[0].UsedPercent
	}
	rx, tx := netRates(report.Interfaces)
	_, _ = h.db.Exec(`INSERT INTO metrics(node_id,collected_at,cpu,memory_percent,disk_percent,latency_ms,rx_rate,tx_rate) VALUES(?,?,?,?,?,?,?,?)`, report.NodeID, report.Timestamp.Unix(), report.Resources.CPUPercent, percent(report.Resources.MemoryUsedBytes, report.Resources.MemoryTotalBytes), disk, report.Network.LatencyMS, rx, tx)
	h.pruneMetrics(time.Now())
	h.setAlert(report.NodeID, "offline", false, "节点恢复在线")
	high := report.Network.ProbeTarget != "" && !report.Network.Reachable || report.Network.LatencyMS > h.cfg.LatencyThresholdMS
	msg := fmt.Sprintf("网络探测异常：%s", report.Network.Error)
	if report.Network.Reachable {
		msg = fmt.Sprintf("网络延迟 %.0f ms（阈值 %.0f ms）", report.Network.LatencyMS, h.cfg.LatencyThresholdMS)
	}
	h.setAlert(report.NodeID, "latency", high, msg)
	w.WriteHeader(http.StatusNoContent)
}

// validAgentToken 校验 Agent 上报 Token：
// 配置了该 node_id 的独立 Token 时必须与之匹配（Token↔node_id 绑定），
// 未配置独立 Token 的节点回退到全局 Token；比较均为恒定时间，避免时序侧信道。
func (h *Handler) validAgentToken(nodeID, token string) bool {
	if t, ok := h.cfg.AgentTokens[nodeID]; ok && t != "" {
		return subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1
	}
	return subtle.ConstantTimeCompare([]byte(h.cfg.Token), []byte(token)) == 1
}

func (h *Handler) setAlert(node, kind string, active bool, message string) {
	now := time.Now().UTC().Unix()
	if active {
		_, _ = h.db.Exec(`INSERT INTO alerts(node_id,kind,message,active,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(node_id,kind) DO UPDATE SET message=excluded.message,active=1,updated_at=excluded.updated_at,resolved_at=NULL`, node, kind, message, 1, now, now)
	} else {
		_, _ = h.db.Exec(`UPDATE alerts SET active=0,resolved_at=?,updated_at=? WHERE node_id=? AND kind=? AND active=1`, now, now, node, kind)
	}
}

// pruneMetrics 删除超过历史保留天数的 metrics 记录。
func (h *Handler) pruneMetrics(now time.Time) {
	_, _ = h.db.Exec(`DELETE FROM metrics WHERE collected_at < ?`, now.Add(-time.Duration(h.cfg.HistoryRetentionDays)*24*time.Hour).Unix())
}
func (h *Handler) views() []nodeView {
	now := time.Now().UTC()
	rows, e := h.db.Query(`SELECT node_id,last_seen,report_json FROM nodes ORDER BY hostname`)
	if e != nil {
		return nil
	}
	defer rows.Close()
	var out []nodeView
	for rows.Next() {
		var id, data string
		var seen int64
		if rows.Scan(&id, &seen, &data) != nil {
			continue
		}
		var r model.Report
		if json.Unmarshal([]byte(data), &r) != nil {
			continue
		}
		online := now.Sub(time.Unix(seen, 0)) <= h.cfg.OfflineAfter
		if !online {
			h.setAlert(id, "offline", true, fmt.Sprintf("超过 %s 未收到上报", h.cfg.OfflineAfter))
		}
		rx, tx := netRates(r.Interfaces)
		out = append(out, nodeView{Report: r, Online: online, Alerts: h.nodeAlerts(id), NetRxBps: rx, NetTxBps: tx})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hostname < out[j].Hostname })
	return out
}
func (h *Handler) nodeAlerts(id string) []alert {
	rows, e := h.db.Query(`SELECT id,node_id,kind,message,created_at FROM alerts WHERE node_id=? AND active=1 ORDER BY updated_at DESC`, id)
	if e != nil {
		return nil
	}
	defer rows.Close()
	var a []alert
	for rows.Next() {
		var x alert
		var t int64
		if rows.Scan(&x.ID, &x.NodeID, &x.Kind, &x.Message, &t) == nil {
			x.CreatedAt = time.Unix(t, 0)
			a = append(a, x)
		}
	}
	return a
}

// NodeExists 判断指定节点是否已存在于数据库中。
func (h *Handler) NodeExists(id string) (bool, error) {
	var n int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE node_id=?`, id).Scan(&n)
	return n > 0, err
}

// RemoveNode 删除节点及其历史指标与告警记录。
func (h *Handler) RemoveNode(id string) error {
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM alerts WHERE node_id=?`,
		`DELETE FROM metrics WHERE node_id=?`,
		`DELETE FROM nodes WHERE node_id=?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (h *Handler) json(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.views())
}
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	// No-cache headers keep the 5s auto-refresh from serving a stale copy.
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	v := h.views()
	data := struct {
		Groups        []groupView
		Nodes         []nodeView
		AlertCount    int
		Threshold     float64
		RxRate        float64
		TxRate        float64
		MemThreshold  float64
		DiskThreshold float64
	}{groupNodes(v), v, 0, h.cfg.LatencyThresholdMS, 0, 0, h.cfg.MemoryThresholdPct, h.cfg.DiskThresholdPct}
	for _, n := range v {
		data.AlertCount += len(n.Alerts)
		data.RxRate += n.NetRxBps
		data.TxRate += n.NetTxBps
	}
	_ = h.dashboard.Execute(w, data)
}
func (h *Handler) nodeDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/nodes/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	for _, n := range h.views() {
		if n.NodeID == id {
			if !n.Online {
				http.Error(w, "节点已离线，无法查看详情", http.StatusNotFound)
				return
			}
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = h.detail.Execute(w, struct {
				Node          nodeView
				MemThreshold  float64
				DiskThreshold float64
			}{n, h.cfg.MemoryThresholdPct, h.cfg.DiskThresholdPct})
			return
		}
	}
	http.NotFound(w, r)
}

type metricPoint struct {
	At      int64   `json:"at"`
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Disk    float64 `json:"disk"`
	Latency float64 `json:"latency"`
	Rx      float64 `json:"rx"`
	Tx      float64 `json:"tx"`
}

func (h *Handler) points(id string, limit int) []metricPoint {
	rows, e := h.db.Query(`SELECT collected_at,cpu,memory_percent,disk_percent,latency_ms,rx_rate,tx_rate FROM metrics WHERE node_id=? ORDER BY collected_at DESC LIMIT ?`, id, limit)
	if e != nil {
		return nil
	}
	defer rows.Close()
	var p []metricPoint
	for rows.Next() {
		var x metricPoint
		if rows.Scan(&x.At, &x.CPU, &x.Memory, &x.Disk, &x.Latency, &x.Rx, &x.Tx) == nil {
			p = append(p, x)
		}
	}
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return p
}
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/"), "/history")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.points(id, 500))
}
func percent(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}
func rate(bps float64) string {
	switch {
	case bps < 1024:
		return fmt.Sprintf("%.1f B/s", bps)
	case bps < 1024*1024:
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	case bps < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB/s", bps/1024/1024)
	default:
		return fmt.Sprintf("%.1f GB/s", bps/1024/1024/1024)
	}
}
func netRates(ifs []model.NetworkInterface) (rx, tx float64) {
	for _, i := range ifs {
		if strings.Contains(i.Flags, "up") && i.MAC != "" {
			rx += i.RxBytesPerSecond
			tx += i.TxBytesPerSecond
		}
	}
	return
}
func isUp(i model.NetworkInterface) bool {
	return strings.Contains(i.Flags, "up") && i.MAC != ""
}
func ipv4s(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if !strings.Contains(a, ":") {
			out = append(out, a)
		}
	}
	return out
}
func loadPct(load float64, cores int) float64 {
	if cores <= 0 {
		return 0
	}
	return load * 100 / float64(cores)
}
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d 秒前", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
}
func checksByType(c []model.Check, typ string) []model.Check {
	var out []model.Check
	for _, x := range c {
		if x.Type == typ {
			out = append(out, x)
		}
	}
	return out
}
func humanBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := float64(unit), 0
	for n := float64(v) / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/div, "KMGT"[exp])
}

// formatUptime 将秒数格式化为「3天15时」「5时30分」「30分」等可读时长。
func formatUptime(s uint64) string {
	const day = uint64(86400)
	d := s / day
	h := s % day / 3600
	m := s % 3600 / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%d天%d时", d, h)
	case h > 0:
		return fmt.Sprintf("%d时%d分", h, m)
	case m > 0:
		return fmt.Sprintf("%d分", m)
	default:
		return fmt.Sprintf("%d秒", s)
	}
}

// sysTime 返回节点上报的系统时间（Agent 本机时钟），零值返回 "-"。
func sysTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

const loginPage = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>登录 · Server Monitor</title>
<style>
body{margin:0;color:#17233a;font:14px system-ui;background:#edf3fa;display:flex;align-items:center;justify-content:center;min-height:100vh}
.box{background:#fff;border:1px solid #d9e4f0;border-radius:17px;box-shadow:0 5px 20px #5470910b;padding:34px 40px;width:320px}
h1{font-size:19px;margin:0 0 6px}
h1:before{content:'//';color:#2ed5c3;margin-right:8px}
p{color:#6680a5;margin:0 0 20px}
label{color:#647da0;display:block;margin:12px 0 6px}
input{width:100%;box-sizing:border-box;border:1px solid #d5e2f2;border-radius:9px;padding:9px 11px;font:inherit;outline:none}
input:focus{border-color:#2ed5c3}
button{margin-top:22px;width:100%;background:#2ed5c3;border:none;border-radius:9px;color:#fff;font:inherit;font-weight:750;padding:10px;cursor:pointer}
.err{color:#e95169;margin-top:12px;text-align:center}
</style>
</head>
<body>
<form class="box" method="post" action="/login">
<h1>Server Monitor</h1>
<p>请输入登录凭据</p>
<label>用户名</label><input name="username" autocomplete="username" required>
<label>密码</label><input type="password" name="password" autocomplete="current-password" required>
<button type="submit">登录</button>
{{if .Err}}<div class="err">用户名或密码错误</div>{{end}}
</form>
</body>
</html>`
