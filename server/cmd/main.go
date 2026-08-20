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

func main() {
	configPath := flag.String("config", "server.yaml", "server YAML configuration file")
	remove := flag.String("remove", "", "删除指定 node_id 的节点（需输入 6 位验证码确认）")
	flag.Parse()
	cfg, err := server.LoadFileConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	h, err := server.New(cfg.Runtime())
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()
	if *remove != "" {
		removeNode(h, *remove)
		return
	}
	printServerConfig(*configPath, cfg)
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

func printServerConfig(path string, c server.FileConfig) {
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
	log.Printf("鉴权 Token: ********")
}
