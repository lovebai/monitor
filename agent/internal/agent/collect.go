package agent

import (
	"bufio"
	"encoding/json"
	"monitor-agent/internal/model"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var lastCPUTotal, lastCPUIdle uint64

// winDisk / winProc / winNet 为一次 Windows 采集返回的数据类型。
type winDisk struct {
	DeviceID  string
	Size      uint64
	FreeSpace uint64
}
type winProc struct {
	Name                 string
	IDProcess            int
	PercentProcessorTime float64
	WorkingSet           uint64
}
type winNet struct {
	Name          string
	ReceivedBytes uint64
	SentBytes     uint64
}
type winData struct {
	Total  uint64
	Free   uint64
	CPU    float64
	Uptime uint64
	Disks  []winDisk
	Procs  []winProc
	Net    []winNet
}

// collectWindowsAll 通过单次 PowerShell 调用采集资源、开机时长、网卡计数与进程占用，
// 避免每个指标各启动一个 PowerShell 进程的开销。
func collectWindowsAll() (*winData, error) {
	// Force UTF-8 on the output pipe: when the agent runs hidden or as a
	// service there is no console, so PowerShell would otherwise write JSON
	// using the OEM codepage (e.g. GBK) and Chinese adapter names would fail
	// to match net.Interfaces() names after JSON decoding.
	const script = "[Console]::OutputEncoding=[Text.Encoding]::UTF8;$os=Get-CimInstance Win32_OperatingSystem;$cpu=Get-CimInstance Win32_Processor|Measure-Object LoadPercentage -Average;$d=Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3'|Select-Object DeviceID,Size,FreeSpace;$p=@(Get-CimInstance Win32_PerfFormattedData_PerfProc_Process -ErrorAction SilentlyContinue|Where-Object {$_.Name -ne '_Total' -and $_.Name -ne 'Idle' -and $_.IDProcess -gt 0}|Select-Object Name,IDProcess,PercentProcessorTime,WorkingSet);$n=@(Get-NetAdapter|Get-NetAdapterStatistics -ErrorAction SilentlyContinue|Select-Object Name,ReceivedBytes,SentBytes);[pscustomobject]@{Total=[uint64]$os.TotalVisibleMemorySize*1024;Free=[uint64]$os.FreePhysicalMemory*1024;CPU=[double]$cpu.Average;Uptime=[int64](((Get-Date)-$os.LastBootUpTime).TotalSeconds);Disks=@($d);Procs=@($p);Net=@($n)}|ConvertTo-Json -Compress"
	b, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil || strings.TrimSpace(string(b)) == "" {
		return nil, err
	}
	var v winData
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// windowsResourcesFrom 从单次采集结果生成资源信息与开机时长。
func windowsResourcesFrom(v *winData) (model.Resources, uint64) {
	if v == nil {
		return model.Resources{}, 0
	}
	r := model.Resources{CPUPercent: v.CPU, MemoryTotalBytes: v.Total, MemoryUsedBytes: v.Total - v.Free}
	// Windows has no native load average; estimate from CPU utilization so the
	// dashboard shows a meaningful value instead of 0.
	cores := runtime.NumCPU()
	if cores > 0 {
		est := v.CPU / 100 * float64(cores)
		r.Load1, r.Load5, r.Load15 = est, est, est
	}
	for _, d := range v.Disks {
		used := d.Size - d.FreeSpace
		pct := 0.0
		if d.Size > 0 {
			pct = float64(used) * 100 / float64(d.Size)
		}
		r.Disks = append(r.Disks, model.Disk{Mountpoint: d.DeviceID, TotalBytes: d.Size, UsedBytes: used, UsedPercent: pct})
	}
	return r, v.Uptime
}

func collectResources(win *winData) (r model.Resources, up uint64) {
	if runtime.GOOS == "linux" {
		f, _ := os.Open("/proc/meminfo")
		if f != nil {
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
			r.MemoryTotalBytes = m["MemTotal"]
			r.MemoryUsedBytes = r.MemoryTotalBytes - m["MemAvailable"]
		}
		b, _ := os.ReadFile("/proc/loadavg")
		p := strings.Fields(string(b))
		if len(p) >= 3 {
			r.Load1, _ = strconv.ParseFloat(p[0], 64)
			r.Load5, _ = strconv.ParseFloat(p[1], 64)
			r.Load15, _ = strconv.ParseFloat(p[2], 64)
		}
		r.CPUPercent = linuxCPUPercent()
		r.Disks = linuxDisks()
		up = uptime()
	} else if runtime.GOOS == "windows" {
		r, up = windowsResourcesFrom(win)
	}
	return
}
func linuxCPUPercent() float64 {
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
	if lastCPUTotal == 0 {
		lastCPUTotal = total
		lastCPUIdle = idle
		return 0
	}
	if total < lastCPUTotal || idle < lastCPUIdle {
		lastCPUTotal, lastCPUIdle = total, idle
		return 0
	}
	dt, di := total-lastCPUTotal, idle-lastCPUIdle
	lastCPUTotal = total
	lastCPUIdle = idle
	if dt == 0 {
		return 0
	}
	return float64(dt-di) * 100 / float64(dt)
}
func linuxDisks() []model.Disk {
	out, e := exec.Command("df", "-B1", "--output=target,size,used").Output()
	if e != nil {
		return nil
	}
	var d []model.Disk
	for i, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if i == 0 {
			continue
		}
		p := strings.Fields(l)
		if len(p) == 3 {
			t, _ := strconv.ParseUint(p[1], 10, 64)
			u, _ := strconv.ParseUint(p[2], 10, 64)
			pct := 0.0
			if t > 0 {
				pct = float64(u) * 100 / float64(t)
			}
			d = append(d, model.Disk{Mountpoint: p[0], TotalBytes: t, UsedBytes: u, UsedPercent: pct})
		}
	}
	return d
}
func (a *Agent) collectInterfaces(win *winData) []model.NetworkInterface {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(a.sampledAt).Seconds()
	ifs, _ := net.Interfaces()
	var result []model.NetworkInterface
	counters := netCounters(win)
	for _, i := range ifs {
		n := model.NetworkInterface{Name: i.Name, MAC: i.HardwareAddr.String(), MTU: i.MTU, Flags: i.Flags.String()}
		addrs, _ := i.Addrs()
		for _, ad := range addrs {
			n.Addresses = append(n.Addresses, ad.String())
		}
		s := counters[i.Name]
		n.RxBytes = s.rx
		n.TxBytes = s.tx
		if old, ok := a.previous[i.Name]; ok && elapsed > 0 {
			// Counters may reset on adapter restart; ignore negative deltas.
			if s.rx >= old.rx {
				n.RxBytesPerSecond = float64(s.rx-old.rx) / elapsed
			}
			if s.tx >= old.tx {
				n.TxBytesPerSecond = float64(s.tx-old.tx) / elapsed
			}
		}
		a.previous[i.Name] = s
		result = append(result, n)
	}
	a.sampledAt = now
	return result
}
func linuxNetCounters() map[string]netSample {
	m := map[string]netSample{}
	b, e := os.ReadFile("/proc/net/dev")
	if e != nil {
		return m
	}
	for _, l := range strings.Split(string(b), "\n") {
		p := strings.Fields(strings.Replace(l, ":", " ", 1))
		if len(p) >= 10 {
			rx, _ := strconv.ParseUint(p[1], 10, 64)
			tx, _ := strconv.ParseUint(p[9], 10, 64)
			m[p[0]] = netSample{rx, tx}
		}
	}
	return m
}
func netCounters(win *winData) map[string]netSample {
	if runtime.GOOS == "windows" {
		return windowsNetCountersFrom(win)
	}
	return linuxNetCounters()
}
func windowsNetCountersFrom(v *winData) map[string]netSample {
	if v == nil {
		return nil
	}
	m := make(map[string]netSample, len(v.Net))
	for _, x := range v.Net {
		m[x.Name] = netSample{rx: x.ReceivedBytes, tx: x.SentBytes}
	}
	return m
}
func cpuModel() string {
	if runtime.GOOS != "linux" {
		return runtime.GOARCH
	}
	b, _ := os.ReadFile("/proc/cpuinfo")
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "model name") {
			return strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
		}
	}
	return "unknown"
}
func uptime() uint64 {
	b, e := os.ReadFile("/proc/uptime")
	if e != nil {
		return 0
	}
	p := strings.Fields(string(b))
	v, _ := strconv.ParseFloat(p[0], 64)
	return uint64(v)
}
