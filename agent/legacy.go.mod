// 兼容旧系统专用模块文件（Windows 7 / Server 2008 / Server 2012）。
// 主项目保持 Go 1.24，本文件仅用于用 Go 1.20 工具链构建 agent-win2008.exe：
//   go1.20 build -modfile=legacy.go.mod -o ../bin/agent-win2008.exe ./cmd
module monitor-agent

go 1.20

require golang.org/x/sys v0.30.0
