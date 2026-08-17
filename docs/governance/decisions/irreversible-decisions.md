# 不可逆决策（Irreversible Decisions）

> 本文件原属 `docs/VISION.md` §4，2026-08-17 拆出为独立决策文档，便于单独维护与索引。
>
> 这些决策已锁定，后续 spec 不得推翻。引入新的不可逆决策时，在此追加并更新 `docs/governance/decisions/README.md` 索引。

---

### 4.1 CLI adapter 架构

平台 spawn 外部 CLI 进程，通过 stdin/stdout pipe 通信，解析各自输出格式。不内置 agent reasoning。

### 4.2 动态路由，非固定 DAG

平台根据任务类型动态决定调用哪些 agent。不硬编码工作流。

### 4.3 Dog personas 保留

狗狗各自承载 personality + role + CLI binding。

### 4.4 Go + Eino 平台语言

平台层用 Go + Eino。平台自身需要的 LLM 调用（路由、分解、合成）走 Eino。

### 4.5 球权账本为一等状态源（2026-08-13 新增）

SG 多 agent 编排以 append-only 事件流 + 纯函数投影状态机（`internal/domains/custody/`）为编排状态真相源；运行时组织为 `internal/domains/` 六边形层（适配器模式包装扁平包，Strangler Fig 逐域迁移）。不推翻前 4 条（仍走 CLI adapter、动态路由、平台层不内置推理）。详见 `docs/designs/FT-ORC-001-multi-agent-orchestration.md`。

### 4.7 受控的 A2A 协议客户端（2026-08-17 新增，放开原 §4.1 红线）

原 §4.1 红线「新建 A2A HTTP server/client」于 2026-08-17 经 operator 决策**修订**：

- **保留禁令**：平台**不内置 A2A server**（不监听外部入站 A2A 请求、不暴露本地 agent 作为可被任意第三方 A2A push 的 server）；平台 reasoning 仍不进 `internal/`，仍走 CLI adapter。即 §4.1 的「spawn 外部 CLI、不内置 reasoning」核心不变。
- **放开客户端**：允许平台作为 **A2A 协议客户端**，经 Google A2A Protocol（`tasks/send` JSON-RPC over HTTPS）调用**外部已部署 agent**（如另一套 SG 实例、独立 A2A agent）。实现位置限 `internal/adapter/a2a/`（与 CLI adapter 同级、实现 `unified.AgentExecutor`），协议类型复用 `pkg/a2a/`；按 breed `variant.client_id = "a2a"` 路由，外部地址经 `variant.a2a_url` 配置。**不引入 `internal/a2a/server/` 或 `internal/a2a/client/` 子目录**（原废弃约定保留）。
- **约束**：A2A 客户端是 CLI adapter 的「并列 carrier」，不是新编排层；球权账本（§4.5）仍是编排一等状态源，外部 A2A agent 的派发须经 `BallLedger` 守卫；外部 agent 仅作为被调用方，平台不为其开放入站回调之外的控制面。

> 修订动因：A2A 多智能体协作内核（@提及路由、链式 handoff、球权状态机）SG 已与 clowder 高度同构，唯一本质缺口是「对接外部 agent 的标准协议客户端」。放开该客户端使 SG 能接入异构 agent 生态，且不破坏 §4.1 的推理隔离与 server 禁令。

### 4.6 CLI carrier 四档 transport + 持久进程池 + PTY + Redis 健康度（2026-08-14 新增；详见 ADR-002）

CLI adapter 在保持「平台 spawn 外部 CLI、不内置 reasoning」（§4.1）前提下，演进为 **carrier 抽象 + 四档 transport 降级链**（bg_daemon → interactive_pty → print_sdk → api_key）、**ACP 风格持久进程池**（warm 复用，规避每回合冷启动）、**PTY 交互载波**（为需要真 TTY 的 CLI 预留）、**Redis carrier 健康度 TTL**（quota 4h / structural 30min / transient 3 连击升级，跨实例共享）。此为对 §4.1 的**细化而非推翻**：仍为外部 CLI 进程、仍不内置推理；新增 transport 形态与进程生命周期管理。详见 `docs/designs/FT-CLI-001-cli-adapter.md`。

代价与回滚见 ADR-002：持久池带来僵尸/lease/MCP 重建（R2）、PTY 增加伪终端复杂度（R3）、Redis 为**新外部依赖**（R6，默认内存实现、可配置切 Redis，无 Redis 时零新增依赖）。

carrier 抽象**按 provider 区分默认链**（2026-08-15 细化，仍为 §4.1 细化而非推翻）：**claude/codex/gemini 默认 `bg_daemon → print_sdk`（优先长会话，per-provider 长会话成熟度）**，各自 warm 池 + PTY runner 经 `WireWarmPools` 仅 `-tags pty` 编译接入、按 provider 构造专属 spawn func（claude/codex/gemini 三种 CLI 各一）；未接入 warm 池时 registry 透明回退 one-shot（gating/standby，零新增依赖、行为等价旧版）；opencode/kimi 因 CLI 自身不支持长会话，维持单 transport one-shot。持久池可经配置回退 one-shot；Redis 默认内存实现。
