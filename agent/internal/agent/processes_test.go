package agent

import (
	"monitor-agent/internal/model"
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
