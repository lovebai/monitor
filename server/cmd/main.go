package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"monitor-server/internal/server"
)

// Version 为构建版本号，按构建日期命名；build.ps1 会通过 -ldflags 注入当天日期。
var Version = "2026.08.20"

func main() {
	configPath := flag.String("config", "server.yaml", "server YAML configuration file")
	remove := flag.String("remove", "", "删除指定 node_id 的节点（需输入 6 位验证码确认）")
	gen := flag.String("gen", "", "生成登录密码的加密哈希并写入配置文件 auth_password")
	debug := flag.Bool("debug", false, "启用调试模式（输出详细请求与 Agent 上报日志，并开放 /debug/pprof）")
	help := flag.Bool("help", false, "显示帮助信息")
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
		return
	}
	log.Printf("Server Monitor 版本: %s", Version)
	if *gen != "" {
		generatePassword(*gen, *configPath)
		return
	}
	cfg, err := server.LoadFileConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	rt := cfg.Runtime()
	rt.Debug = *debug
	h, err := server.New(rt)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()
	if *remove != "" {
		removeNode(h, *remove)
		return
	}
	printServerConfig(*configPath, cfg, *debug)
	httpServer := &http.Server{Addr: cfg.Listen, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("monitor server listening on %s", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server stopped: %v", err)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

// usage 打印命令行帮助信息。
func usage() {
	fmt.Printf(`用法: %s [选项]

Server Monitor 监控服务端。Agent 主动上报，服务端负责接收、存储与展示。

选项:
  -config <文件>    指定 YAML 配置文件（默认 server.yaml）
  -gen <密码>       生成登录密码的 PBKDF2 加密哈希并自动写入配置文件 auth_password
  -remove <node_id> 删除指定节点（含历史指标与告警，需输入 6 位验证码确认）
  -debug            启用调试模式：输出详细请求与 Agent 上报日志，并开放 /debug/pprof
  -help             显示本帮助信息

示例:
  server.exe -config server.yaml
  server.exe -gen "你的密码" -config server.yaml
  server.exe -remove web-01
  server.exe -debug -config server.yaml
`, os.Args[0])
}

// generatePassword 生成登录密码的加密哈希，打印到终端并自动写入配置文件的 auth_password。
func generatePassword(password, configPath string) {
	hash, err := server.GeneratePasswordHash(password)
	if err != nil {
		log.Fatalf("生成加密密码失败: %v", err)
	}
	fmt.Printf("生成的加密密码：%s\n", hash)
	if err := server.UpdateConfigPassword(configPath, hash); err != nil {
		log.Printf("提示：未能自动写入配置文件 %s（%v），请手动将上面的哈希填入 auth_password", configPath, err)
		return
	}
	log.Printf("已写入配置文件 %s 的 auth_password，请妥善保管原密码", configPath)
}

// removeNode 打印 6 位随机验证码，用户输入一致后才删除数据库中的节点。
func removeNode(h *server.Handler, id string) {
	exists, err := h.NodeExists(id)
	if err != nil {
		log.Fatalf("查询节点失败: %v", err)
	}
	if !exists {
		log.Fatalf("节点 %s 不存在", id)
	}
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	fmt.Printf("即将删除节点 %s，请输入验证码 [%s] 确认：\n", id, code)
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(input) != code {
		log.Fatalf("验证码错误，已取消删除")
	}
	if err := h.RemoveNode(id); err != nil {
		log.Fatalf("删除节点失败: %v", err)
	}
	log.Printf("已删除节点 %s，页面将不再显示", id)
}

func printServerConfig(path string, c server.FileConfig, debug bool) {
	log.Printf("==== Server Monitor 启动 ====")
	log.Printf("配置文件: %s", path)
	log.Printf("监听地址: %s", c.Listen)
	log.Printf("数据库: %s", c.DatabasePath)
	log.Printf("离线判定: %s", c.OfflineAfterText)
	log.Printf("延迟阈值: %.0f ms", c.LatencyThresholdMS)
	log.Printf("内存阈值: %.0f%%", c.MemoryThresholdPct)
	log.Printf("磁盘阈值: %.0f%%", c.DiskThresholdPct)
	log.Printf("历史保留: %d 天", c.HistoryRetentionDays)
	if c.AuthEnabled {
		log.Printf("网页鉴权: 开启（用户 %s）", c.AuthUsername)
	} else {
		log.Printf("网页鉴权: 关闭")
	}
	log.Printf("Agent 独立 Token: %d 个节点（node_id 绑定）", len(c.AgentTokens))
	if debug {
		log.Printf("调试模式: 开启（输出请求/上报日志，/debug/pprof 可用）")
	} else {
		log.Printf("调试模式: 关闭")
	}
}
