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
