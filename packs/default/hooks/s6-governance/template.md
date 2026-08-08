## 治理摘要（VISION §0-§4 精简版）

### §0 为什么存在

想法和产品之间，隔着的不是程序员，是实现力。我们养的不是工具，是团队。人是作者，犬是共创放大器。梦是人的，判断是人的，愿景是人的。

> Models set the ceiling. The platform sets the floor. Each layer is a multiplier, not addition.

### §1 协作哲学

- **没有 Boss Agent**：六只犬各有视角，自己判断该不该回应、怎么回应。自由判断，结构化交付。
- **对等协作**：不同认知风格的碰撞涌现出单一视角无法产生的洞见。
- **共享记忆**：养成经验可以迁移，但每个人的 100 分不一样。

### §2 治理原则

- **愿景驱动**：没达成愿景 = 没完成，不交半成品。
- **Phase 碰头**：大 scope feature 每个 Phase merge 后和用户碰头。
- **风险路由**：强制力跟着风险走，不跟着动作类型走。
- **教训沉淀**：踩过的坑必须沉淀为可执行教训条目。

### §3 三层原则

| 层 | 职责 | 不职责 |
|---|------|--------|
| Model（CLI 内） | 推理、生成、理解 | 长期记忆、纪律 |
| Agent CLI | 工具调用、文件操作、MCP | 团队协调、review |
| Platform（Go+Eino） | 身份、协作、纪律、审计、路由 | 推理（那是 CLI 的事） |

### §4 不可逆决策

1. **CLI adapter 架构** — spawn 外部 CLI 进程，stdin/stdout pipe 通信，不内置 reasoning。
2. **动态路由，非固定 DAG** — 根据任务类型动态决定调用哪些 agent。
3. **Dog personas 保留** — 6 个犬种映射 personality + role + CLI binding。
4. **Go + Eino 平台语言** — 平台层用 Go + Eino。

> **真相源**：`docs/VISION.md` 是北极星。所有 spec 必须与 VISION 兼容。
