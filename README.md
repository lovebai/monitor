# Go 设备监控探针

一个轻量的 Server + Agent 监控项目。Agent 主动上报，不需要在受监控设备上开放入站端口。

## 功能特性

- 采集：主机、OS、系统时间、备注别名（alias）、CPU 型号/核心数、内存、磁盘、负载、网卡（MAC/MTU/状态/IP/流量速率）、TCP 探测延迟、进程与服务健康检查、CPU/内存占用 Top 5 进程。
- 分组：Agent 配置 `group` 指定所属分组，控制台按分组展示；未配置归入 `DEFAULT`。
- 主页（5 秒局部刷新，不整页刷新）：
  - 实时更新节点状态、CPU/内存/磁盘、负载（按核数归一化百分比 + 原始值）、网络速率（已启用网卡汇总）、探测延迟、Agent 系统时间与告警；
  - 节点离线：卡片标红、状态点变红并显示「已离线」，不可点击进入详情页；
  - 内存/磁盘使用率超过 `memory_threshold_percent` / `disk_threshold_percent`（默认 80%）时，数值与进度条变红；
  - 离线声音报警：节点离线立即响铃并每 30 秒重复，点击「停止报警」确认当前离线批次，下一个设备离线时重新报警。
- 节点详情页：摘要卡片、内存、磁盘（全部分区）、已启用网卡（仅名称/MAC/IPv4）、服务与进程表格、进程资源 Top 5（CPU/内存占用）、历史曲线（CPU/内存/磁盘/网络速率/延迟）、告警，5 秒局部刷新。
- 告警：超出 `offline_after` 未上报即离线；网络不可达或 TCP 延迟超过 `latency_threshold_ms` 触发网络告警。
- 节点删除：`server.exe -remove <node_id>`，输入 6 位随机验证码确认后删除数据库中的节点及其历史指标与告警。
- 读接口鉴权：`auth_enabled: true` 时，主页/详情页/JSON 读接口需登录（会话 Cookie，默认关闭）；Agent 上报仍使用 Bearer Token，不受影响。
- 安全与存储：Bearer Token 上报鉴权（支持每节点独立 Token + node_id 绑定）、可选登录鉴权、2 MiB 请求限制、SQLite 持久化（含 Token 的配置与数据库自动收紧权限 0600）、Web 仪表盘与 JSON API。
- 启动提示：Server/Agent 启动时在终端输出版本号（按构建日期命名）与生效配置（Token 脱敏显示）。

平台支持：

- Linux：完整采集 CPU、内存、磁盘、负载与网卡流量（`/proc`、`df` 等）；进程 CPU/内存占用 Top 5 通过 `/proc` 读取并按两次采集差分计算。
- Windows：内置 CIM 通过单次 PowerShell 调用采集 CPU、内存、磁盘、网卡字节速率（`Get-NetAdapterStatistics`）与进程 CPU/内存占用 Top 5（`Win32_PerfFormattedData_PerfProc_Process` 性能计数器）；负载以 CPU 利用率估算（Windows 无原生负载均值）。

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

产物输出到 `bin/`：Windows 版 `server.exe` / `agent.exe`，Linux amd64 版 `server-linux-amd64` / `agent-linux-amd64`。构建时自动将当天日期（如 `2026.08.20`）写入版本号，Server/Agent 启动日志会输出。

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
history_retention_days: 30    # metrics 历史数据保留天数，超期数据在每次 Agent 上报时自动清理
auth_enabled: false           # 是否开启网页/JSON 读接口登录鉴权（默认关闭）
auth_username: admin          # 开启鉴权时的登录用户名
auth_password: 123456         # 开启鉴权时的登录密码
agent_tokens:                 # 每节点独立 Token（node_id 绑定），未列出的节点使用全局 token
  web-01: changeme-0001
  web-02: changeme-0002
```

`agent_tokens` 语义：配置了独立 Token 的节点只能用该 Token 上报（Token 与 node_id 绑定，防止一个 Token 泄露后冒充任意节点）；未配置的节点回退到全局 `token`。比较使用恒定时间算法，避免时序侧信道。轮换方式：同步修改 `server.yaml` 与对应 `agent.yaml` 后重启 Server 与 Agent；Linux 上 Server 启动时会自动将数据库与 Agent 配置文件的权限收紧为 0600。

### 配置 Agent

`agent.yaml`（参考 `agent/agent.example.yaml`）：

```yaml
server_url: http://127.0.0.1:8080  # Server 地址（必填）
token: 123456                      # 与 Server 相同的 Token（必填）
node_id: web-01                    # 节点 ID（默认取主机名）
alias: 生产环境 Web 服务器           # 备注别名，显示在主页节点卡片与详情页标题中
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

开启 `auth_enabled` 后，浏览器访问会先跳转到 `/login` 登录；JSON 读接口（`/api/v1/nodes`、历史接口）需携带登录会话，否则返回 401。

### 删除节点

```powershell
server.exe -remove <node_id>
```

程序会先打印 6 位随机验证码，输入一致后才会删除数据库中对应节点（连同历史指标与告警），删除后主页不再显示该节点。

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

- 主页按分组展示节点卡片，顶部汇总在线数、上下行速率与活动告警数；卡片底部浅色显示 Agent 系统时间；5 秒局部刷新，无需整页刷新。
- 节点离线：卡片红底红边、状态点变红、显示「已离线」，不可点击进入详情页；离线声音报警每 30 秒重复，点击「停止报警」后当前批次不再报警，下一个设备离线时重新报警。
- 详情页：摘要卡片、内存、磁盘（全部分区）、网卡（仅已启用网卡的名称/MAC/IPv4）、服务与进程表格、进程资源 Top 5（CPU/内存占用）、历史曲线（CPU/内存/磁盘/网络速率/延迟）、告警，全部 5 秒局部刷新。
- Server 与 Agent 启动时输出生效配置（Token 脱敏），便于核对参数。

## 数据库表

SQLite 数据库（`database_path`，默认 `monitor.db`）包含三张表：

### nodes — 节点最新状态

| 字段 | 类型 | 说明 |
|---|---|---|
| `node_id` | TEXT | 节点 ID，主键 |
| `hostname` | TEXT | 主机名 |
| `last_seen` | INTEGER | 最近一次上报时间（Unix 秒） |
| `report_json` | TEXT | 最近一次完整上报 JSON（资源、网卡、检查、Top5 进程、别名、系统时间等） |

Agent 每次上报对该表整行覆盖（UPSERT），只保留最新快照；离线判定基于 `last_seen` 与 `offline_after`。

### metrics — 历史指标

| 字段 | 类型 | 说明 |
|---|---|---|
| `node_id` | TEXT | 节点 ID |
| `collected_at` | INTEGER | 采集时间（服务端收到上报的时刻，Unix 秒） |
| `cpu` | REAL | CPU 使用率（%） |
| `memory_percent` | REAL | 内存使用率（%） |
| `disk_percent` | REAL | 磁盘使用率（%，仅第一块磁盘） |
| `latency_ms` | REAL | TCP 探测延迟（ms） |
| `rx_rate` / `tx_rate` | REAL | 已启用网卡汇总的收发速率（字节/秒） |

每次上报追加一行，供详情页历史曲线与 `/api/v1/nodes/<id>/history` 使用；按 `history_retention_days`（默认 30 天）自动清理超期数据；`idx_metrics_node_time` 索引加速按节点 + 时间查询。

### alerts — 告警事件

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | INTEGER | 自增主键 |
| `node_id` | TEXT | 节点 ID |
| `kind` | TEXT | 告警类型（`offline` 离线 / `latency` 网络延迟） |
| `message` | TEXT | 告警内容 |
| `active` | INTEGER | 是否未解决（1 = 活跃） |
| `created_at` / `updated_at` / `resolved_at` | INTEGER | 创建 / 更新 / 解决时间（Unix 秒） |

同一节点同一类型只保留一条活跃告警（`UNIQUE(node_id,kind)`）；节点恢复后标记为已解决并写入 `resolved_at`。

## 生产建议

- 使用 HTTPS（反向代理或服务端 TLS），将 Token 放入安全的配置管理系统。
- Windows 用 `agent.exe -install` 注册为服务，Linux 用 systemd（见上文示例）；上报失败会在下一个间隔自动重试。
