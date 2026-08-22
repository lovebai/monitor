package server

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"monitor-server/internal/model"
)

func TestHomePageOfflineCardBlocked(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"dur": formatUptime, "sysTime": sysTime,
	}
	tmpl := template.Must(template.New("page").Funcs(funcs).Parse(page))
	on := nodeView{Report: model.Report{
		NodeID:     "n1",
		Hostname:   "h1",
		Alias:      "生产服务器",
		Timestamp:  time.Now(),
		OS:         model.OSInfo{UptimeSeconds: 3*86400 + 15*3600},
		Hardware:   model.Hardware{LogicalCPUs: 4},
		SystemTime: time.Date(2026, 8, 19, 9, 30, 0, 0, time.FixedZone("CST", 8*3600)),
		Resources: model.Resources{
			MemoryUsedBytes:  90 * 1024 * 1024 * 1024,
			MemoryTotalBytes: 100 * 1024 * 1024 * 1024,
			Disks:            []model.Disk{{Mountpoint: "/", UsedPercent: 85}},
		},
	}, Online: true}
	off := nodeView{Report: model.Report{NodeID: "n2", Hostname: "h2", Timestamp: time.Now()}, Online: false}
	v := []nodeView{on, off}
	data := struct {
		Groups        []groupView
		Nodes         []nodeView
		AlertCount    int
		Threshold     float64
		RxRate        float64
		TxRate        float64
		MemThreshold  float64
		DiskThreshold float64
		AuthEnabled   bool
		Server        serverStats
	}{groupNodes(v), v, 0, 500, 0, 0, 80, 80, true, serverStats{Hostname: "srv1", DBPath: "monitor.db", DBFileSize: 2048}}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html := b.String()
	if !strings.Contains(html, `href="/nodes/n1"`) {
		t.Error("online card should keep href to detail page")
	}
	if strings.Contains(html, `href="/nodes/n2"`) {
		t.Error("offline card must not link to detail page")
	}
	if !strings.Contains(html, `class="node off" data-id="n2" title="节点已离线，无法查看详情"`) {
		t.Error("offline card should carry off class and offline title")
	}
	if !strings.Contains(html, `data-role="mem" class="danger">90.0%`) {
		t.Error("memory above threshold should be marked danger")
	}
	if !strings.Contains(html, `data-role="disk" class="danger">85.0%`) {
		t.Error("disk above threshold should be marked danger")
	}
	if !strings.Contains(html, `class="net up" data-role="sys-time">系统时间:　2026-08-19 09:30:00`) {
		t.Error("node card should show agent 系统时间 in light style")
	}
	if !strings.Contains(html, `<span>4 CPU</span><span class="alias">生产服务器</span>`) {
		t.Error("node card chips should end with alias span")
	}
}

func TestFormatUptime(t *testing.T) {
	cases := map[uint64]string{
		0:                        "0秒",
		45:                       "45秒",
		1800:                     "30分",
		5*3600 + 30*60:           "5时30分",
		3*86400 + 15*3600 + 1200: "3天15时",
	}
	for in, want := range cases {
		if got := formatUptime(in); got != want {
			t.Errorf("formatUptime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAgo(t *testing.T) {
	now := time.Now()
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30 秒前"},
		{90 * time.Second, "1 分钟前"},
		{59 * time.Minute, "59 分钟前"},
		{61 * time.Minute, "1 小时前"},
		{23*time.Hour + 30*time.Minute, "23 小时前"},
		{25 * time.Hour, "1 天前"},
		{3*24*time.Hour + 5*time.Hour, "3 天前"},
	}
	for _, c := range cases {
		if got := ago(now.Add(-c.d)); got != c.want {
			t.Errorf("ago(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestSysTime(t *testing.T) {
	if got := sysTime(time.Time{}); got != "-" {
		t.Errorf("sysTime(zero) = %q, want %q", got, "-")
	}
	cst := time.FixedZone("CST", 8*3600)
	if got := sysTime(time.Date(2026, 8, 19, 9, 30, 0, 0, cst)); got != "2026-08-19 09:30:00" {
		t.Errorf("sysTime = %q, want %q", got, "2026-08-19 09:30:00")
	}
}

func TestNodesFragmentRender(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"dur": formatUptime, "sysTime": sysTime,
	}
	tmpl := template.Must(template.New("page").Funcs(funcs).Parse(page))
	on := nodeView{Report: model.Report{NodeID: "n1", Hostname: "h1", Timestamp: time.Now(), OS: model.OSInfo{Name: "linux"}, Hardware: model.Hardware{LogicalCPUs: 4}}, Online: true}
	data := struct {
		Groups        []groupView
		Nodes         []nodeView
		AlertCount    int
		Threshold     float64
		RxRate        float64
		TxRate        float64
		MemThreshold  float64
		DiskThreshold float64
		Server        serverStats
	}{groupNodes([]nodeView{on}), []nodeView{on}, 0, 500, 0, 0, 80, 80, serverStats{Hostname: "srv1"}}

	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "nodes", data); err != nil {
		t.Fatalf("render fragment failed: %v", err)
	}
	html := b.String()
	if !strings.Contains(html, `href="/nodes/n1"`) || !strings.Contains(html, "data-id=\"n1\"") {
		t.Error("fragment should include online node card")
	}
	if !strings.Contains(html, "<span class=\"gcount\">1 台</span>") {
		t.Error("fragment should include group count")
	}

	empty := struct {
		Groups        []groupView
		Nodes         []nodeView
		AlertCount    int
		Threshold     float64
		RxRate        float64
		TxRate        float64
		MemThreshold  float64
		DiskThreshold float64
		Server        serverStats
	}{nil, nil, 0, 500, 0, 0, 80, 80, serverStats{}}
	b.Reset()
	if err := tmpl.ExecuteTemplate(&b, "nodes", empty); err != nil {
		t.Fatalf("render empty fragment failed: %v", err)
	}
	if !strings.Contains(b.String(), "尚未收到 Agent 上报") {
		t.Error("empty fragment should show empty-state message")
	}
}

func TestNodesHTMLFragmentEndpoint(t *testing.T) {
	h := newTestHandlerAuth(t, false)
	body := `{"node_id":"n1","hostname":"h1","group":"web","os":{"name":"linux","architecture":"amd64"},"hardware":{"logical_cpus":4},"resources":{},"interfaces":[],"network":{}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ingest = %d, want 204", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/nodes-html", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("nodes-html = %d, want 200", rr.Code)
	}
	html := rr.Body.String()
	if !strings.Contains(html, `data-id="n1"`) || !strings.Contains(html, `href="/nodes/n1"`) {
		t.Error("nodes-html should include the reported node card")
	}
	if !strings.Contains(html, `data-grp="web"`) {
		t.Error("nodes-html should include the node group title")
	}
}

func TestHomePageLogoutButton(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"dur": formatUptime, "sysTime": sysTime,
	}
	tmpl := template.Must(template.New("page").Funcs(funcs).Parse(page))
	render := func(auth bool) string {
		var b strings.Builder
		data := struct {
			Groups        []groupView
			Nodes         []nodeView
			AlertCount    int
			Threshold     float64
			RxRate        float64
			TxRate        float64
			MemThreshold  float64
			DiskThreshold float64
			AuthEnabled   bool
			Server        serverStats
		}{nil, nil, 0, 500, 0, 0, 80, 80, auth, serverStats{}}
		if err := tmpl.Execute(&b, data); err != nil {
			t.Fatalf("render failed: %v", err)
		}
		return b.String()
	}
	if html := render(true); !strings.Contains(html, "注销") || !strings.Contains(html, `action="/logout"`) {
		t.Error("auth_enabled=true 时主页应显示注销按钮（POST /logout）")
	}
	if html := render(false); strings.Contains(html, "注销") {
		t.Error("auth_enabled=false 时主页不应显示注销按钮")
	}
}

func TestHomePageServerInfoFooter(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"dur": formatUptime, "sysTime": sysTime,
	}
	tmpl := template.Must(template.New("page").Funcs(funcs).Parse(page))
	st := serverStats{
		Hostname: "srv-01", OSName: "windows", Arch: "amd64",
		Load1: 0.55, Load5: 0.48, Load15: 0.42, CPUPercent: 23.4,
		MemUsedBytes: 4 << 30, MemTotalBytes: 8 << 30,
		DiskUsedPct: 62.5, DiskUsedBytes: 100 << 30, DiskTotalBytes: 160 << 30,
		DBFileSize: 12345678, DBPath: "monitor.db",
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "nodes", struct {
		Groups        []groupView
		Nodes         []nodeView
		AlertCount    int
		Threshold     float64
		RxRate        float64
		TxRate        float64
		MemThreshold  float64
		DiskThreshold float64
		Server        serverStats
	}{nil, nil, 0, 500, 0, 0, 80, 80, st}); err != nil {
		t.Fatalf("render footer failed: %v", err)
	}
	html := b.String()
	for _, want := range []string{
		"Server 主机状态",
		"srv-01 · amd64",
		"0.55 / 0.48 / 0.42",
		"23.4%",
		"62.5%",
		"数据库文件",
		"monitor.db",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("server info footer missing %q", want)
		}
	}
	if !strings.Contains(html, `<div class="si-head">Server 主机状态</div>`) {
		t.Error("server info should use single card with si-head title")
	}
	if got := strings.Count(html, `class="si-item"`); got != 6 {
		t.Errorf("si-item count = %d, want 6", got)
	}
}

func TestPagesHaveSVGFavicon(t *testing.T) {
	pages := map[string]string{"dashboard": page, "detail": detailPage2, "login": loginPage}
	for name, p := range pages {
		if !strings.Contains(p, `<link rel="icon" type="image/svg+xml"`) {
			t.Errorf("%s page missing svg favicon link", name)
		}
	}
}

func TestDetailPageTopProcesses(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"procChecks": func(c []model.Check) []model.Check { return checksByType(c, "process") },
		"svcChecks":  func(c []model.Check) []model.Check { return checksByType(c, "service") },
		"add":        func(a, b int) int { return a + b },
		"dur":        formatUptime,
	}
	tmpl := template.Must(template.New("detail").Funcs(funcs).Parse(detailPage2))
	n := nodeView{Report: model.Report{
		NodeID:    "n1",
		Hostname:  "h1",
		Alias:     "生产服务器",
		Timestamp: time.Now(),
		Hardware:  model.Hardware{LogicalCPUs: 4},
		Resources: model.Resources{MemoryTotalBytes: 8 << 30},
		Checks: []model.Check{
			{Type: "process", Name: "nginx", Healthy: true, Detail: "运行中 ×2", PIDs: []int{123, 124}},
			{Type: "service", Name: "sshd", Healthy: false, Detail: "未运行"},
		},
		TopCPU: []model.ProcessStat{
			{Name: "chrome", PID: 100, CPUPercent: 42.5, MemoryBytes: 500 << 20},
			{Name: "java", PID: 200, CPUPercent: 12.3, MemoryBytes: 1 << 30},
		},
		TopMemory: []model.ProcessStat{
			{Name: "java", PID: 200, CPUPercent: 12.3, MemoryBytes: 1 << 30, MemoryPct: 12.5},
			{Name: "chrome", PID: 100, CPUPercent: 42.5, MemoryBytes: 500 << 20, MemoryPct: 6.3},
		},
	}, Online: true}
	var b strings.Builder
	if err := tmpl.Execute(&b, struct {
		Node          nodeView
		MemThreshold  float64
		DiskThreshold float64
	}{n, 80, 80}); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html := b.String()
	for _, want := range []string{
		"进程检查", "服务检查",
		"n1 · 生产服务器 · ",
		"<td>nginx</td><td><span class=\"st ok\">● 运行中 ×2（PID 123 124 ）</span></td>",
		"<td>sshd</td><td><span class=\"st bad\">⚠ 未运行</span></td>",
		"进程资源 Top 5", "CPU 占用 Top 5", "内存占用 Top 5",
		"历史曲线", "hsvg-pct", "hsvg-rate", "hsvg-lat",
		"<td>1</td><td>chrome</td><td>100</td><td>42.5%</td></tr>",
		"<td>2</td><td>java</td><td>200</td><td>12.3%</td></tr>",
		"<td>1</td><td>java</td><td>200</td><td>1.0 GiB（12.5%）</td></tr>",
		"<td>2</td><td>chrome</td><td>100</td><td>500.0 MiB（6.3%）</td></tr>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
	for _, bad := range []string{"<td>42.5%</td><td>", "<td>1.0 GiB（12.5%）</td><td>", "<th>CPU</th><th>内存", "<th>内存</th><th>CPU"} {
		if strings.Contains(html, bad) {
			t.Errorf("detail page should not contain %q (CPU/内存 must stay separate)", bad)
		}
	}
	if strings.Contains(html, "<th>类型</th>") {
		t.Error("detail page should not contain a 类型 column; 进程/服务 must be separate modules")
	}
}
