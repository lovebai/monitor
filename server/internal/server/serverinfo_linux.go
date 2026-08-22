//go:build linux

package server

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// collectServerStats 在 Linux 上通过 /proc 与 statfs 采集 Server 主机状态。
func collectServerStats(dbPath string, prev *cpuSample) serverStats {
	st := serverStats{Hostname: hostname(), OSName: runtime.GOOS, Arch: runtime.GOARCH}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		p := strings.Fields(string(b))
		if len(p) >= 3 {
			st.Load1, _ = strconv.ParseFloat(p[0], 64)
			st.Load5, _ = strconv.ParseFloat(p[1], 64)
			st.Load15, _ = strconv.ParseFloat(p[2], 64)
		}
	}
	st.CPUPercent = linuxCPUPercent(prev)
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		m := map[string]uint64{}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			p := strings.Fields(sc.Text())
			if len(p) >= 2 {
				n, _ := strconv.ParseUint(p[1], 10, 64)
				m[strings.TrimSuffix(p[0], ":")] = n * 1024
			}
		}
		st.MemTotalBytes = m["MemTotal"]
		if avail := m["MemAvailable"]; avail > 0 {
			st.MemUsedBytes = st.MemTotalBytes - avail
		}
	}
	if total, used := linuxDiskUsage(dbPath); total > 0 {
		st.DiskTotalBytes = total
		st.DiskUsedBytes = used
		st.DiskUsedPct = float64(used) * 100 / float64(total)
	}
	return st
}

func linuxCPUPercent(prev *cpuSample) float64 {
	b, e := os.ReadFile("/proc/stat")
	if e != nil {
		return 0
	}
	p := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
	if len(p) < 5 {
		return 0
	}
	var total uint64
	for _, v := range p[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
	}
	idle, _ := strconv.ParseUint(p[4], 10, 64)
	if !prev.ok || total < prev.total || idle < prev.idle {
		prev.total, prev.idle, prev.ok = total, idle, true
		return 0
	}
	dt, di := total-prev.total, idle-prev.idle
	prev.total, prev.idle = total, idle
	if dt == 0 {
		return 0
	}
	return float64(dt-di) * 100 / float64(dt)
}

// linuxDiskUsage 统计数据库所在文件系统的总量与已用空间。
func linuxDiskUsage(dbPath string) (total, used uint64) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(dbPath), &fs); err != nil {
		return 0, 0
	}
	bs := uint64(fs.Bsize)
	total = fs.Blocks * bs
	avail := fs.Bavail * bs
	return total, total - avail
}
