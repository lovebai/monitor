//go:build !linux && !windows

package server

import "runtime"

// collectServerStats 在不支持的主机上仅返回静态信息。
func collectServerStats(dbPath string, prev *cpuSample) serverStats {
	return serverStats{Hostname: hostname(), OSName: runtime.GOOS, Arch: runtime.GOARCH}
}
