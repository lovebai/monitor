# Go 设备监控探针

一个轻量的 Server + Agent 监控项目。Agent 主动上报，不需要在受监控设备上开放入站端口。

## 功能特性

- 采集：主机、OS、CPU 型号/核心数、内存、磁盘、负载、网卡（MAC/MTU/状态/IP/流量速率）、TCP 探测延迟、进程与服务健康检查、CPU/内存占用 Top 5 进程。
- 分组：Agent 配置 `group` 指定所属分组，控制台按分组展示；未配置归入 `DEFAULT`。
- 主页（5 秒局部刷新，不整页刷新）：
  - 实时更新节点状态、CPU/内存/磁盘、负载（按核数归一化百分比 + 原始值）、网络速率（已启用网卡汇总）、探测延迟与告警；
  - 节点离线：卡片标红、状态点变红并显示「已离线」，不可点击进入详情页；
  - 内存/磁盘使用率超过 `memory_threshold_percent` / `disk_threshold_percent`（默认 80%）时，数值与进度条变红；
  - 离线声音报警：节点离线立即响铃并每 30 秒重复，点击「停止报警」确认当前离线批次，下一个设备离线时重新报警。
- 节点详情页：摘要卡片、内存、磁盘（全部分区）、已启用网卡（仅名称/MAC/IPv4）、服务与进程表格、进程资源 Top 5（CPU/内存占用）、告警，5 秒局部刷新。
- 告警：超出 `offline_after` 未上报即离线；网络不可达或 TCP 延迟超过 `latency_threshold_ms` 触发网络告警。
- 安全与存储：Bearer Token 鉴权、2 MiB 请求限制、SQLite 持久化、Web 仪表盘与 JSON API。
- 启动提示：Server/Agent 启动时在终端输出生效配置（Token 脱敏显示）。

平台支持：

- Linux：完整采集 CPU、内存、磁盘、负载与网卡流量（`/proc`、`df` 等）；进程 CPU/内存占用 Top 5 通过 `/proc` 读取并按两次采集差分计算。
- Windows：内置 CIM 采集 CPU、内存、磁盘；进程 CPU/内存占用 Top 5 通过 `Win32_PerfFormattedData_PerfProc_Process` 性能计数器采集；网卡字节速率通过 `Get-NetAdapterStatistics` 采集；负载以 CPU 利用率估算（Windows 无原生负载均值）。

## 项目结构

- `server/`：接收上报、SQLite 历史与告警事件、监控控制台。
- `agent/`：部署在被监控机器上的采集器。
- `bin/`：构建产物（`server.exe` / `agent.exe` / `server-linux-amd64` / `agent-linux-amd64`）。
- `build.ps1`：一键构建脚本。

## 快速开始

### 构建

```powershell
powershell -ExecutionPolicy Bypass -File build.ps1
```

产物输出到 `bin/`：Windows 版 `server.exe` / `agent.exe`，Linux amd64 版 `server-linux-amd64` / `agent-linux-amd64`。

开发调试也可以直接运行源码：

```powershell
cd server
Copy-Item server.example.yaml server.yaml   # 编辑配置后启动
go run ./cmd -config server.yaml
```

### 兼容旧系统（Windows Server 2008 / Windows 7）

Go 1.21 起官方不再支持 Windows 7 / Server 2008 / Server 2012（最低要求 Windows 10 / Server 2016），
因此用 Go 1.24 编译的 `agent.exe` 无法在这些旧系统上运行。主项目保持 Go 1.24 不变，
如需部署到 64 位旧系统，可额外构建专用兼容版：

```powershell
powershell -ExecutionPolicy Bypass -File build.ps1 -Legacy
```

产物为 `bin/agent-win2008.exe`（基于 Go 1.20.14，最后一个支持这些系统的官方版本，仅 amd64）。
首次执行会自动下载 Go 1.20.14 工具链到 `.tools/`（已加入 .gitignore）。
32 位旧系统不在支持范围内，请升级系统或改用其他采集方案。

### 配置 Server

`server.yaml`（参考 `server/server.example.yaml`）：

```yaml
listen: :8080                # 监听地址
token: 123456                # 鉴权 Token（必填）
database_path: monitor.db    # SQLite 数据库路径
offline_after: 90s           # 超过该时长未上报判定离线
latency_threshold_ms: 500    # 网络延迟告警阈值（ms）
memory_threshold_percent: 80 # 内存使用率变红阈值（%）
disk_threshold_percent: 80   # 磁盘使用率变红阈值（%）
```

### 配置 Agent

`agent.yaml`（参考 `agent/agent.example.yaml`）：

```yaml
server_url: http://127.0.0.1:8080  # Server 地址（必填）
token: 123456                      # 与 Server 相同的 Token（必填）
node_id: web-01                    # 节点 ID（默认取主机名）
group: web                         # 所属分组（默认 DEFAULT）
interval: 10s                      # 上报间隔（默认 30s）
probe_target: 1.1.1.1:443          # TCP 探测目标（省略端口默认 443）
processes:
  - nginx
  - java
services:
  - nginx
  - ssh
```

### 运行

1. 启动 Server：`server.exe -config server.yaml`（Linux 使用 `server-linux-amd64`）。
2. 在被监控设备启动 Agent：`agent.exe -config agent.yaml`；使用 `-once` 可只采集上报一次，`Ctrl+C` 停止。
3. 打开 `http://server:8080/` 查看仪表盘；`GET /api/v1/nodes` 获取 JSON。

## 注册为 Windows 服务

Agent 内置 Windows 服务支持（服务名 `AgentMonitor`，开机自启，异常退出后自动重启）。

以管理员身份打开 PowerShell，执行：

```powershell
# 注册并启动服务（config 使用绝对路径）
agent.exe -install -config C:\monitor\agent.yaml

# 停止并删除服务（不需要配置文件）
agent.exe -uninstall

# 查看服务状态
sc query AgentMonitor
```

说明：

- 注册成功后服务随 Windows 开机自启；服务异常退出会按 5s / 10s / 30s 间隔自动重启。
- 停止服务请使用 `sc stop AgentMonitor` 或 `services.msc`，Agent 会优雅退出（控制台运行时用 `Ctrl+C`）。
- Linux 部署可用 systemd 托管，示例：

```ini
[Unit]
Description=Monitor Agent
After=network-online.target

[Service]
ExecStart=/opt/monitor/agent-linux-amd64 -config /opt/monitor/agent.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## 页面说明

- 主页按分组展示节点卡片，顶部汇总在线数、上下行速率与活动告警数；5 秒局部刷新，无需整页刷新。
- 节点离线：卡片红底红边、状态点变红、显示「已离线」，不可点击进入详情页；离线声音报警每 30 秒重复，点击「停止报警」后当前批次不再报警，下一个设备离线时重新报警。
- 详情页：摘要卡片、内存、磁盘（全部分区）、网卡（仅已启用网卡的名称/MAC/IPv4）、服务与进程表格、进程资源 Top 5（CPU/内存占用）、告警，全部 5 秒局部刷新。
- Server 与 Agent 启动时输出生效配置（Token 脱敏），便于核对参数。

## 生产建议

- 使用 HTTPS（反向代理或服务端 TLS），将 Token 放入安全的配置管理系统。
- Windows 用 `agent.exe -install` 注册为服务，Linux 用 systemd（见上文示例）；上报失败会在下一个间隔自动重试。
- 告警目前实时展示在控制台；可按需接入邮件、Webhook、企业微信等通知渠道。
