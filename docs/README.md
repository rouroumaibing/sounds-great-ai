# Documentation

> 结构总览。每条文档的"真相源"角色见 `AGENTS.md` 的 Truth Sources 表。

## Structure

| Directory / File | Purpose |
|------------------|---------|
| `VISION.md` | 北极星 — 仅方向与理念（§0–§3）。治理/架构/路线图已拆分至下方各文件 |
| `SOP.md` | 开发 SOP — 风险路由、review、质量工具 |
| `public-lessons.md` | 经验教训 — LL-XXX 条目、可执行原则 |
| `ROADMAP.md` | 项目路线图（Phase 进度） |
| `reference/cli.md` | CLI 安装指南 |
| `reference/API.md` | API 参考 |
| `governance/` | 治理类文档 |
| `governance/decision-matrix.md` | 决策漏斗（三层决策、可逆性、硬排除） |
| `governance/vision-compliance.md` | 愿景合规（spec 检查清单、Prompt Hooks 系统） |
| `governance/meta-aesthetics.md` | 元美学（第一性原理 magic word 展开） |
| `governance/decisions/` | 架构决策记录（不可逆决策 + ADR-XXX） |
| `architecture/` | 平台架构（能力清单、记忆、协作、谱系） |
| `architecture/platform-capabilities.md` | 平台能力与狗狗角色（原 VISION §5/§6） |
| `architecture/memory-philosophy.md` | 记忆系统哲学（7 公理 + 21 定律） |
| `architecture/memory-system-overview.md` | 记忆系统全景（6 器官） |
| `architecture/collaboration-landscape.md` | 协同全景（@mention / 球权 / 决策权限） |
| `architecture/architecture-lineage.md` | 架构谱系（全量主题索引） |
| `architecture/overview.md` | 架构总览（三层 + A2A + 数据流 mermaid） |
| `designs/` | 子系统"代码级 Tech Story"（FT-XXX-001，带 `文件:行号` 锚点） |
| `plans/` | 实现计划 / 实现记录 |
| `brand/` | 产品叙事与品牌（故事、角色设定、头像图） |
| `features/` | 特性文档占位（F-XXX）；当前特性设计见 `designs/` 的 SG-XXX |

## 命名约定

- **VISION / SOP / ROADMAP / public-lessons**：顶层方向与方法论文档。
- **governance/**：治理流程与决策（decision-matrix / vision-compliance / meta-aesthetics + decisions/）。
- **architecture/**：平台"是什么"（能力、记忆、协作、谱系）。
- **designs/（FT-XXX-001）**：具体子系统的代码级设计真相源（前后端链路 + `文件:行号`）。
- **plans/**：实现计划或已完成实现的记录。
- **reference/**：对外接口/安装类运维文档（API / cli）。
- **brand/**：产品叙事，与工程技术文档解耦。

## Truth Sources

完整真相源索引见 `AGENTS.md`。
