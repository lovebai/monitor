package agent

import (
	"math"
	"monitor-agent/internal/model"
	"runtime"
	"strings"
	"testing"
)

func TestTopNByCPUAndMemory(t *testing.T) {
	procs := []model.ProcessStat{
		{Name: "a", PID: 1, CPUPercent: 5, MemoryBytes: 100},
		{Name: "b", PID: 2, CPUPercent: 9, MemoryBytes: 50},
		{Name: "c", PID: 3, CPUPercent: 1, MemoryBytes: 300},
		{Name: "d", PID: 4, CPUPercent: 7, MemoryBytes: 200},
		{Name: "e", PID: 5, CPUPercent: 3, MemoryBytes: 400},
		{Name: "f", PID: 6, CPUPercent: 11, MemoryBytes: 40},
	}
	cpu := topNByCPU(procs, topN)
	if len(cpu) != topN {
		t.Fatalf("topNByCPU length = %d, want %d", len(cpu), topN)
	}
	wantCPU := []string{"f", "b", "d", "a", "e"}
	for i, w := range wantCPU {
		if cpu[i].Name != w {
			t.Errorf("topNByCPU[%d] = %s, want %s", i, cpu[i].Name, w)
		}
	}
	mem := topNByMemory(procs, topN)
	wantMem := []string{"e", "c", "d", "a", "b"}
	for i, w := range wantMem {
		if mem[i].Name != w {
			t.Errorf("topNByMemory[%d] = %s, want %s", i, mem[i].Name, w)
		}
	}
	if got := topNByCPU(procs[:2], topN); len(got) != 2 {
		t.Errorf("topNByCPU with fewer items = %d, want 2", len(got))
	}
}

func TestCleanProcName(t *testing.T) {
	cases := map[string]string{
		"chrome":   "chrome",
		"chrome#3": "chrome",
		"svchost":  "svchost",
		"a#b":      "a#b",
	}
	for in, want := range cases {
		if got := cleanProcName(in); got != want {
			t.Errorf("cleanProcName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseProcStatLine(t *testing.T) {
	// 构造与 /proc/<pid>/stat 相同格式的行：utime=140、stime=160（字段下标 11、12）。
	fields := []string{"S", "0", "1", "1", "0", "-1", "4194560", "0", "0", "0", "0", "140", "160"}
	line := "12345 (my proc) " + strings.Join(fields, " ")
	name, ticks, ok := parseProcStatLine(line)
	if !ok {
		t.Fatal("parseProcStatLine should succeed")
	}
	if name != "my proc" {
		t.Errorf("name = %q, want %q", name, "my proc")
	}
	if ticks != 300 {
		t.Errorf("ticks = %d, want 300", ticks)
	}
	if _, _, ok := parseProcStatLine("garbage"); ok {
		t.Error("malformed line should fail")
	}
}

func TestWindowsTopProcessesFrom(t *testing.T) {
	d := &winData{Procs: []winProc{
		{Name: "chrome#3", IDProcess: 100, PercentProcessorTime: 200, WorkingSet: 1024},
		{Name: "svchost", IDProcess: 500, PercentProcessorTime: 50, WorkingSet: 2048},
	}}
	procs := windowsTopProcessesFrom(d)
	if len(procs) != 2 {
		t.Fatalf("len = %d, want 2", len(procs))
	}
	if procs[0].Name != "chrome" || procs[0].PID != 100 || procs[0].MemoryBytes != 1024 {
		t.Errorf("procs[0] = %+v", procs[0])
	}
	cores := runtime.NumCPU()
	want := 200 / float64(cores)
	if math.Abs(procs[0].CPUPercent-want) > 1e-9 {
		t.Errorf("cpu = %v, want %v", procs[0].CPUPercent, want)
	}
	if windowsTopProcessesFrom(nil) != nil {
		t.Error("nil winData should return nil")
	}
}

func TestWindowsResourcesFrom(t *testing.T) {
	d := &winData{Total: 8000, Free: 2000, CPU: 50, Uptime: 3600, Disks: []winDisk{{DeviceID: "C:", Size: 1000, FreeSpace: 400}}}
	r, up := windowsResourcesFrom(d)
	if r.MemoryTotalBytes != 8000 || r.MemoryUsedBytes != 6000 || up != 3600 {
		t.Errorf("memory/uptime wrong: %+v up=%d", r, up)
	}
	if len(r.Disks) != 1 || r.Disks[0].UsedPercent != 60 {
		t.Errorf("disks wrong: %+v", r.Disks)
	}
	cores := runtime.NumCPU()
	wantLoad := 50.0 / 100 * float64(cores)
	if r.Load1 != wantLoad || r.Load5 != wantLoad || r.Load15 != wantLoad {
		t.Errorf("load = %v/%v/%v, want %v", r.Load1, r.Load5, r.Load15, wantLoad)
	}
	if r2, up2 := windowsResourcesFrom(nil); r2.CPUPercent != 0 || up2 != 0 {
		t.Error("nil winData should return empty resources")
	}
}

func TestWindowsNetCountersFrom(t *testing.T) {
	d := &winData{Net: []winNet{{Name: "以太网", ReceivedBytes: 100, SentBytes: 200}}}
	m := windowsNetCountersFrom(d)
	if m["以太网"] != (netSample{rx: 100, tx: 200}) {
		t.Errorf("net counters wrong: %+v", m)
	}
	if windowsNetCountersFrom(nil) != nil {
		t.Error("nil winData should return nil")
	}
}
