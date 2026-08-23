# Sounds Great AI Feature Roadmap

> 维护者：狗狗队伍
>
> 本 roadmap 反映 SG 实际梳理的代码级设计故事（Tech Story）。每条对应 `docs/features/FT-XXX-*.md`，
> 均基于 `internal/` + `web/src/` + `cmd/` 真实代码实读，锚点可回源。
>
> 面板类功能（plugins / marketplace / concierge / voice / connectors）的分阶段实现排期见
> [plans/panels-roadmap.md](plans/panels-roadmap.md)。

| ID | 主题 | 说明 |
|----|------|------|
| [FT-ORC-001](features/FT-ORC-001-multi-agent-orchestration.md) | Multi-Agent Orchestration | 多智能体编排总览：任务路由、球权、狗狗队伍协作的前后端逻辑 |
| [FT-A2A-001](features/FT-A2A-001-a2a-communication.md) | A2A Communication | A2A 通信（FT-ORC-001 的 A2A 子集细化）：@mention 路由、线程隔离、结构化交接 |
| [FT-CLI-001](features/FT-CLI-001-cli-adapter.md) | CLI Adapter | 6 个 CLI adapter（claude/codex/gemini/opencode/kimi/a2a）的 spawn contract 与 carrier 链 |
| [FT-CMR-001](features/FT-CMR-001-cross-model-review.md) | Cross-Model Review | 跨模型评审 / QC 7 步循环（`internal/sop/qc_loop.go`）的代码级逻辑 |
| [FT-PI-001](features/FT-PI-001-persistent-identity.md) | Persistent Identity | 持久身份：F231 关系胶囊 + F276 人物关系记忆 + Continuity |
| [FT-SM-001](features/FT-SM-001-shared-memory.md) | Shared Memory | 共享记忆：`internal/memory/lane_*.go` 的候选生产→审批→召回注入链路 |
| [FT-SKILL-001](features/FT-SKILL-001-skills-framework.md) | Skills Framework | Skills 框架：按需加载 prompt 系统的前后端逻辑 |
| [FT-PLUGIN-001](features/FT-PLUGIN-001-plugins-marketplace.md) | Plugins & Marketplace | 插件系统与市场（panels P1–P4）：zip 安装（zip-slip 防护）、skills 审查门禁、breed 注册、ed25519 验签安装；含 concierge/voice/connectors 配置面板 |
| [FT-ACC-001](features/FT-ACC-001-accounts-keys-auth.md) | 账户与密钥 | 设置页「账户与密钥」与「客户配置安全」设计 |
| [FT-MEM-001](features/FT-MEM-001-member-management.md) | 成员管理 | 设置页「成员管理」设计 |
| [FT-DEV-001](features/FT-DEV-001-makefile-daemon-reclaim.md) | 构建/开发环境 | Makefile 守护生命周期设计（dev/prod/daemon 等 target） |
| [FT-DS-001](features/FT-DS-001-dog-dossier.md) | Dog Dossier | 狗狗能力画像：特性全景（人设 × 能力档案 × 关系画像 × 行为评估四层拼图）+ 能力档案子系统交付（证据沉淀 × 蒸馏提案审批 × 名册认知路由，对齐 clowder F208） |
