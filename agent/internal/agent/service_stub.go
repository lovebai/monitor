//go:build !windows

package agent

import "errors"

// IsWindowsService 非 Windows 平台恒为 false。
func IsWindowsService() bool { return false }

// RunAsService 仅在 Windows 可用。
func RunAsService(*Agent) error { return errors.New("服务模式仅支持 Windows") }

// InstallService 仅在 Windows 可用。
func InstallService(string) error { return errors.New("服务安装仅支持 Windows") }

// UninstallService 仅在 Windows 可用。
func UninstallService() error { return errors.New("服务卸载仅支持 Windows") }
