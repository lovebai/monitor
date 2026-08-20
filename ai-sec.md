# 安全审查报告（第三轮复审）：monitor 项目（server / agent）

审查人角色：安全工程师
审查日期：2026-08-20
审查范围：`server/`、`agent/` 源码与配置（基于提交 `83caa84` 修复后代码，内网生产部署视角复核）
说明：本轮验证上一轮报告的 P1 修复落实情况。Go 工具链当前环境无法联网下载，未执行编译/测试；结论基于代码与测试用例静态审查（新增 `token_test.go`/`config_test.go` 用例已覆盖关键逻辑）。

---

## 一、修复落实追踪（对照上轮报告）

### ✅ 已修复（提交 83caa84）

| 上轮条目 | 修复内容 | 位置 |
|----------|----------|------|
| 🟠 中危 #3 Token 体系脆弱 | **Token↔node_id 绑定**：新增 `agent_tokens` 配置（`node_id: token`），配置了独立 Token 的节点必须用它上报，全局 Token 不再适用该节点；未配置节点回退全局 Token | `server/internal/server/config.go`、`server.go` `validAgentToken` |
| 🟡 低危 #5 上报 Token `!=` 比较 | **恒定时间比较**：`validAgentToken` 全部走 `crypto/subtle.ConstantTimeCompare` | `server/internal/server/server.go` |
| 🟠 中危 #7 数据库权限 | **DB 文件 `os.Chmod(dbPath, 0600)`**（非 Windows） | `server/internal/server/server.go` `New` |
| 🟠 中危 #7 Agent 配置权限 | **`agent.yaml` 加载后 `os.Chmod(path, 0600)`**（非 Windows） | `agent/internal/agent/config.go` `LoadConfig` |
| — | 启动日志输出独立 Token 节点**数量**（不泄露值）；示例配置补充 `agent_tokens` 用法注释 | `server/cmd/main.go`、`server/server.example.yaml` |

**测试覆盖**：`token_test.go` 验证了独立 Token 匹配、跨节点 Token 拒绝（`n1` 用 `tok2` → 401）、配置独立 Token 后全局 Token 失效、未配置节点回退全局、空/错误 Token 拒绝、纯全局模式回退；`config_test.go` 验证 `agent_tokens` 解析。

**质量评价**：修复实现正确且严谨——先按 `node_id` 查独立 Token，命中则只认该 Token（不再回退全局），未命中才回退全局；比较均为恒定时间；示例中 `web-01: changeme-0001` 为占位符而非真实弱口令。

### ❌ 未修复（上轮建议未实施）

| 条目 | 现状 |
|------|------|
| 🟠 明文 HTTP / 无 TLS | `agent.go` 仍 `http://` 明文上报；`server` 无 TLS；会话 Cookie 无 `Secure` |
| 🟠 限流与完整超时 | `http.Server` 仍仅 `ReadHeaderTimeout: 5s`；`ingest` 无速率限制 |
| 🟠 DB 写放大 | `pruneMetrics` 每次上报全表 DELETE；`views()` 每次 GET 仍为离线节点写 `setAlert` |
| 🟠 审计日志 | 仍无（登录/上报/401/告警/删除均无留痕） |
| 🟡 登录防爆破 | `/login` 仍无失败锁定/限流（强口令下风险低） |
| 🟡 `-remove` 验证码 | 仍用 `math/rand` + 6 位（`server/cmd/main.go`） |
| 🟡 配置护栏 | 仍无"非回环监听 + 未开鉴权拒绝启动" |
| 🟡 安全头 / XSS 收口 / history 参数一致性 / probe_target 白名单 | 均未实施 |

### 🆕 本轮新观察点

1. **认证检查移到 body 解码之后**（`server.go` `ingest`）：为支持 node_id 绑定，先 `Decode` 完整上报体、再校验 Token。副作用：**未认证请求也会触发 2 MiB 内的 JSON 解析**，配合无速率限制（见上），匿名攻击者可发大量带 2 MiB 体的请求消耗解析 CPU——属于 DoS 面的轻微扩大。缓解思路：校验逻辑可改为"先解析出 `node_id` 即校验"（流式解码）或对 401 路径加简单计数。
2. **`agent_tokens` 缩进解析仅认空格**（`config.go`）：解析依赖 `strings.HasPrefix(raw, " ")`，**Tab 缩进不会被识别**为子项，会被忽略。示例与文档用空格无碍，但手写配置时易踩坑（配置静默失效 → 该节点回落全局 Token，实际是"失效到更宽松"方向）。建议解析时对 `\t` 同样处理，或在启动日志中打印已加载的独立 Token 节点数（当前已打印数量，可辅助发现）。
3. **`server.yaml` 权限未收紧**：本轮只对 DB 与 `agent.yaml` 做了 `0600`，但 `server.yaml` 含**全局 Token 与登录口令**，仍无权限校验（Linux 默认 0644 时本机其他用户可读）。建议与 DB 一致处理。

---

## 二、剩余风险总览（修复后）

| # | 严重度 | 风险 | 状态 |
|---|--------|------|------|
| 1 | 🟠 中危 | 明文 HTTP：Token、口令、会话、节点数据可被嗅探（Wi-Fi/共享网段） | 未修复 |
| 2 | 🟠 中危 | 无请求限流 + 超时不完整 + DB 写放大（每次上报 5 次写 + 全表 DELETE） | 未修复 |
| 3 | 🟠 中危 | 无审计日志（内网内部威胁/横向移动事件无法复盘） | 未修复 |
| 4 | 🟡 低危 | `/login` 无失败锁定（强口令下实际风险低） | 未修复 |
| 5 | 🟡 低危 | `-remove` 验证码 `math/rand` 6 位 | 未修复 |
| 6 | 🟡 低危 | `server.yaml` 权限未收紧（全局 Token + 口令明文） | 新观察 |
| 7 | 🟡 低危 | 认证检查在 body 解析之后 → 未认证请求消耗解析 CPU（DoS 面扩大） | 新观察 |
| 8 | 🟡 低危 | 配置护栏缺失（非回环 + 未开鉴权可启动）、安全头缺失、XSS 手写 `esc()`、`history` 参数不一致、`probe_target` 无白名单、会话 Cookie 无 `Secure` | 未修复 |

---

## 三、通过项（确认维持）

- ✅ 登录鉴权：恒定时间比较、`crypto/rand` 32 字节会话、HttpOnly+SameSite Cookie、7 天过期、无会话固定、门控无绕过；
- ✅ **Agent 上报认证**：node_id 绑定 + 恒定时间比较 + 测试覆盖；
- ✅ DB 与 agent.yaml 权限 0600（非 Windows）；
- ✅ SQL 全参数化（含删除事务）、2 MiB 上报限制、服务端时间权威、`html/template` 自动转义、命令执行全参数化、无 `InsecureSkipVerify` 弱化开关、WAL 模式、Token 启动脱敏。

---

## 四、建议（按当前剩余风险排序）

### P1
1. **限流 + 完整超时 + 降低写放大**：`http.Server` 补 `ReadTimeout/WriteTimeout/IdleTimeout`；`ingest` 按 IP 令牌桶限流；`pruneMetrics` 降频（如每小时或按指标行数触发）；`views()` 离线告警改为"状态翻转才写库"；将 Token 校验提前到"解析出 node_id 即校验"，避免未认证请求全量解析 body（新观察 #7）。
2. **审计日志**：登录成败（来源 IP）、401 拒绝（node_id+来源）、告警触发/恢复、`-remove` 操作。
3. **`server.yaml` 权限**：加载后 `os.Chmod(0600)`（非 Windows）+ 启动时对含凭证配置文件做权限告警（新观察 #6）。

### P2
4. **管理面 TLS**（`ListenAndServeTLS`）→ 会话 Cookie 加 `Secure`；Agent 支持自定义 CA 的 HTTPS。
5. **`agent_tokens` 解析兼容 Tab 缩进**，或启动日志输出已绑定节点数辅助核对（新观察 #2）。
6. **登录防爆破**：失败计数 + 按 IP 锁定。
7. **`-remove` 验证码**：`crypto/rand` + 8 位。
8. 配置护栏（非回环 + 未开鉴权拒绝启动）、安全响应头、XSS 渐进收口、`history` 参数 `/` 过滤、`probe_target` 白名单。

---

## 五、结论

本轮修复**质量高、切中要害**：上一轮最优先的 Token 体系问题（单全局 Token 无绑定、`!=` 时序比较）已完整解决——node_id 绑定、恒定时间比较、数据库与 Agent 配置权限 0600，且配套测试用例覆盖了绑定/回退/跨节点拒绝等关键路径，示例配置也是占位符而非弱口令。

**当前剩余风险已全部为"可接受的低-中危运维基线"**：明文 HTTP（内网可用网段隔离先行缓解）、限流/写放大（性能与 DoS 韧性）、审计日志（合规复盘）。建议按 P1 → P2 顺序在后续迭代中补齐；若监控系统不直接暴露给不可信网络，以上各项均不阻塞当前上线。
