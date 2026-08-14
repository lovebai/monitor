package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monitor-agent/internal/model"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

type Agent struct {
	cfg       Config
	client    *http.Client
	mu        sync.Mutex
	previous  map[string]netSample
	sampledAt time.Time
}
type netSample struct{ rx, tx uint64 }

func New(c Config) *Agent {
	return &Agent{cfg: c, client: &http.Client{Timeout: 10 * time.Second}, previous: map[string]netSample{}}
}
func (a *Agent) Report() error {
	r := a.Collect()
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, a.cfg.ServerURL+"/api/v1/reports", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
func (a *Agent) Collect() model.Report {
	h, _ := os.Hostname()
	r := model.Report{NodeID: a.cfg.NodeID, Hostname: h, Group: a.cfg.Group, Timestamp: time.Now().UTC(), OS: model.OSInfo{Name: runtime.GOOS, Architecture: runtime.GOARCH}, Hardware: model.Hardware{LogicalCPUs: runtime.NumCPU()}, Resources: collectResources(), Interfaces: a.collectInterfaces(), Network: probe(a.cfg.ProbeTarget)}
	r.Hardware.CPUModel = cpuModel()
	r.Hardware.TotalMemoryBytes = r.Resources.MemoryTotalBytes
	r.OS.UptimeSeconds = uptime()
	r.Checks = runChecks(a.cfg.Processes, a.cfg.Services)
	return r
}

// Run 持续采集并上报；ctx 取消时退出。once 为 true 时仅上报一次。
func (a *Agent) Run(ctx context.Context, once bool) {
	report := func() {
		if err := a.Report(); err != nil {
			log.Printf("report failed: %v", err)
		}
	}
	report()
	if once {
		return
	}
	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			report()
		case <-ctx.Done():
			return
		}
	}
}
