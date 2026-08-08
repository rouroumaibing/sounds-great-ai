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
