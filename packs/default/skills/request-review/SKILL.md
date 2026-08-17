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

## 身份不变量（硬约束，fail-closed）

Review 必须由**与作者不同的 dog 身份**执行。身份以执行该 breed 的 variant（模型）的 `dog_id` 为准——两只狗解析到不同的 `dog_id` 即视为不同身份（跨模型），无论它们是否同属一个 breed。

- **`reviewer dog_id` 必须 ≠ `author dog_id`**。同一 `dog_id` 自我 review 在交接与写回两处均被 fail-closed 拒绝。
- 跨 dog_id = 跨模型 review（如 bianmu 写、xigou 审），优先选不同 `dog_id`（不同模型）的队友。
- 当 variant `dog_id` 不足以判定时，回退到 breed `dog_id`；两者都为空则按"无法判定身份"处理，不默认放行。

写回 verdict 时：只有被指定的 reviewer（`dog_id` + 发起 review 的 thread）可写回；其他人写回、或写回到非直连 review thread，都会被拒。
