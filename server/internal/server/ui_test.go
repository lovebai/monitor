package server

import (
	"html/template"
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
		Timestamp:  time.Now(),
		OS:         model.OSInfo{UptimeSeconds: 3*86400 + 15*3600},
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
	}{groupNodes(v), v, 0, 500, 0, 0, 80, 80}
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

func TestSysTime(t *testing.T) {
	if got := sysTime(time.Time{}); got != "-" {
		t.Errorf("sysTime(zero) = %q, want %q", got, "-")
	}
	cst := time.FixedZone("CST", 8*3600)
	if got := sysTime(time.Date(2026, 8, 19, 9, 30, 0, 0, cst)); got != "2026-08-19 09:30:00" {
		t.Errorf("sysTime = %q, want %q", got, "2026-08-19 09:30:00")
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
		"<td>nginx</td><td><span class=\"st ok\">● 运行中 ×2（PID 123 124 ）</span></td>",
		"<td>sshd</td><td><span class=\"st bad\">⚠ 未运行</span></td>",
		"进程资源 Top 5", "CPU 占用 Top 5", "内存占用 Top 5",
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
