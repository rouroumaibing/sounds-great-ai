# 架构谱系

从 thread 到 feature 的全量架构主题谱系。每个主题标注：定位、关键决策、依赖关系、当前状态。

---

## 1. 核心架构主题

### 1.1 三层分离（VISION §3）

| 层 | 职责 | 实现 | 不可逆决策 |
|----|------|------|-----------|
| Model | 推理 | 外部 CLI agent（Claude/Codex/Gemini/opencode） | §4.1 — 平台不做推理 |
| Agent CLI | 工具调用 | CLI adapter（stdin/stdout pipe） | §4.1 — 不建 HTTP A2A |
| Platform | 协调 | Go + Eino 后端 | §4.4 — Go 语言选择 |

**依赖方向**：Platform → Adapter → CLI（单向，不反向）

### 1.2 狗狗角色系统

| 主题 | 定位 | 关键决策 | 状态 |
|------|------|----------|------|
| Breed Config | 身份由 JSON 驱动 | 角色是数据，不是代码 | 完成 |
| Per-breed 身份文件 | CLAUDE.md / GEMINI.md / XIGOU.md / DEMU.md / ZANGAO.md / RURALDOG.md | 6 狗狗全覆盖 | 完成 |
| LL-010 落地 | 共享能力不绑定狗狗 | 限制声明表改为"倾向"非"硬约束" | 完成 |

### 1.3 记忆系统

| 主题 | 定位 | 关键文档 | 状态 |
|------|------|----------|------|
| 记忆哲学 | 7 公理 + 21 定律 + 判据 | `memory-philosophy.md` | 完成 |
| 记忆全景 | 6 器官互制 | `memory-system-overview.md` | 完成 |
| Memory Lanes | 证据/决策/教训存储 | `internal/memory/` | 完成 |
| RAG 检索 | ragstore 向量+BM25 | `internal/ragstore/` | 完成 |
| Cue Plane | 上下文线索管理 | `internal/cue/` | 完成 |

### 1.4 协作机制

| 主题 | 定位 | 关键文档 | 状态 |
|------|------|----------|------|
| @mention 路由 | A2A handoff 触发 | `s8-a2a-format` hook | 完成 |
| 传球三选一 | 自决/@句柄/等外部/@leader | `AGENTS.md` + `l3-ball-passing` hook | 完成 |
| hold_ball | 球权保管 + 定时唤醒 | `AGENTS.md` | 完成 |
| 决策漏斗 | 三层决策权限 | `decision-matrix.md` | 完成 |
| Magic Words | 10 个拉闸词 | `AGENTS.md` | 完成 |
| 治理协议 | hotfix/fallback/创意-实现解耦 | `AGENTS.md` | 完成 |

### 1.5 质量门禁

| 主题 | 定位 | 关键 skill | 状态 |
|------|------|-----------|------|
| TDD | Red-Green-Refactor | `tdd` skill | 完成 |
| Quality Gate | 交付自检 | `quality-gate` skill | 完成 |
| Merge Gate | 合入 main 门禁 | `merge-gate` skill | 完成 |
| Review Protocol | 跨狗狗 review | `request-review` / `receive-review` skill | 完成 |
| Cross-cat Handoff | 跨犬交接 | `cross-cat-handoff` skill | 完成 |
| Handoff Grounding | 接球真相核验 | `receive-handoff-grounding` skill | 完成 |

### 1.6 Hook 系统

| 系列 | 数量 | 注入时机 | 状态 |
|------|------|----------|------|
| S 系列 | 12 | session-init | 完成 |
| D 系列 | 21 | per-turn / dynamic | 完成（内容已充实） |
| L 系列 | 5 | lifecycle | 完成 |
| 其他 | 4 | bootstrap / callback / navigation / routing | 完成 |

### 1.7 Skills 系统

| 类别 | 数量 | 示例 | 状态 |
|------|------|------|------|
| 核心开发 | 5 | tdd / debugging / writing-plans / request-review / receive-review | 完成 |
| 治理门禁 | 4 | quality-gate / merge-gate / cross-cat-handoff / receive-handoff-grounding | 完成 |
| 协作 | 6 | cross-thread-sync / collaborative-thinking / thread-orchestration / code-as-harness / self-evolution / feat-lifecycle | 完成 |
| 记忆 | 2 | memory-navigation / memory-search-best-practices | 完成 |
| 上下文 | 2 | context-self-management / fresh-context-review | 完成 |
| 应急 | 2 | vision-rescue / incident-response | 完成 |
| 富媒体 | 3 | rich-messaging / workspace-navigator / browser-preview | 完成 |
| 生成 | 1 | image-generation | 完成 |
| **合计** | **25** | | 完成 |

---

## 2. 架构依赖图

```
VISION.md (北极星)
├── AGENTS.md (共享治理)
│   ├── Magic Words → meta-aesthetics.md
│   ├── 决策漏斗 → decision-matrix.md
│   └── 治理协议 → SOP.md
├── Per-breed 身份 (6 个)
│   └── Breed Config (JSON)
├── Hook 系统 (42 个模板)
│   ├── S 系列 → AGENTS.md 引用
│   ├── D 系列 → 动态注入
│   └── L 系列 → 生命周期
├── Skills 系统 (25 个)
│   └── 引用 AGENTS.md 治理机制
├── 记忆系统
│   ├── memory-philosophy.md (公理)
│   └── memory-system-overview.md (全景)
└── SOP.md (开发流程)
    └── public-lessons.md (教训)
```

---

## 3. 谱系维护

- 新增架构主题时更新本文档
- 新增 skill / hook 时更新对应表格
- 状态变更时更新"状态"列
- 本文是架构全貌索引，不是详细文档——详情查对应文档
