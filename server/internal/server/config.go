package server

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// FileConfig uses a deliberately small YAML subset so Server remains dependency-light.
// Values can be written as plain text or quoted strings.
type FileConfig struct {
	Listen               string
	Token                string
	DatabasePath         string
	OfflineAfterText     string
	LatencyThresholdMS   float64
	MemoryThresholdPct   float64
	DiskThresholdPct     float64
	HistoryRetentionDays int
}

func LoadFileConfig(path string) (FileConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, fmt.Errorf("read server config: %w", err)
	}
	c := FileConfig{Listen: ":8080", DatabasePath: "monitor.db", OfflineAfterText: "90s", LatencyThresholdMS: 500, MemoryThresholdPct: 80, DiskThresholdPct: 80, HistoryRetentionDays: 30}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch key {
		case "listen":
			c.Listen = value
		case "token":
			c.Token = value
		case "database_path":
			c.DatabasePath = value
		case "offline_after":
			c.OfflineAfterText = value
		case "latency_threshold_ms":
			fmt.Sscanf(value, "%f", &c.LatencyThresholdMS)
		case "memory_threshold_percent":
			fmt.Sscanf(value, "%f", &c.MemoryThresholdPct)
		case "disk_threshold_percent":
			fmt.Sscanf(value, "%f", &c.DiskThresholdPct)
		case "history_retention_days":
			fmt.Sscanf(value, "%d", &c.HistoryRetentionDays)
		}
	}
	if c.Token == "" {
		c.Token = os.Getenv("MONITOR_TOKEN")
	}
	if c.Token == "" {
		return c, fmt.Errorf("token is required in %s or MONITOR_TOKEN", path)
	}
	if _, err := time.ParseDuration(c.OfflineAfterText); err != nil {
		return c, fmt.Errorf("invalid offline_after: %w", err)
	}
	return c, nil
}
func (c FileConfig) Runtime() Config {
	d, _ := time.ParseDuration(c.OfflineAfterText)
	return Config{Token: c.Token, DatabasePath: c.DatabasePath, OfflineAfter: d, LatencyThresholdMS: c.LatencyThresholdMS, MemoryThresholdPct: c.MemoryThresholdPct, DiskThresholdPct: c.DiskThresholdPct, HistoryRetentionDays: c.HistoryRetentionDays}
}
