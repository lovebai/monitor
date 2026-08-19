package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"

	"monitor-agent/internal/agent"
)

func main() {
	configPath := flag.String("config", "agent.yaml", "configuration file (JSON or simple YAML)")
	once := flag.Bool("once", false, "collect and report once, then exit")
	install := flag.Bool("install", false, "register and start as a Windows service")
	uninstall := flag.Bool("uninstall", false, "stop and remove the Windows service")
	flag.Parse()
	if *uninstall {
		if err := agent.UninstallService(); err != nil {
			log.Fatal(err)
		}
		return
	}
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	printAgentConfig(*configPath, cfg)
	if *install {
		if err := agent.InstallService(*configPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	runner := agent.New(cfg)
	if agent.IsWindowsService() {
		if err := agent.RunAsService(runner); err != nil {
			log.Fatal(err)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runner.Run(ctx, *once)
}

func printAgentConfig(path string, c agent.Config) {
	log.Printf("==== Agent Monitor 启动 ====")
	log.Printf("配置文件: %s", path)
	log.Printf("Server 地址: %s", c.ServerURL)
	log.Printf("节点 ID: %s", c.NodeID)
	log.Printf("所属分组: %s", groupText(c.Group))
	log.Printf("上报间隔: %s", intervalText(c))
	log.Printf("探测目标: %s", targetText(c.ProbeTarget))
	log.Printf("进程检查: %s", listText(c.Processes))
	log.Printf("服务检查: %s", listText(c.Services))
	log.Printf("鉴权 Token: %s", maskToken(c.Token))
}

func groupText(g string) string {
	if g == "" {
		return "DEFAULT（未配置）"
	}
	return g
}
func intervalText(c agent.Config) string {
	if c.IntervalText != "" {
		return c.IntervalText
	}
	return c.Interval.String()
}
func targetText(t string) string {
	if t == "" {
		return "未配置"
	}
	return t
}
func listText(l []string) string {
	if len(l) == 0 {
		return "未配置"
	}
	return strings.Join(l, ", ")
}
func maskToken(t string) string {
	if len(t) <= 4 {
		return "****"
	}
	return "****"
}
