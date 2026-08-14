package agent

import (
	"fmt"
	"monitor-agent/internal/model"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func probe(target string) model.Network {
	if target == "" {
		return model.Network{}
	}
	if !strings.Contains(target, ":") {
		target += ":443"
	}
	start := time.Now()
	c, e := net.DialTimeout("tcp", target, 3*time.Second)
	n := model.Network{ProbeTarget: target, Reachable: e == nil}
	if e != nil {
		n.Error = e.Error()
		return n
	}
	n.LatencyMS = float64(time.Since(start).Microseconds()) / 1000
	c.Close()
	return n
}
func runChecks(processes, services []string) []model.Check {
	var out []model.Check
	for _, p := range processes {
		out = append(out, checkProcess(p))
	}
	for _, s := range services {
		out = append(out, checkService(s))
	}
	return out
}
func checkProcess(name string) model.Check {
	c := model.Check{Type: "process", Name: name}
	if runtime.GOOS == "windows" {
		c = windowsProcessCheck(c)
	} else {
		c = linuxProcessCheck(c)
	}
	if c.Healthy {
		c.PID = c.PIDs[0]
		c.Detail = fmt.Sprintf("运行中 ×%d", c.Count)
	} else {
		c.Detail = "未运行"
	}
	return c
}
func linuxProcessCheck(c model.Check) model.Check {
	b, e := exec.Command("pgrep", "-f", c.Name).Output()
	if e != nil {
		return c
	}
	for _, f := range strings.Fields(string(b)) {
		if pid, err := strconv.Atoi(f); err == nil {
			c.PIDs = append(c.PIDs, pid)
		}
	}
	c.Count = len(c.PIDs)
	c.Healthy = c.Count > 0
	return c
}
func windowsProcessCheck(c model.Check) model.Check {
	img := c.Name
	if !strings.HasSuffix(strings.ToLower(img), ".exe") {
		img += ".exe"
	}
	b, e := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", img), "/FO", "CSV", "/NH").Output()
	if e != nil || strings.Contains(strings.ToLower(string(b)), "no tasks") {
		return c
	}
	for _, l := range strings.Split(string(b), "\n") {
		p := tasklistCSV(l)
		if len(p) >= 2 && strings.EqualFold(p[0], img) {
			if pid, err := strconv.Atoi(p[1]); err == nil {
				c.PIDs = append(c.PIDs, pid)
			}
		}
	}
	c.Count = len(c.PIDs)
	c.Healthy = c.Count > 0
	return c
}
func tasklistCSV(l string) []string {
	l = strings.TrimSpace(l)
	if len(l) < 2 || l[0] != '"' {
		return nil
	}
	return strings.Split(l[1:len(l)-1], `","`)
}
func checkService(name string) model.Check {
	c := model.Check{Type: "service", Name: name}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("sc", "query", name)
	} else {
		cmd = exec.Command("systemctl", "is-active", name)
	}
	b, e := cmd.Output()
	v := strings.ToLower(string(b))
	c.Healthy = e == nil && (strings.Contains(v, "running") || strings.Contains(v, "active"))
	if c.Healthy {
		c.Detail = "运行中"
	} else {
		c.Detail = "未运行"
	}
	return c
}
