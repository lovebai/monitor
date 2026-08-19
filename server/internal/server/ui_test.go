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
		"pct": percent, "ago": ago, "checks": healthyChecks, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
	}
	tmpl := template.Must(template.New("page").Funcs(funcs).Parse(page))
	on := nodeView{Report: model.Report{
		NodeID:    "n1",
		Hostname:  "h1",
		Timestamp: time.Now(),
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
}

func TestDetailPageTopProcesses(t *testing.T) {
	funcs := template.FuncMap{
		"pct": percent, "ago": ago, "checks": healthyChecks, "bytes": humanBytes,
		"rate": rate, "isUp": isUp, "ipv4s": ipv4s, "loadPct": loadPct,
		"procChecks": func(c []model.Check) []model.Check { return checksByType(c, "process") },
		"svcChecks":  func(c []model.Check) []model.Check { return checksByType(c, "service") },
		"add":        func(a, b int) int { return a + b },
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
