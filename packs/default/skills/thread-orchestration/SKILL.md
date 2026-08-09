---
name: thread-orchestration
description: >
  大任务的主动拆解与多 thread 并行编排。
  Use when: 任务涉及 2+ 个独立可交付子任务，需要不同犬参与、不同 thread 并行推进。
  Not for: 单一任务、已有 thread 间被动协调、单 session 内并行。
  Output: 子 thread 创建 + 选犬 + 各 thread 交付 + 主 thread 汇聚报告。
triggers:
  - "拆解任务"
  - "多thread"
  - "并行编排"
  - "orchestration"
---

# Thread Orchestration（线程编排）

## 流程

1. **拆解** — 识别独立可交付子任务
2. **选犬** — 每个子任务选最合适的狗狗
3. **创建子 thread** — `propose_thread(title, reason, preferredCats, projectPath)`
4. **分发** — 各子 thread 独立推进
5. **汇聚** — 主 thread 收集各子 thread 结果

## projectPath

子 thread 的工作区/真相源归属，不是外部目标仓。

## 报告模式

| 模式 | 适用 |
|------|------|
| `final-only` | 做完后需要结果回来（默认） |
| `none` | 下游自治理，不需要报告 |
| `state-transitions` | 需要阶段进度更新 |
| `blocking-ack` | 需要每步阻塞确认 |

## 不做的事

- 单一任务直接做，不拆
- 已有 thread 间协调用 cross-thread-sync
- 单 session 内并行用 CLI 内置能力
