package agent

import (
	"runtime"
	"testing"
)

func TestWindowsUptimeCollected(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("uptime 采集为 Windows 专用逻辑")
	}
	r, up := windowsResources()
	if r.CPUPercent == 0 && r.MemoryTotalBytes == 0 {
		t.Skip("WMI 不可用（受限环境），跳过断言")
	}
	if up == 0 {
		t.Error("windowsResources 应返回非零的系统开机时长")
	}
}
