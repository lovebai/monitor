# 安全审查报告（第五轮复审）：monitor 项目（server / agent）

审查人角色：安全工程师
审查日期：2026-08-22
审查范围：基于最新提交（路由重构 `51fd872`、PBKDF2 密码哈希 `ab1a524` 等）后的代码复审，内网生产部署视角
说明：Go 工具链当前环境无法联网下载，未执行编译/测试；结论基于代码与测试用例静态审查。

---

## 一、修复落实追踪（对比第四轮）

### ✅ 本轮新增修复

| 主题 | 修复内容 | 位置 |
|------|----------|------|
| **登录口令加密存储** | 新增 PBKDF2-HMAC-SHA256 密码哈希：210000 次迭代、16 字节随机盐（`crypto/rand`）、32 字节密钥；格式 `pbkdf2$sha256$210000$<盐>$<哈希>`；`server.exe -gen "密码"` 生成并自动写入配置；登录时若配置为哈希则校验哈希、明文仅兼容迁移 | `server/internal/server/password.go`、`server/cmd/main.go` |
| **路由重构 + 鉴权中间件** | 改用 Go 1.22+ `http.ServeMux`（方法+路径参数）；`requireAuth` 中间件统一包裹全部受保护路由；公开路由仅 `/login`、`/logout`、`/api/v1/reports` | `server/internal/server/server.go` |
| **history 参数一致性问题（上轮 #7 子项）** | 路由参数化后自动解决：`GET /nodes/{nodeID}` 单段匹配，nodeID 天然不含 `/`，无需手工过滤 | `server/internal/server/server.go` |
| **审计日志雏形（部分解决）** | 新增 `logf`：请求行（方法/路径/来源 IP）、登录失败（用户名）、Token 拒绝（node_id）、收到上报详情 | `server/internal/server/server.go` |
| **pprof 受控** | `-debug` 才开放 `/debug/pprof/`，且包裹在 `requireAuth` 内 | `server/internal/server/server.go` |
| **Agent 启动日志** | 移除 Token 打印（连掩码都不再输出） | `agent/cmd/main.go` |
| — | `-help` 帮助信息、退出登录按钮（POST /logout）、登录页模板拆分、浏览器本地生成哈希的配置生成器（`crypto.getRandomValues`/Web Crypto，哈希不出本机） | 多处 |

**质量评价**：
- PBKDF2 参数（210000 迭代 / SHA-256 / 16B 盐 / 恒定时间比较）符合 OWASP 口令存储基线；浏览器生成器与服务端格式一致且本地计算不传服务器，设计良好；
- 路由重构后鉴权面清晰：受保护/公开路由由注册位置决定，`routing_test.go` 覆盖了未知路径 404、错误方法 405、多段路径 404 等边界，未发现绕过；
- 上轮遗留的 history 参数一致性问题随重构自然消除。

### ❌ 仍未修复（延续）

| 条目 | 现状 |
|------|------|
| 明文 HTTP / 无 TLS | 未变 |
| 限流 / 完整超时 / DB 写放大 | 未变（`ReadHeaderTimeout: 5s`；ingest 无速率限制；pruneMetrics 每次上报；views() 每次 GET 写告警） |
| 登录防爆破 | 未变（无失败锁定/限流） |
| `-remove` 验证码 `math/rand` 6 位 | 未变 |
| 配置护栏（非回环 + 未开鉴权拒绝启动） | 未变 |
| `server.yaml` 权限 0600 | 未变（仍只有 DB 与 agent.yaml 做了 Chmod） |
| 认证检查在 body 解析之后 | 未变（先 Decode 再校验 Token，未认证请求消耗解析 CPU） |
| 安全头 / XSS 收口 / probe_target 白名单 / Cookie `Secure` | 未变 |

---

## 二、本轮新观察点

1. **审计日志默认关闭**（`logf` 仅 `cfg.Debug` 时输出）：新增的登录失败/Token 拒绝/上报详情日志在**生产默认（无 `-debug`）下不产生任何输出**。"有日志能力"≠"有审计"；且登录失败日志只记用户名、未记来源 IP。建议：401/登录失败至少默认记录（不含敏感数据），请求明细可由 `-debug` 控制。
2. **PBKDF2 迭代次数由配置决定且无上限**（`password.go` `VerifyPasswordHash`：`strconv.Atoi(parts[1])` 后直接使用）：若配置文件被篡改（叠加 server.yaml 权限问题），可写入超大迭代次数使登录校验陷入长时间计算（登录 DoS）。配置文件为管理员可控，实际威胁低，但建议对迭代次数做上限校验（如 1万~1000万）。
3. **pprof 在 `auth_enabled: false` + `-debug` 时公开**：`/debug/pprof/` 的 requireAuth 在未开鉴权时是空转的。虽然 `-debug` 是显式开启，但 pprof 会暴露内存对象、goroutine 栈（可能含请求/数据片段），且与仪表盘同端口同网段暴露。建议：pprof 无条件要求鉴权，或提示仅限本机调试。
4. **`-gen "密码"` 明文进命令行**：密码会出现在进程参数列表（Linux `/proc/<pid>/cmdline`、Windows 任务管理器）与 shell 历史中。建议文档提示：可用配置生成器（浏览器本地哈希）替代，或在交互提示后读 stdin。
5. **`UpdateConfigPassword` 以 0644 写回配置文件**（`os.WriteFile(path, ..., 0644)`）：新建文件时权限为 0644，与 DB/agent.yaml 的 0600 策略不一致（已有文件则保留原权限）。

---

## 三、当前风险清单（第五轮）

| # | 严重度 | 风险 | 状态 |
|---|--------|------|------|
| 1 | 🟠 中危 | 明文 HTTP：独立 Token、登录口令、会话、节点数据可被嗅探（Wi-Fi/共享网段） | 未修复 |
| 2 | 🟠 中危 | 无请求限流 + 超时不完整 + DB 写放大 | 未修复 |
| 3 | 🟠 中危 | 审计日志默认关闭（生产无留痕；登录失败无来源 IP） | 部分修复（仅 debug） |
| 4 | 🟡 低危 | `server.yaml` 权限未收紧（全部节点 Token + 口令哈希，默认 0644） | 未修 |
| 5 | 🟡 低危 | 认证检查在 body 解析之后（未认证请求消耗解析 CPU） | 未修 |
| 6 | 🟡 低危 | 登录无失败锁定；`-remove` 验证码 `math/rand` 6 位 | 未修 |
| 7 | 🟡 低危 | PBKDF2 迭代次数无上限（配置篡改→登录 DoS，低） | 新观察 |
| 8 | 🟡 低危 | pprof 在无鉴权 + debug 时公开 | 新观察 |
| 9 | 🟡 低危 | `-gen` 密码进命令行参数；`UpdateConfigPassword` 0644 写回 | 新观察 |
| 10 | 🟡 低危 | 配置护栏缺失、安全头缺失、XSS 手写 `esc()`、probe_target 无白名单、Cookie 无 `Secure` | 未修 |

---

## 四、通过项（确认维持）

- ✅ **Agent 上报认证闭环**：node_id 强制绑定、无全局 Token、未登记拒绝、恒定时间比较、空配置拒绝启动、测试完整；
- ✅ **登录口令加密**：PBKDF2-SHA256/210000 迭代/随机盐/恒定时间比较，`-gen` 一键写入，浏览器生成器本地计算；
- ✅ **路由鉴权面清晰**：ServeMux 注册式路由 + requireAuth 中间件，公开路由仅 3 个；`routing_test.go` 覆盖边界；
- ✅ SQL 全参数化（含删除事务）、2 MiB 上报限制、服务端时间权威、`html/template` 自动转义、命令执行全参数化、无 `InsecureSkipVerify`、WAL、DB 与 agent.yaml 权限 0600、Token 不落日志。

---

## 五、建议（按当前剩余风险排序）

### P1
1. **审计日志默认开启**：401（Token 无效 / 登录失败）默认输出（含来源 IP），请求明细与上报详情留给 `-debug`。
2. **限流 + 完整超时 + 降低写放大**：补 `ReadTimeout/WriteTimeout/IdleTimeout`；`ingest` 按 IP 令牌桶限流；校验提前到解析出 node_id 即执行；`pruneMetrics` 降频；`views()` 离线告警"状态翻转才写库"。
3. **`server.yaml` 权限 0600**（含 `UpdateConfigPassword` 写回时保持 0600）。
4. **登录防爆破**：失败计数 + 按 IP 锁定。

### P2
5. **管理面 TLS** + 会话 Cookie `Secure`；Agent 支持自定义 CA。
6. **PBKDF2 迭代次数上限校验**；pprof 无条件要求鉴权；`-gen` 改交互输入提示。
7. `-remove` 验证码改 `crypto/rand` + 8 位；配置护栏（非回环 + 未开鉴权拒绝启动）。
8. 安全响应头、XSS 渐进收口、`probe_target` 白名单。

---

## 六、结论

第五轮复审：登录口令从明文升级为 **PBKDF2 加密存储**，路由重构使**鉴权面更清晰、参数处理更严谨**（顺带消除了上轮的 history 一致性问题），并补齐了 `-help`、退出登录等配套。口令存储这条线已达标（符合口令存储基线），Token 线已闭环。

剩余风险仍集中在**运维基线**：明文 HTTP、限流/写放大、审计日志默认关闭、server.yaml 权限、登录防爆破，外加本轮几个低危观察（PBKDF2 迭代上限、pprof 暴露面、`-gen` 命令行密码）。**均不阻塞内网生产上线**；其中"审计日志默认开启 + 401 记来源 IP"与"server.yaml 0600"两项成本最低、价值最高，建议优先做。
