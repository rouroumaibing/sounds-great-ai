---
name: quality-gate
description: >
  开发完成后的自检门禁：愿景对照 + spec 合规 + 验证。
  Use when: 准备对交付作完成声明、需要整理风险匹配的自证。
  Not for: 收到 review 反馈（用 receive-review）、merge（用 merge-gate）。
  Output: Spec 合规报告（含愿景4覆盖度）。
triggers:
  - "完成"
  - "交付"
  - "quality gate"
  - "自检"
---

# Quality Gate（交付自检门禁）

交付前自检。不是走形式，是给自己一个结构化的"真的做完了吗"检查。

## 检查清单

### 1. 愿景对照

- 这个交付物服务了 VISION.md §0 的哪个目标？
- 与 §4 不可逆决策兼容？（不重新引入 DAG / 内置 reasoning / 非 CLI 架构）
- 与 §3 三层原则兼容？（不把推理放进平台层）

### 2. Spec 合规

- 如果有 spec：逐条验证 Acceptance Criteria，每条附证据
- 如果无 spec（直接改代码）：AGENTS.md 时刻 2 提交前检查通过？

### 3. 风险匹配

五轴风险判断（行为/数据/安全/契约/不可逆），每轴回答：
- 命中了吗？
- 命中后选了什么车道？
- 车道要求的验证做了吗？

### 4. 验证证据

| 检查 | 命令 | 结果 |
|------|------|------|
| Go build | `go build ./...` | ✅/❌ |
| Go test | `go test ./...` | ✅/❌ |
| Go vet | `go vet ./...` | ✅/❌ |
| 文件大小 | 200 行警告 / 350 行硬限 | ✅/❌ |
| 红旗模式 | AGENTS.md 红旗表 | ✅/❌ |

### 5. 红旗自检

- 是否引入了红旗模式表中的任何模式？
- 是否新增 `internal/` 顶层目录而未更新 VISION.md §6？
- 是否跨了 Phase 边界？

**任何一项 ❌ → 不交付，先修。**
