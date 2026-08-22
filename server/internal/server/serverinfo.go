package server

import (
	"os"
	"sync"
	"time"
)

// serverStatsRefreshInterval 控制 Server 主机状态的重采间隔。
const serverStatsRefreshInterval = 10 * time.Second

// serverStats 是 Server 所在主机的运行状态，渲染在主页底部。
type serverStats struct {
	Hostname       string
	OSName         string
	Arch           string
	Load1          float64
	Load5          float64
	Load15         float64
	CPUPercent     float64
	MemUsedBytes   uint64
	MemTotalBytes  uint64
	DiskUsedPct    float64
	DiskUsedBytes  uint64
	DiskTotalBytes uint64
	DBFileSize     uint64
	DBPath         string
}

// cpuSample 保存最近一次 CPU 计数，用于计算两次采样间的使用率。
type cpuSample struct {
	total uint64
	idle  uint64
	ok    bool
}

// serverInfo 带缓存地采集 Server 主机状态，约每 10 秒刷新一次，
// 避免每个页面请求都重复执行系统调用。
type serverInfo struct {
	mu     sync.Mutex
	cached serverStats
	at     time.Time
	prev   cpuSample
	dbPath string
}

func newServerInfo(dbPath string) *serverInfo {
	return &serverInfo{dbPath: dbPath}
}

// stats 返回当前缓存的 Server 主机状态，超过刷新间隔时重新采集。
func (s *serverInfo) stats() serverStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.at) >= serverStatsRefreshInterval {
		st := collectServerStats(s.dbPath, &s.prev)
		st.DBPath = s.dbPath
		st.DBFileSize = dbFileSize(s.dbPath)
		s.cached = st
		s.at = time.Now()
	}
	return s.cached
}

// dbFileSize 返回数据库主文件与 WAL 文件的总大小（WAL 模式下数据可能暂存在 -wal 中）。
func dbFileSize(path string) uint64 {
	var n uint64
	for _, p := range []string{path, path + "-wal"} {
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			n += uint64(fi.Size())
		}
	}
	return n
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
