package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"monitor-server/internal/server"
)

func main() {
	configPath := flag.String("config", "server.yaml", "server YAML configuration file")
	flag.Parse()
	cfg, err := server.LoadFileConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	printServerConfig(*configPath, cfg)
	h, err := server.New(cfg.Runtime())
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()
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

func printServerConfig(path string, c server.FileConfig) {
	log.Printf("==== Server Monitor 启动 ====")
	log.Printf("配置文件: %s", path)
	log.Printf("监听地址: %s", c.Listen)
	log.Printf("数据库: %s", c.DatabasePath)
	log.Printf("离线判定: %s", c.OfflineAfterText)
	log.Printf("延迟阈值: %.0f ms", c.LatencyThresholdMS)
	log.Printf("内存阈值: %.0f%%", c.MemoryThresholdPct)
	log.Printf("磁盘阈值: %.0f%%", c.DiskThresholdPct)
	log.Printf("鉴权 Token: %s", maskToken(c.Token))
}

func maskToken(t string) string {
	if len(t) <= 4 {
		return "****"
	}
	return t[:4] + "****"
}
