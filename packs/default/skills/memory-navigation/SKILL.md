---
name: memory-navigation
description: >
  记忆系统三入口路由（search_evidence / graph_resolve / list_recent）决策树 + 噪音控制。
  Use when: 没先验、压缩后回顾、"我记得最近讨论过 X"、search 反复 low-hit。
  Not for: 已有精确 anchor 直接 Read；代码符号查 Grep/LSP。
  Output: 选定入口 + 噪音控制参数 + 进入对应工具。
triggers:
  - "搜记忆"
  - "recall"
  - "之前讨论过"
  - "memory navigation"
---

# Memory Navigation（记忆三入口路由）

## 决策树

```
有精确 anchor（F186 / ADR-019）？
  → graph_resolve(anchor) — 精确图展开

有模糊 query？
  → search_evidence(query, mode=hybrid) — 语义+词面混合

零先验 / 冷启动 / "最近做了什么"？
  → list_recent(scope, since) — 时间窗口扫描
```

## 噪音控制

| 参数 | 作用 |
|------|------|
| `limit` | 控制 top-K，默认 5，最大 20 |
| `scope` | docs（结论）/ threads（过程）/ all（全貌）|
| `mode` | lexical（精确词面）/ semantic（向量）/ hybrid（混合+rerank）|
| `depth` | summary（默认）/ raw（全文）|

## 常见错误

- 用 `all` 期望全貌 → docs 密度压制 threads，分两刀搜
- 用 `semantic` 搜 Feature ID → 用 `lexical` 或 `hybrid`
- 搜到摘要就推理 → 摘要是索引不是答案，Read 原文
