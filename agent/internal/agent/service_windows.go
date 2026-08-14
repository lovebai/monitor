//go:build windows

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const ServiceName = "AgentMonitor"

type serviceHandler struct{ runner *Agent }

// NewServiceHandler 构造 Windows 服务处理器，供 svc.Run 使用。
func NewServiceHandler(r *Agent) *serviceHandler { return &serviceHandler{runner: r} }

func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.runner.Run(ctx, false)
		close(done)
	}()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			cancel()
			return false, 0
		}
	}
}

// IsWindowsService 报告当前进程是否由 Windows 服务控制管理器启动。
func IsWindowsService() bool {
	b, _ := svc.IsWindowsService()
	return b
}

// RunAsService 以 Windows 服务方式运行 Agent，随服务停止而退出。
func RunAsService(r *Agent) error {
	return svc.Run(ServiceName, NewServiceHandler(r))
}

// InstallService 将 Agent 注册为开机自启的 Windows 服务（需管理员权限）并启动。
func InstallService(configPath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器失败: %w", err)
	}
	defer m.Disconnect()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	if old, err := m.OpenService(ServiceName); err == nil {
		old.Close()
		return fmt.Errorf("服务 %s 已存在，请先执行 -uninstall", ServiceName)
	}
	s, err := m.CreateService(ServiceName, exe, mgr.Config{
		DisplayName: "Go Device Monitor Agent",
		Description: "采集并上报主机 CPU、内存、磁盘、网卡、进程与服务状态到监控 Server。",
		StartType:   mgr.StartAutomatic,
	}, "-config", absConfig)
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}
	defer s.Close()
	// 服务异常退出后自动重启（5s / 10s / 30s 依次重试）
	_ = exec.Command("sc", "failure", ServiceName, "reset=", "86400", "actions=", "restart/5000/restart/10000/restart/30000").Run()
	if err := s.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}
	return nil
}

// UninstallService 停止并删除 Agent 服务（需管理员权限）。
func UninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("服务 %s 不存在", ServiceName)
	}
	defer s.Close()
	_, _ = s.Control(svc.Stop) // 忽略"服务未运行"类错误
	return s.Delete()
}
