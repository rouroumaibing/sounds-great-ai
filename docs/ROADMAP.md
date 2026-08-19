# Roadmap

> 本文件承接原 `docs/VISION.md` §7（2026-08-17 拆出）。VISION 现只保留方向与理念（§0–§3）。

## Phase 进度总表

| Phase | 目标 | 状态 |
|-------|------|------|
| **1. Platform Infra** | CLI adapter + config + router + SOP + skills + memory + MCP + hooks | **完成** |
| **2. RAG Integration** | 向量存储接入平台、context_assemble、FTS5 混合检索 | **完成** |
| **3. A2A Coordination** | 多 agent 动态协作、@mention 路由 | **完成** |
| **4. Skills System** | skill 加载、注入、外部 skill 吸收 | **完成** |
| **5. SOP Gates** | 质量门禁、review 流程、安全策略 | **完成** |
| **6. Transport** | WebSocket + HTTP API + 前端 | **完成** |
| **7. Polish** | 文档、示例、性能优化、hook 模板、Memory System + Cue Plane + ACP Pool + PWA | **主体完成，剩余子项** |
| **8. Skills Framework 治理化** | 持久化意图 + carrier 挂载 + sync 调谐 + drift 治理 + HTTP API + 前端面板（扩展 Phase 4） | **Planned** |

## Phase 7: Polish (In Progress)

| Item | Status |
|------|--------|
| 45 hook templates | Completed |
| Memory System (Lanes + Cue Plane) | Completed |
| ACP Process Pool | Completed |
| PWA | Completed |
| 文档治理 (AGENTS.md governance) | Completed |
| Skills (5→42) | Completed |
| RAG on-demand retrieval | Planned |
| SOP basic gates | Planned |

## Future

- Eval framework
- Telemetry dashboard
- Multi-project support

## Phase 8: Skills Framework 治理化 (Planned)

> 设计 Spec：`docs/plans/skills-framework-governance-spec.md`。扩展 Phase 4（Skills System 已完成），前置 Phase 1–7 已满足。
> 关键架构分歧（D1–D5）：clowder 靠 CLI 原生 symlink 读 skill 正文；SG 经 Prompt Builder 注入为主、claude 可选 symlink 为辅。不引入新不可逆决策。

| Item | Status |
|------|--------|
| 8.1 数据模型 + 持久化（skills-config.json + SkillManager 重写 + 废弃 agent.SkillManager） | **Done (2026-08-18)** |
| 8.2 HTTP API 扩展（GET 全量/:id / PATCH / sync / drift） | **Done (2026-08-18)** |
| 8.3 Build 集成（SkillIDs 注入生产流 + 删死路由 helper） | **Done (2026-08-18)** |
| 8.4 d11 resolver 真实匹配（填 template 占位） | **Done (2026-08-18)** |
| 8.5 可选物理挂载（claude `.claude/skills` symlink reconciler + drift 治理） | **Done (2026-08-18)** |
| 8.6 前端 SkillsPanel + Drift UI | **Done (2026-08-18)** |
| 8.7 Cross-model review + CI gate | Done (2026-08-18) |
