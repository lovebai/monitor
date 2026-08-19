package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"monitor-server/internal/model"
)

type Config struct {
	Token              string
	DatabasePath       string
	OfflineAfter       time.Duration
	LatencyThresholdMS float64
	MemoryThresholdPct float64
	DiskThresholdPct   float64
}
type Handler struct {
	cfg       Config
	db        *sql.DB
	dashboard *template.Template
	detail    *template.Template
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
	funcs := template.FuncMap{
		"pct":        percent,
		"ago":        ago,
		"checks":     healthyChecks,
		"procChecks": func(c []model.Check) []model.Check { return checksByType(c, "process") },
		"svcChecks":  func(c []model.Check) []model.Check { return checksByType(c, "service") },
		"bytes":      humanBytes,
		"rate":       rate,
		"isUp":       isUp,
		"ipv4s":      ipv4s,
		"loadPct":    loadPct,
		"add":        func(a, b int) int { return a + b },
	}
	t := template.Must(template.New("dashboard").Funcs(funcs).Parse(page))
	d := template.Must(template.New("detail").Funcs(funcs).Parse(detailPage2))
	return &Handler{cfg: cfg, db: db, dashboard: t, detail: d}, nil
}
func (h *Handler) Close() error { return h.db.Close() }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/reports":
		h.ingest(w, r)
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
func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+h.cfg.Token {
		http.Error(w, "unauthorized", 401)
		return
	}
	defer r.Body.Close()
	var report model.Report
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&report) != nil || report.NodeID == "" {
		http.Error(w, "invalid report", 400)
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
	_, _ = h.db.Exec(`DELETE FROM metrics WHERE collected_at < ?`, time.Now().Add(-30*24*time.Hour).Unix())
	h.setAlert(report.NodeID, "offline", false, "节点恢复在线")
	high := report.Network.ProbeTarget != "" && !report.Network.Reachable || report.Network.LatencyMS > h.cfg.LatencyThresholdMS
	msg := fmt.Sprintf("网络探测异常：%s", report.Network.Error)
	if report.Network.Reachable {
		msg = fmt.Sprintf("网络延迟 %.0f ms（阈值 %.0f ms）", report.Network.LatencyMS, h.cfg.LatencyThresholdMS)
	}
	h.setAlert(report.NodeID, "latency", high, msg)
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) setAlert(node, kind string, active bool, message string) {
	now := time.Now().UTC().Unix()
	if active {
		_, _ = h.db.Exec(`INSERT INTO alerts(node_id,kind,message,active,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(node_id,kind) DO UPDATE SET message=excluded.message,active=1,updated_at=excluded.updated_at,resolved_at=NULL`, node, kind, message, 1, now, now)
	} else {
		_, _ = h.db.Exec(`UPDATE alerts SET active=0,resolved_at=?,updated_at=? WHERE node_id=? AND kind=? AND active=1`, now, now, node, kind)
	}
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
	d := time.Since(t).Round(time.Second)
	if d < time.Minute {
		return d.String() + " 前"
	}
	return fmt.Sprintf("%.0f 分钟前", d.Minutes())
}
func healthyChecks(c []model.Check) string {
	if len(c) == 0 {
		return "未配置"
	}
	bad := 0
	for _, x := range c {
		if !x.Healthy {
			bad++
		}
	}
	if bad > 0 {
		return fmt.Sprintf("%d 项异常", bad)
	}
	return "全部正常"
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

const legacyPage = `legacy`
