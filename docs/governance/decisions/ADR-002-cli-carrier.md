# ADR-002: CLI Carrier 四档 Transport + 持久进程池 + PTY + Redis 健康度

> 本文是 `docs/governance/decisions/irreversible-decisions.md` §4.6 的展开 ADR。§4.6 为锁定决策摘要，本文件记录决策背景、代价与回滚。

## Status

Accepted（2026-08-14 新增；2026-08-15 按 provider 区分默认链细化）

## Context

CLI adapter 在保持「平台 spawn 外部 CLI、不内置 reasoning」（§4.1）的前提下，需要解决三类运行效率与鲁棒性问题：

- 每回合冷启动 CLI 进程开销大；
- 需要真 TTY 的 CLI（交互式）无法用纯 pipe 承载；
- 多实例部署下 carrier 健康度无法共享，易重复踩坑。

## Decision

CLI adapter 演进为 **carrier 抽象 + 四档 transport 降级链**（bg_daemon → interactive_pty → print_sdk → api_key）、**ACP 风格持久进程池**（warm 复用，规避每回合冷启动）、**PTY 交互载波**（为需要真 TTY 的 CLI 预留）、**Redis carrier 健康度 TTL**（quota 4h / structural 30min / transient 3 连击升级，跨实例共享）。

此为对 §4.1 的**细化而非推翻**：仍为外部 CLI 进程、仍不内置推理；新增 transport 形态与进程生命周期管理。

**carrier 抽象按 provider 区分默认链**（2026-08-15 细化，仍为 §4.1 细化而非推翻）：

- **claude / codex / gemini 默认 `bg_daemon → print_sdk`**（优先长会话，per-provider 长会话成熟度），各自 warm 池 + PTY runner 经 `WireWarmPools` 仅 `-tags pty` 编译接入、按 provider 构造专属 spawn func（claude/codex/gemini 三种 CLI 各一）；
- 未接入 warm 池时 registry 透明回退 one-shot（gating/standby，零新增依赖、行为等价旧版）；
- **opencode / kimi** 因 CLI 自身不支持长会话，维持单 transport one-shot；
- 持久池可经配置回退 one-shot；Redis 默认内存实现。

详细前后端链路与 `文件:行号` 锚点见 `docs/designs/FT-CLI-001-cli-adapter.md`。

## Consequences

**代价（回滚参考）：**

- 持久池带来僵尸 / lease / MCP 重建复杂度（R2）；
- PTY 增加伪终端复杂度（R3）；
- Redis 为**新外部依赖**（R6，默认内存实现、可配置切 Redis，无 Redis 时零新增依赖）。

**不变量（仍成立）：** §4.1 核心不变 —— 仍为平台 spawn 外部 CLI、平台层不内置 reasoning；新增 transport 仅是外部 CLI 进程的承载形态与生命周期管理。

**回滚：** 持久池可经配置回退 one-shot；Redis 默认内存实现，缺失 Redis 时优雅降级、零新增硬依赖。
