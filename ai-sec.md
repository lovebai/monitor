# 安全审查报告：monitor 项目（server / agent）

审查人角色：安全工程师
审查日期：2026-08-20
审查范围：`server/`（Go 1.24 + modernc sqlite）、`agent/`（Go 1.24，含 Windows 服务与旧系统兼容构建）、构建脚本 `build.ps1`、示例配置

---

## 一、风险总览

| # | 严重度 | 风险 | 位置 | 一句话描述 |
|---|--------|------|------|-----------|
| 1 | 🔴 高危 | 仪表盘/JSON API 完全无鉴权 | `server/internal/server/server.go` `ServeHTTP` | 除上报接口外，主页、节点详情、JSON API 均无任何认证，内网任意用户可查看全部节点敏感数据 |
| 2 | 🔴 高危 | 全链路明文 HTTP 传输 | `agent/internal/agent/agent.go`、`agent/agent.example*.yaml`、`server/cmd/main.go` | Token 与采集数据（IP/MAC/进程/服务）明文传输，可被窃听与篡改 |
| 3 | 🟠 中危 | Token 强度与校验方式薄弱 | `server/internal/server/server.go` `ingest`、`server/server.example.yaml` | 示例 Token 为 `123456`；`!=` 非恒定时间比较；无失败尝试限速（可暴力破解） |
| 4 | 🟠 中危 | 无请求限流与连接超时不完整 | `server/cmd/main.go`、`server/internal/server/server.go` | 仅设 `ReadHeaderTimeout`；ingest 可被高频打库（每次上报 4 次 DB 写 + 1 次 DELETE） |
| 5 | 🟠 中危 | HTTP 安全响应头缺失 | `server/internal/server/server.go` `home/json/nodeDetail/history` | 无 CSP / X-Frame-Options / nosniff，若存在 XSS 无缓解层，且可被第三方 iframe 嵌入 |
| 6 | 🟠 中危 | 敏感数据文件权限未收紧 | `server/internal/server/server.go` `New`、配置解析 | SQLite（含全部上报数据）与含 Token 的配置文件依赖 umask，Linux 上常见 0644 可被本机其他用户读取 |
| 7 | 🟡 低危 | XSS 防护依赖手写 `esc()` | `server/internal/server/ui.go`、`detail_ui.go` | 初始渲染走 `html/template`（安全），但 5s 刷新全部靠手写 `esc()`，属"默认不安全、靠自觉"模式 |
| 8 | 🟡 低危 | 上报数据含敏感进程/服务信息且无脱敏 | `agent/internal/agent/*`、`server/internal/model/model.go` | 进程名、服务名、磁盘挂载点、网卡 MAC/IP 全量上报；叠加问题 #1 即全网可查 |
| 9 | 🟡 低危 | 服务端无审计日志 | `server/cmd/main.go` | 无请求/事件日志，安全事件无法追溯 |
| 10 | 🟡 低危 | `history` 端点路径参数未统一校验 | `server/internal/server/server.go` `history` | 与 `nodeDetail` 的 `/` 过滤不一致（参数化查询下无注入，属健壮性/一致性） |

---

## 二、详细发现

### 🔴 高危 1：Web 界面与 JSON API 完全无鉴权（信息泄露）

`server/internal/server/server.go` 中 `ServeHTTP` 的路由分发：

```go
case r.Method == http.MethodGet && r.URL.Path == "/api/v1/nodes":          // 无鉴权
case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/nodes/") && strings.HasSuffix(r.URL.Path, "/history"): // 无鉴权
case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/nodes/"): // 无鉴权
case r.Method == http.MethodGet && r.URL.Path == "/":                        // 无鉴权
```

仅 `POST /api/v1/reports` 有 `Bearer Token` 校验。

**影响**：任何能访问 `listen` 端口（默认 `:8080`，绑定所有网卡）的人无需认证即可：
- 枚举全部节点：主机名、分组、CPU/内存/磁盘/负载；
- 获取内网拓扑情报：所有已启用网卡的 **MAC 地址、IPv4 地址**、磁盘挂载点；
- 获取业务情报：配置的进程检查、服务检查名称与运行状态（如 nginx、ssh、wscsvc）、Top 5 进程名与 PID；
- 通过 `/api/v1/nodes/{id}/history` 获取 30 天历史指标（CPU/内存/磁盘/延迟/网速）。

这些数据是内网渗透的侦察金矿（可定位主机角色、判断服务暴露面），且 API 返回 `Cache-Control: no-store` 无法依赖浏览器缓存缓解。

### 🔴 高危 2：全链路明文 HTTP，Token 与数据可被中间人窃取/篡改

- `agent/agent.example.yaml` / `agent.example1.yaml`：`server_url: http://127.0.0.1:8080`；
- `agent/internal/agent/agent.go` `Report()`：`http.NewRequest(... a.cfg.ServerURL+"/api/v1/reports")`，`Authorization: Bearer <token>` 随报文明文发送；
- `server/cmd/main.go`：`http.Server` 直接 `ListenAndServe()`，无 TLS 配置。

**影响**：跨网络部署时（README 明确支持"部署在被监控机器上"），Token、主机名、IP、MAC、进程清单均可被网络路径上的任意节点嗅探；攻击者还可**篡改上报数据**（伪造告警、伪造离线、植入恶意进程名），并**窃取 Token 后冒充任何 Agent 上报**。

**说明**：代码没有提供 `InsecureSkipVerify` 之类的降级开关（这点是好的），但也没有强制 HTTPS、未提示证书校验失败原因，示例配置即明文。

### 🟠 中危 3：Token 强度与校验方式薄弱

`server/internal/server/server.go` `ingest`：

```go
if r.Header.Get("Authorization") != "Bearer "+h.cfg.Token { http.Error(w, "unauthorized", 401) }
```

- **弱默认值**：`server/server.example.yaml` 中 `token: 123456`，若直接照抄使用等于无认证；
- **非恒定时间比较**：Go 字符串 `!=` 为普通字节比较，存在时序侧信道（配合大量请求理论上可逐字节还原 Token）；
- **无失败限速**：401 响应不记日志、不计数，攻击者可无限暴力尝试，配合弱 Token 极易命中；
- **明文存储**：Token 以明文写在 `server.yaml` / `agent.yaml`，文件权限未收紧（见 #6）。

### 🟠 中危 4：无请求限流，连接超时配置不完整

- `server/cmd/main.go`：`&http.Server{ReadHeaderTimeout: 5s}` —— 只有请求头超时，**无 `ReadTimeout` / `WriteTimeout`**，慢速 body 上传可长期占用连接（Slowloris 变体）；
- `ingest` 每次上报执行：`INSERT nodes` + `INSERT metrics` + `DELETE metrics`（全表扫描清理）+ `setAlert`（`INSERT ... ON CONFLICT` 或 `UPDATE`），即 **4 次数据库写**。无速率限制下，恶意客户端（或异常 Agent 循环）可高频 POST 造成 DB 写放大与 CPU 占用；
- GET 端点每次调用 `views()` 也会为离线节点执行 `setAlert` 写库，刷新页面即触发写放大。

### 🟠 中危 5：HTTP 安全响应头缺失

`home` / `json` / `nodeDetail` / `history` 四个处理器均只设置了 `Cache-Control` / `Content-Type`，缺少：
- `Content-Security-Policy`（无缓解层，一旦 XSS 即无防护）；
- `X-Frame-Options: DENY` 或 `frame-ancestors`（页面含内网 IP/MAC/进程信息，可被恶意站点 iframe 嵌入套取）；
- `X-Content-Type-Options: nosniff`。

### 🟠 中危 6：敏感数据文件权限未收紧

- `server/internal/server/server.go` `New()`：`sql.Open("sqlite", cfg.DatabasePath)` 未做 `os.Chmod(0600)`。SQLite 落盘权限由 umask 决定，Linux 默认 0644 时**本机任意用户可读**全部上报数据（IP/MAC/进程/服务/告警）；
- 含明文 Token 的 `server.yaml` / `agent.yaml` 同样无权限校验/提示；
- 本地 `server/monitor.db` 实测权限为 `-rwxrwxrwx`（当前环境为 Windows 挂载卷，但 Linux 部署时需显式收紧）。

### 🟡 低危 7：XSS 防护依赖手写 `esc()`（当前无实际漏洞，但模式脆弱）

- 初始渲染：全部使用 Go `html/template`，属性与 URL 均自动转义/净化（如 `href="/nodes/{{.NodeID}}"` 会被 URL 净化，`javascript:` 等被拦为 `#ZgotmplZ`），✅ 安全；
- 5s 局部刷新：`ui.go` / `detail_ui.go` 的 JS 中所有 `innerHTML` 拼接点（告警消息、进程/服务名、网卡名/MAC、Top5 进程名、磁盘挂载点）**均调用了 `esc()`**，且经 Go struct 类型约束的字段（pid、百分比等）为数字型，✅ 当前无注入点；
- 风险在于：这是"每处手动转义"模式，新增一个 `innerHTML` 拼接点漏掉 `esc()` 即出现存储型 XSS（恶意 Agent 可携带 payload 的字段：`node_id`、`hostname`、`group`、`checks[].name/detail`、`disks[].mountpoint`、`interfaces[].name/mac`、`top_cpu/top_memory[].name`、`alerts[].message`）。

### 🟡 低危 8：Agent 上报内容无脱敏

`agent/internal/agent/collect.go`、`processes.go`：网卡 MAC/全量 IP、磁盘挂载点、Top 5 进程名+PID、配置的进程/服务检查全部原样上报。若叠加 #1（Web 无鉴权），这些信息等同于公开。

### 🟡 低危 9：无审计日志

`server/cmd/main.go` 仅 `log.Printf` 启动信息；`ingest` 的 401 拒绝、数据入库、告警触发均无日志。无法追溯"谁在上报、谁被拒绝、何时告警"。

### 🟡 低危 10：`history` 端点参数一致性

`history` 用 `TrimPrefix/TrimSuffix` 提取节点 ID 后**直接作为 SQL 参数**（参数化，无注入），但未像 `nodeDetail` 那样拒绝含 `/` 的 ID，路径解析行为不一致（`/api/v1/nodes/a/b/history` 也会进入该分支），属健壮性问题。

---

## 三、做得好的地方（通过项）

- ✅ **SQL 注入免疫**：所有 SQL 均使用占位符参数化查询（`?`），包括带用户输入的 `node_id` 路径参数；
- ✅ **上报请求体限制**：`http.MaxBytesReader(w, r.Body, 2<<20)` 限制 2 MiB；
- ✅ **上报时间以服务端为准**：`report.Timestamp = time.Now().UTC()`，防止 Agent 时钟漂移/伪造时间绕过离线判定；
- ✅ **html/template 自动转义**：服务端初始渲染无模板注入；
- ✅ **Token 启动脱敏**：server/agent 启动日志均输出 `********`，不打印真实 Token；
- ✅ **Agent 命令执行参数化**：`pgrep -f` / `tasklist /FI` / `sc query` / `systemctl is-active` 均以参数数组方式调用，无 shell 拼接，**无命令注入**；
- ✅ **无弱化开关**：Agent 的 HTTP client 使用默认 Transport（未设 `InsecureSkipVerify`），HTTPS 证书会被正常校验；
- ✅ **WAL 模式 + 30 天数据清理**：`journal_mode=WAL`、`DELETE FROM metrics WHERE collected_at < now-30d`；
- ✅ **离线卡片不提供详情链接**，避免对无数据节点的无效访问；
- ✅ 服务安装（`InstallService`）需要管理员权限，属正常要求。

---

## 四、修复建议（按优先级）

### P0 —— 必须尽快
1. **为全部 GET 端点加鉴权**（`server/internal/server/server.go`）：
   - 方案 A（推荐）：读取 `Authorization: Bearer` 或 `?token=`，与 `h.cfg.Token` 做恒定时间比较（`crypto/subtle.ConstantTimeCompare`），未通过返回 401；注意 `/` 与静态页同样需要；
   - 方案 B：改为内网专用端口 + 防火墙白名单（`listen: 127.0.0.1:8080` 或走反向代理）。
2. **启用 HTTPS**：`http.Server` 增加 TLS（`ListenAndServeTLS`），文档/示例改为 `https://`；Agent 增加对自签名/私有 CA 的支持方式（`tls.Config.RootCAs` 自定义 CA 文件配置项，**不要**提供 `InsecureSkipVerify` 开关）。

### P1 —— 高优先级
3. **Token 强度与校验**：
   - 恒定时间比较：`crypto/subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1`；
   - 示例配置改为占位符（如 `token: "CHANGE_ME"`）并强制长度 ≥ 16，避免误用 `123456`；
   - ingest 端增加失败计数与退避（如 5 次失败锁 1 分钟），并记录审计日志。
4. **请求限流与超时**：
   - `http.Server` 补 `ReadTimeout` / `WriteTimeout` / `IdleTimeout`；
   - ingest 端点按 IP 做令牌桶限流（如 1 次/秒），GET 端点适当放宽但同样限流；`views()` 中离线告警写入改为"仅在状态翻转时写库"（当前每次请求都写）。
5. **数据库与配置文件权限**：初始化后 `os.Chmod(dbPath, 0600)`；启动时检查配置文件权限并给出警告（Windows 下跳过）。

### P2 —— 常规加固
6. **安全响应头**：全局中间件统一附加 `Content-Security-Policy: default-src 'self'`、`X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff`（注意当前页面内联 `<script>`/`<style>`，CSP 需用 `'unsafe-inline'` 或改为外部资源）。
7. **XSS 收口**：把 JS 的 `esc()` 改为注入式渲染（`textContent` + `createElement`）或引入小模板库，杜绝新增拼接点漏转义；对 `alert.message` 等可被 Agent 控制的字段一律走转义路径。
8. **审计日志**：记录上报来源 IP、401 拒绝、告警触发/恢复（可复用现有 `alerts` 表 + 简单 `log.Printf`）。
9. **Agent 侧加固**：上报敏感度说明（文档提示进程/服务名会上传）；对 `probe_target` 做合法性校验（仅 IP/域名:端口，禁止协议头与路径）；示例配置中的进程检查（`chrome`、`Everything`）改为占位示例并提示脱敏。
10. **`history` 端点**：与 `nodeDetail` 一致，拒绝含 `/` 的节点 ID。

### 低优先级 / 可选
11. mTLS 或双向认证（Server 验证 Agent 身份，防 Token 泄露后冒充上报）；
12. Server 数据加密落盘（SQLite 本身无加密，敏感数据依赖文件权限与磁盘加密）；
13. 为上报/查询提供可选压缩（`Content-Encoding: gzip`），降低明文体积与抓包可读性（不替代 TLS）。

---

## 五、结论

项目代码质量整体不错：SQL 全参数化、模板转义到位、命令执行无注入、请求体限流、Token 脱敏等基础安全实践已经具备。

但存在两个**必须修复的高危问题**：
1. **监控 Web 端零鉴权**（内网信息泄露面最大，一键可达全部节点 IP/MAC/进程情报）；
2. **全链路明文 HTTP**（Token 与数据可被窃听篡改，且示例配置即明文）。

其余为中低危加固项。建议按 P0 → P1 → P2 顺序处理，修复后可在对外暴露前重新复查。
