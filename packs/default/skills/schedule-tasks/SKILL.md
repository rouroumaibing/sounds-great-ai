---
name: schedule-tasks
description: >
  定时任务注册、管理、能力指南。支持周期任务和一次性延迟任务。
  Use when: 用户想设定时任务、定期提醒、周期巡检、延迟执行。
  Not for: 已有 builtin 任务的手动触发。
  Output: 注册/管理定时任务，任务到点唤醒犬执行。
triggers:
  - "定时"
  - "schedule"
  - "cron"
  - "提醒"
---

# Schedule Tasks（定时任务）

## 触发类型

| 类型 | 配置 | 示例 |
|------|------|------|
| cron | `{"type":"cron","expression":"0 9 * * *"}` | 每天 9 点 |
| interval | `{"type":"interval","ms":3600000}` | 每小时 |
| once | `{"type":"once","delayMs":120000}` | 2 分钟后 |

## 流程

1. 确认触发类型 + 参数
2. preview 确认配置
3. 注册（需 operator 审批）
4. 任务到点 → 唤醒指定犬执行

## 注意
- 定时任务唤醒的犬有完整能力（rich block、search 等）
- 任务持久化，重启后恢复
