---
name: memory-search-best-practices
description: >
  记忆系统多刀检索 + recall coverage 策略（8 类题型 recipe）。
  Use when: "哪些地方提过 X" / "X 的来源" / "有没有提过 Y" / 冷启动 / 搜了一刀觉得不够。
  Not for: 选第一刀走哪个工具（用 memory-navigation）。
  Output: 多 query 多 scope 召回 union + coverage matrix。
triggers:
  - "哪些地方"
  - "所有提到"
  - "coverage"
  - "source map"
---

# Memory Search Best Practices（多刀检索）

## 8 类题型

| 题型 | 刀数 | 策略 |
|------|------|------|
| "哪些地方提过 X" | 2-3 | docs + threads 分搜，union |
| "X 的来源/source map" | 2 | docs 找结论，threads 找讨论 |
| "有没有提过 Y" (absence) | 2 | 正向搜 + 反向搜，确认空集 |
| "上次到现在变了什么" (delta) | 2 | 旧时间窗 + 新时间窗对比 |
| 冷启动 onboard | 3-4 | 多角度扫，建立认知地图 |
| "X 和 Y 的关系" | 2 | X 搜 + Y 搜，找交集 |
| "X 的最新状态" | 1-2 | 搜 X + 按时间排序取最新 |
| 纯语义模糊 | 2 | hybrid + semantic blind-spot insurance |

## 何时停

- 2 刀都有高匹配 → 停
- 3 刀仍 low-hit → 换入口（graph_resolve / list_recent）
- coverage matrix 无 gap → 停

## 反模式

- 一刀命中就推理（碎片够了）→ 至少再搜一轮不同角度
- 只搜 docs 不搜 threads（或反之）→ 分道搜
- 用 all scope 期望全貌 → docs 压制 threads
