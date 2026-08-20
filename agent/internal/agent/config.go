package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	ServerURL    string        `json:"server_url"`
	Token        string        `json:"token"`
	NodeID       string        `json:"node_id"`
	Alias        string        `json:"alias"`
	Group        string        `json:"group"`
	Interval     time.Duration `json:"-"`
	IntervalText string        `json:"interval"`
	ProbeTarget  string        `json:"probe_target"`
	Processes    []string      `json:"processes"`
	Services     []string      `json:"services"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		c = parseSimpleYAML(string(b))
	}
	if c.ServerURL == "" || c.Token == "" {
		return c, fmt.Errorf("server_url and token are required")
	}
	if c.NodeID == "" {
		h, _ := os.Hostname()
		c.NodeID = h
	}
	if c.IntervalText == "" {
		c.IntervalText = "30s"
	}
	c.Interval, err = time.ParseDuration(c.IntervalText)
	if err != nil || c.Interval < time.Second {
		return c, fmt.Errorf("invalid interval")
	}
	// 配置文件含 Token，收紧权限（Windows 上无实际强制作用，仅尽力而为）。
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0600)
	}
	return c, nil
}

// Supports the supplied flat sample YAML; JSON is recommended for complex lists.
func parseSimpleYAML(s string) Config {
	var c Config
	var list *[]string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "- ") && list != nil {
			*list = append(*list, strings.Trim(strings.TrimPrefix(l, "- "), "\"'"))
			continue
		}
		p := strings.SplitN(l, ":", 2)
		if len(p) != 2 {
			continue
		}
		k, v := strings.TrimSpace(p[0]), strings.Trim(strings.TrimSpace(p[1]), "\"'")
		list = nil
		switch k {
		case "server_url":
			c.ServerURL = v
		case "token":
			c.Token = v
		case "node_id":
			c.NodeID = v
		case "alias":
			c.Alias = v
		case "group":
			c.Group = v
		case "interval":
			c.IntervalText = v
		case "probe_target":
			c.ProbeTarget = v
		case "processes":
			list = &c.Processes
		case "services":
			list = &c.Services
		}
	}
	return c
}
