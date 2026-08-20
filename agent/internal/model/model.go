package model

import "time"

type Report struct {
	NodeID     string             `json:"node_id"`
	Hostname   string             `json:"hostname"`
	Alias      string             `json:"alias,omitempty"`
	Group      string             `json:"group"`
	Timestamp  time.Time          `json:"timestamp"`
	SystemTime time.Time          `json:"system_time"`
	OS         OSInfo             `json:"os"`
	Hardware   Hardware           `json:"hardware"`
	Resources  Resources          `json:"resources"`
	Interfaces []NetworkInterface `json:"interfaces"`
	Network    Network            `json:"network"`
	Checks     []Check            `json:"checks"`
	TopCPU     []ProcessStat      `json:"top_cpu,omitempty"`
	TopMemory  []ProcessStat      `json:"top_memory,omitempty"`
}
type OSInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Architecture  string `json:"architecture"`
	UptimeSeconds uint64 `json:"uptime_seconds"`
}
type Hardware struct {
	CPUModel         string `json:"cpu_model"`
	LogicalCPUs      int    `json:"logical_cpus"`
	TotalMemoryBytes uint64 `json:"total_memory_bytes"`
}
type Resources struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes"`
	Disks            []Disk  `json:"disks"`
	Load1            float64 `json:"load_1"`
	Load5            float64 `json:"load_5"`
	Load15           float64 `json:"load_15"`
}
type Disk struct {
	Mountpoint  string  `json:"mountpoint"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}
type NetworkInterface struct {
	Name             string   `json:"name"`
	MAC              string   `json:"mac"`
	MTU              int      `json:"mtu"`
	Flags            string   `json:"flags"`
	Addresses        []string `json:"addresses"`
	RxBytes          uint64   `json:"rx_bytes"`
	TxBytes          uint64   `json:"tx_bytes"`
	RxBytesPerSecond float64  `json:"rx_bytes_per_second"`
	TxBytesPerSecond float64  `json:"tx_bytes_per_second"`
}
type Network struct {
	ProbeTarget string  `json:"probe_target,omitempty"`
	LatencyMS   float64 `json:"latency_ms,omitempty"`
	Reachable   bool    `json:"reachable"`
	Error       string  `json:"error,omitempty"`
}
type Check struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Detail  string `json:"detail,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Count   int    `json:"count,omitempty"`
	PIDs    []int  `json:"pids,omitempty"`
}
type ProcessStat struct {
	Name        string  `json:"name"`
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
	MemoryPct   float64 `json:"memory_percent"`
}
