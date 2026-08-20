package agent

import (
	"monitor-agent/internal/model"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const topN = 5

// procPrevCPU 保存上一次采集时各进程消耗的 CPU 时钟数，
// procPrevTotal 保存上一次采集时系统总的 CPU 时钟数，用于计算两次采集间的 CPU 占用率。
var procPrevCPU = map[int]uint64{}
var procPrevTotal uint64

// topProcesses 返回 CPU 占用率与内存占用最高的 topN 个进程。
// win 为 Windows 单次采集结果；up 为 Linux 开机时长（首次采样时用于平均占用估算）。
func topProcesses(totalMem, up uint64, win *winData) (topCPU, topMemory []model.ProcessStat) {
	var procs []model.ProcessStat
	if runtime.GOOS == "windows" {
		procs = windowsTopProcessesFrom(win)
	} else {
		procs = linuxTopProcesses(up)
	}
	for i := range procs {
		if totalMem > 0 {
			procs[i].MemoryPct = float64(procs[i].MemoryBytes) * 100 / float64(totalMem)
		}
	}
	return topNByCPU(procs, topN), topNByMemory(procs, topN)
}

func topNByCPU(procs []model.ProcessStat, n int) []model.ProcessStat {
	out := append([]model.ProcessStat(nil), procs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CPUPercent > out[j].CPUPercent })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func topNByMemory(procs []model.ProcessStat, n int) []model.ProcessStat {
	out := append([]model.ProcessStat(nil), procs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MemoryBytes > out[j].MemoryBytes })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// linuxTopProcesses 通过 /proc 读取各进程 CPU 时钟数与常驻内存（VmRSS）。
// CPU 占用率优先使用两次采集间的差分；首次采集退化为进程启动以来的平均占用。
func linuxTopProcesses(up uint64) []model.ProcessStat {
	total := cpuTotalTicks()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	cores := runtime.NumCPU()
	next := make(map[int]uint64)
	var procs []model.ProcessStat
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		name, ticks, ok := readProcStat(pid)
		if !ok {
			continue
		}
		next[pid] = ticks
		cpu := 0.0
		if prev, ok := procPrevCPU[pid]; ok && procPrevTotal > 0 && total >= procPrevTotal && ticks >= prev {
			if dt := total - procPrevTotal; dt > 0 {
				cpu = float64(ticks-prev) * 100 / float64(dt)
			}
		} else if up > 0 && cores > 0 {
			// USER_HZ 按 100 计：ticks/100 为 CPU 秒数，再按运行时长与核心数归一化为整体容量百分比。
			cpu = float64(ticks) / (float64(up) * float64(cores))
		}
		procs = append(procs, model.ProcessStat{Name: name, PID: pid, CPUPercent: cpu, MemoryBytes: procRSS(pid)})
	}
	procPrevCPU = next
	procPrevTotal = total
	return procs
}

func cpuTotalTicks() uint64 {
	b, e := os.ReadFile("/proc/stat")
	if e != nil {
		return 0
	}
	p := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
	var total uint64
	for _, v := range p[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
	}
	return total
}

// readProcStat 解析 /proc/<pid>/stat，返回进程名与 utime+stime 时钟数。
func readProcStat(pid int) (name string, ticks uint64, ok bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, false
	}
	return parseProcStatLine(string(b))
}

func parseProcStatLine(s string) (name string, ticks uint64, ok bool) {
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close <= open {
		return "", 0, false
	}
	name = s[open+1 : close]
	// 括号后依次为 state(3), ppid(4), ..., utime(14), stime(15)，即剩余字段下标 11、12。
	rest := strings.Fields(s[close+2:])
	if len(rest) < 13 {
		return "", 0, false
	}
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	return name, utime + stime, true
}

func procRSS(pid int) uint64 {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, "VmRSS:") {
			continue
		}
		f := strings.Fields(l)
		if len(f) >= 2 {
			kb, _ := strconv.ParseUint(f[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

// windowsTopProcessesFrom 从单次采集结果中取各进程 CPU 占用率与内存工作集。
func windowsTopProcessesFrom(v *winData) []model.ProcessStat {
	if v == nil {
		return nil
	}
	cores := runtime.NumCPU()
	out := make([]model.ProcessStat, 0, len(v.Procs))
	for _, p := range v.Procs {
		cpu := p.PercentProcessorTime
		if cores > 0 {
			cpu /= float64(cores)
		}
		out = append(out, model.ProcessStat{Name: cleanProcName(p.Name), PID: p.IDProcess, CPUPercent: cpu, MemoryBytes: p.WorkingSet})
	}
	return out
}

// cleanProcName 去掉多实例进程名后的 # 序号（如 chrome#3 -> chrome）。
func cleanProcName(n string) string {
	if i := strings.LastIndexByte(n, '#'); i > 0 {
		if _, err := strconv.Atoi(n[i+1:]); err == nil {
			return n[:i]
		}
	}
	return n
}
