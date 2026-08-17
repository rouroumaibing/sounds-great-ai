---
name: receive-review
description: >
  处理 reviewer 反馈：修复 + 技术论证（禁止表演性同意）。
  Use when: 收到 review 结果、reviewer 提了 P1/P2、需要处理反馈。
  Not for: 发 review 请求、自检。
  Output: 逐项修复确认 + reviewer 放行。
triggers:
  - "review 结果"
  - "review 意见"
  - "reviewer 说"
  - "fix these"
---

# Receive Review

处理 reviewer 反馈的完整流程。核心原则：**技术正确性 > 社交舒适，验证后再实现，禁止表演性同意。**

## 流程

1. **逐项确认** — 每个 review 意见都明确回应：同意 / 反驳（附技术论证）
2. **修复** — 对同意的意见，修复并验证
3. **反驳** — 对不同意的意见，给出技术论证，不是"我觉得"
4. **放行** — 所有 P1 解决后，请 reviewer 确认

禁止表演性同意（"好的好的"但不改）。禁止盲目实现（reviewer 说啥就做啥，不验证）。

## Reviewer Delta Annotation（跨模型 review 增值量化）

作为 reviewer，在自己的 findings 中标注 delta tag，量化"非作者模型"带来的增量价值：

| Tag | 含义 | 用途 |
|-----|------|------|
| `[delta:covered]` | 该 finding 作者已自行发现 | 量化作者自检覆盖率 |
| `[delta:new]` | 该 finding 是作者**未发现**的新发现 | 量化跨模型 review 增值 |
| `[delta:N/A]` | 该 finding 不适用 delta 标注（愿景级/架构级） | 排除非代码级 finding |

**格式**：在 finding 行末加 tag

```
P2-1: 边界条件未处理 — src/foo.ts:42 [delta:covered]
P1-1: 并发写入 race condition — src/bar.ts:18 [delta:new]
P3-1: 建议重新考虑整体架构方向 [delta:N/A]
```

**注意**：
- 标注是轻量 annotation，不增加 review 流程摩擦。
- 无作者自检结论时（未附带自查明细），不标注。
- Delta 数据自然累积在 review 记录中，由 eval 聚合分析（`reviewer_delta` 指标 = new / (new + covered)）。
- 标注不影响 finding 的 severity 判定或处理流程。

## 直连 review carrier（写回路由）

写回 verdict 时，权威来源是 **direct review carrier**——直接承载 review 请求、发起 review 的 thread，不是任务祖先 thread，也不是第一次误投的落点。若两者冲突，停止沿错路级联，回 direct review carrier。

- 只有在 direct review carrier 向指定 reviewer 发修复确认，才等待 reviewer 对当前 SHA 放行。
- 同一 `dog_id` 自我写回、或非指定 reviewer 写回，均被 fail-closed 拒绝。
- cloud / GitHub review 修复后只重新触发 cloud review，等 PR truth source；local peer 只在非 cloud 行为 delta / scope 扩大时介入。
