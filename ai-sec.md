# 安全审查报告（第四轮复审）：monitor 项目（server / agent）

审查人角色：安全工程师
审查日期：2026-08-20
审查范围：基于提交 `3a89586`（移除全局 Token，强制 node_id 绑定）后的代码复审，内网生产部署视角
说明：Go 工具链当前环境无法联网下载，未执行编译/测试；结论基于代码与测试用例静态审查。

---

## 一、修复落实追踪（对比第三轮）

### ✅ 本轮新增修复（提交 3a89586）——Token 体系从"推荐独立"升级为"强制登记"

| 上轮状态 | 本轮变更 | 位置 |
|----------|----------|------|
| 独立 Token + 全局回退 | **移除全局 Token**：`Config.Token` / `FileConfig.Token` / `token:` 配置键 / `MONITOR_TOKEN` 环境变量全部删除 | `server/internal/server/config.go`、`server.go`、`server/cmd/main.go` |
| 未登记节点回退全局 Token | **未登记 node_id 一律 401（fail-closed）**：`validAgentToken` 未命中或空值直接拒绝，不再有任何回退路径 | `server/internal/server/server.go` |
| 示例含 `token: 123456` | 示例/README 移除全局弱口令，agent 侧改为 `changeme-0001` 占位符并与 server 侧对应 | `server/server.example.yaml`、`agent/agent.example.yaml`、`README.md` |
| — | **配置强校验**：`agent_tokens` 为空则拒绝启动（`TestLoadFileConfigRequiresAgentTokens` 覆盖） | `server/internal/server/config.go` |

**测试更新**：`TestUnlistedNodeRejected`（未登记节点任何 Token → 401）、`TestAgentTokenBinding`（跨节点 Token → 401、错误/空 Token → 401）、全部 config_test 改为 `agent_tokens` 语义。

**质量评价**：这是比上轮建议更严格的实现——"未登记即拒绝"消除了 Token 泄露后可冒名任意节点的路径（上轮"回退全局"模式在 Token 泄露后仍可伪造未登记节点）。配合启动日志输出登记节点数，运维可核对登记是否生效。**Token 认证这条线已闭环，达到当前架构下的最佳实践。**

### ❌ 仍未修复（自上轮延续）

| 条目 | 现状 |
|------|------|
| 明文 HTTP / 无 TLS | `agent.go` 仍 `http://` 明文；Server 无 TLS；会话 Cookie 无 `Secure` |
| 限流 / 完整超时 / DB 写放大 | `ReadHeaderTimeout: 5s` 仅此一项；`ingest` 无速率限制；`pruneMetrics` 每次上报全表 DELETE；`views()` 每次 GET 写离线告警 |
| 审计日志 | 无 |
| `-remove` 验证码 | 仍 `math/rand` + 6 位 |
| 登录防爆破 | 仍无失败锁定 |
| 配置护栏 | 仍无"非回环监听 + 未开鉴权拒绝启动" |
| 安全头 / XSS 收口 / history 参数 / probe_target 白名单 | 均未实施 |

### 上轮新观察项的状态

| 上轮观察 | 本轮状态 |
|----------|----------|
| 认证检查在 body 解析之后（未认证请求消耗解析 CPU） | 未修（`ingest` 仍先 `Decode` 再校验 Token）——DoS 面扩大点仍在，建议限流时一并处理 |
| `agent_tokens` 仅认空格缩进（Tab 会静默失效） | 未修，但**风险方向改变**：原"失效→回落全局（更宽松）"已不存在；现在 Tab 缩进失效 = 该节点无法上报（**fail-closed，安全方向**），危害降为"新节点登记后上不了线"的可用性问题，且启动日志的登记节点数可辅助发现 |
| `server.yaml` 权限未收紧 | **仍未修**：本轮只对 DB（`server.go:115`）与 `agent.yaml`（`config.go:50`）做 `0600`，`server.yaml` 含全部 `agent_tokens` 与登录口令，Linux 默认 0644 时本机其他用户可读 |

---

## 二、当前风险清单（第四轮）

| # | 严重度 | 风险 | 状态 |
|---|--------|------|------|
| 1 | 🟠 中危 | 明文 HTTP：独立 Token、登录口令、会话、节点数据可被嗅探（Wi-Fi/共享网段） | 未修复 |
| 2 | 🟠 中危 | 无请求限流 + 超时不完整 + DB 写放大（每次上报约 5 次写 + 全表 DELETE） | 未修复 |
| 3 | 🟠 中危 | 无审计日志（内部威胁/横向移动事件无法复盘） | 未修复 |
| 4 | 🟡 低危 | `server.yaml` 权限未收紧（全部节点 Token + 登录口令明文，默认 0644） | 未修（上轮已提） |
| 5 | 🟡 低危 | 认证检查在 body 解析之后 → 未认证请求消耗解析 CPU（配合无限流） | 未修（上轮已提） |
| 6 | 🟡 低危 | `-remove` 验证码 `math/rand` 6 位；登录无失败锁定 | 未修复 |
| 7 | 🟡 低危 | 配置护栏缺失、安全头缺失、XSS 手写 `esc()`、`history` 参数不一致、`probe_target` 无白名单、Cookie 无 `Secure` | 未修复 |
| 8 | 🟢 观察 | 新节点上线需预先在 `server.yaml` 登记并重启 Server（运维摩擦，安全/可用性权衡，文档已说明） | 设计取舍 |

---

## 三、通过项（确认维持）

- ✅ **Agent 上报认证（已闭环）**：强制 node_id 绑定、无全局 Token、未登记即拒绝、恒定时间比较、配置为空拒绝启动、完整测试覆盖；
- ✅ 登录鉴权：恒定时间比较、`crypto/rand` 32 字节会话、HttpOnly+SameSite Cookie、7 天过期、无会话固定、门控无绕过；
- ✅ DB 与 agent.yaml 权限 0600（非 Windows）；SQL 全参数化（含删除事务）；2 MiB 上报限制；服务端时间权威；`html/template` 自动转义；命令执行全参数化；无 `InsecureSkipVerify`；WAL；Token 启动脱敏。

---

## 四、建议（按当前剩余风险排序）

### P1
1. **限流 + 完整超时 + 降低写放大**：`http.Server` 补 `ReadTimeout/WriteTimeout/IdleTimeout`；`ingest` 按 IP 令牌桶限流（同时覆盖"未认证请求解析 CPU"问题，建议校验提前到解析出 `node_id` 即执行）；`pruneMetrics` 降频（按时间或行数阈值）；`views()` 离线告警"状态翻转才写库"。
2. **`server.yaml` 权限**：加载后 `os.Chmod(0600)`（非 Windows）+ 对含凭证配置文件做权限告警（与 DB/agent.yaml 对齐）。
3. **审计日志**：登录成败（来源 IP）、401 拒绝（node_id+来源）、告警触发/恢复、`-remove` 操作。

### P2
4. 管理面 TLS（`ListenAndServeTLS`）+ 会话 Cookie `Secure`；Agent 支持自定义 CA。
5. 登录防爆破（失败计数 + 按 IP 锁定）；`-remove` 验证码改 `crypto/rand` + 8 位。
6. 配置护栏（非回环 + 未开鉴权拒绝启动）；`agent_tokens` 解析兼容 Tab 缩进（避免新节点"静默登记失败"的可用性问题）。
7. 安全响应头、XSS 渐进收口、`history` 参数 `/` 过滤、`probe_target` 白名单。

---

## 五、结论

第四轮复审：**Token 认证体系已达到当前架构下的最佳实践并闭环**——无全局 Token、node_id 强制绑定、未登记拒绝上报、恒定时间比较、空配置拒绝启动、测试完整、示例无弱口令。这是整个项目中安全加固最彻底的一条线。

剩余风险全部为**低-中危的运维基线项**：明文 HTTP（内网可网段隔离先行）、限流/写放大（性能与 DoS 韧性）、审计日志（复盘合规）、`server.yaml` 权限（与既有 0600 策略对齐的小遗漏）。**均不阻塞当前内网生产上线**，建议按 P1 → P2 迭代补齐。
