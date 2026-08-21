## A2A 协作格式

### @mention 语法

在消息中使用 `@代号` 提及队友，触发 A2A handoff：

```
@xigou 请搜索 ExecuteRequest 的所有调用点
@jinmao 检索关于 hook 注入的相关知识
@demu 追踪这个错误的日志
```

可用的 @mention 代号：
- `@bianmu` / `@边牧` — 任务分解与调度
- `@xigou` / `@灵缇` — 代码搜索与分析
- `@jinmao` / `@金毛` — 知识检索
- `@demu` / `@德牧` — 日志追踪与诊断
- `@zangao` / `@藏獒` — 输出格式化
- `@zhonghuatianyuanquan` / `@中华田园犬` — 安全审查

### Handoff 协议

当 agent A 在响应中 `@mention` 了 agent B，Platform 层会：

1. **记录 handoff**：在 A2A Thread 中记录 `Handoff{From: A, To: B, Artifact: 响应文本}`
2. **SOP 门禁**：检查 A2A 深度（默认 max=3），超限则升级到 CVO
3. **选择 reviewer**：跨狗狗 review 优先（`RequireDifferentBreed: true`）
4. **执行 B**：将 A 的输出作为 artifact 传给 B 执行

### A2A 消息格式

A2A 协作以 `@mention` 文本约定 + 球权账本（`internal/domains/custody`）为主路径，由 Platform 层的 `internal/a2a/hub.go`（内存 handoff 历史 + OpenTelemetry）辅助记录；日常犬间编排走 CLI adapter stdin/stdout pipe。

```
Thread {
    ID           string
    Query        string
    Participants []string
    Handoffs     []Handoff
    Depth        int
}
```

> **受控 A2A 协议客户端（§4.7）**：平台可作 **A2A 客户端**，经 Google A2A Protocol `tasks/send` JSON-RPC over HTTPS 调用**外部已部署 agent**（实现限 `internal/adapter/a2a/`，协议类型复用 `pkg/a2a/`，按 breed `variant.client_id="a2a"` + `a2a_url` 路由）。**禁止**新建 A2A HTTP **server**（暴露本地 agent 供第三方 push），也**禁止**引入 `internal/a2a/server/` 或 `internal/a2a/client/` 子目录（`不可逆决策` §4.1+§4.7）。

> **球权为编排一等状态源（§4.5）**：`BallLedger` 是 append-only 事件账本 + 纯函数 8 态投影，既服务 Trail/Brief 可观测，也参与运行守卫——`execution.go:handleA2AHandoff` 在派发前读 `BallLedger.Snapshot` 确认持球者，被第三方接管则中止 handoff（读驱动，非仅审计）。
