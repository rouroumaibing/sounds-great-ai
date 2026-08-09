## A2A 协作格式（对标 clowder-ai S4 a2a-format）

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

A2A 通信通过 stdin/stdout pipe（非 HTTP），由 Platform 层的 `internal/a2a/` 管理：

```
Thread {
    ID           string
    Query        string
    Participants []string
    Handoffs     []Handoff
    Depth        int
}
```

> **实现状态**：`internal/a2a/` 当前为 Minimal 实现。@mention 是主要协作模式，A2A Hub 是辅助。不建 HTTP server/client（VISION §4.1）。

> **不可逆决策**：A2A 通信走 CLI adapter pipe，不建 HTTP server/client（VISION §4.1）。
