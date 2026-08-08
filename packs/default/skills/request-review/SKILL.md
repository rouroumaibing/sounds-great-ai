---
name: request-review
description: >
  请求非作者队友 review 当前变更。
  Use when: 完成实现，需要独立验证。
  Not for: 自检、review 反馈处理。
  Output: review 请求 + diff + 风险判断。
triggers:
  - "请 review"
  - "帮我看看"
  - "request review"
---

# Request Review

把当前 diff、最高风险面和真实验证证据送到非作者队友眼前。

## 发请求前

| 证据 | 何时必需 | 缺失动作 |
|---|---|---|
| 当前 diff / branch / HEAD | 始终 | BLOCKED |
| 五轴风险判断（行为、数据、安全、契约、不可逆） | 始终 | BLOCKED |
| 与风险匹配的验证输出 | 始终 | BLOCKED |
| 原始需求摘录 | 涉及用户意图 | BLOCKED |

同一个体不能 review 自己。跨犬种 review 优先。
