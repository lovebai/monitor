# 配置生成器工具

`config-generator.html` 是一个零依赖的**可视化配置生成器**，浏览器直接打开即可使用，无需安装任何依赖或启动服务。

## 使用方法

双击 `config-generator.html`（或拖入浏览器）：

1. **Server 配置**：填写监听地址、阈值、历史保留天数、登录鉴权，并登记各 Agent 的独立 Token（node_id + Token，至少一项）；点击“生成 server.yaml”，可复制或下载。
2. **Agent 配置**：填写 Server 地址、节点 Token / node_id、分组、进程与服务检查等（每台被监控设备一份）；点击“生成 agent.yaml”，可复制或下载。
3. 快捷操作：Agent 面板的“登记到 Server agent_tokens”按钮会把当前节点 Token 直接加入左侧 Server 登记列表，避免两边手填不一致。

节点 Token 输入框旁的“随机”按钮可一键生成 16 位英文大小写组合（基于 `crypto.getRandomValues` 加密安全随机数）。
开启登录鉴权后，密码输入框旁的“生成加密哈希”按钮会在浏览器本地用 Web Crypto 计算 PBKDF2-HMAC-SHA256 哈希（210000 次迭代 + 随机盐），生成的 `server.yaml` 将写入加密后的 `auth_password`，与服务端 `server.exe -gen` 输出格式完全一致；不生成哈希则输出明文密码（服务端仍兼容，但不推荐）。

## 生成格式说明

- 生成的 `server.yaml` 与 `server/internal/server/config.go` 的解析格式一致（`agent_tokens` 为缩进映射）；`auth_password` 支持加密哈希或明文（明文仅用于迁移兼容）。
- 生成的 `agent.yaml` 与 `agent/internal/agent/config.go` 的解析格式一致（支持 JSON 或简单 YAML，本工具输出简单 YAML）。
- 值中若包含 `#` 或引号会自动加引号，避免被当作注释。

> 提示：生成后请把真实 Token / 密码替换为强随机值，并按 README 说明轮换与保管。
