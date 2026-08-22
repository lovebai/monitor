//go:build windows

package server

import (
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// collectServerStats 在 Windows 上通过 kernel32 API 采集 Server 主机状态。
func collectServerStats(dbPath string, prev *cpuSample) serverStats {
	st := serverStats{Hostname: hostname(), OSName: runtime.GOOS, Arch: runtime.GOARCH}
	cpu := windowsCPUPercent(prev)
	st.CPUPercent = cpu
	// Windows 无原生负载均值，按项目惯例用 CPU 使用率估算。
	if cores := runtime.NumCPU(); cores > 0 {
		est := cpu / 100 * float64(cores)
		st.Load1, st.Load5, st.Load15 = est, est, est
	}
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r != 0 {
		st.MemTotalBytes = ms.TotalPhys
		st.MemUsedBytes = ms.TotalPhys - ms.AvailPhys
	}
	if total, used := windowsDiskUsage(filepath.Dir(dbPath)); total > 0 {
		st.DiskTotalBytes = total
		st.DiskUsedBytes = used
		st.DiskUsedPct = float64(used) * 100 / float64(total)
	}
	return st
}

func windowsCPUPercent(prev *cpuSample) float64 {
	var idle, kernel, user syscall.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r == 0 {
		return 0
	}
	ft := func(f syscall.Filetime) uint64 {
		return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)
	}
	total := ft(kernel) + ft(user)
	idleT := ft(idle)
	if !prev.ok || total < prev.total || idleT < prev.idle {
		prev.total, prev.idle, prev.ok = total, idleT, true
		return 0
	}
	dt, di := total-prev.total, idleT-prev.idle
	prev.total, prev.idle = total, idleT
	if dt == 0 {
		return 0
	}
	return float64(dt-di) * 100 / float64(dt)
}

// windowsDiskUsage 统计数据库所在磁盘（目录）的总量与已用空间。
func windowsDiskUsage(dir string) (total, used uint64) {
	if dir == "" {
		dir = "."
	}
	if dir[len(dir)-1] != '\\' && dir[len(dir)-1] != '/' {
		dir += "\\"
	}
	dirp, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0
	}
	var freeAvail, totalBytes, totalFree uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(dirp)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 || totalBytes == 0 {
		return 0, 0
	}
	return totalBytes, totalBytes - freeAvail
}
