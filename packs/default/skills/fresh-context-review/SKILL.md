---
name: fresh-context-review
description: >
  Author-triggered fresh-context scan of PR diff before formal review.
  Finding generator, NOT approval authority.
  Use when: quality-gate 通过、PR 非 trivial、想降低正式 reviewer 认知负荷。
  Not for: 正式 review verdict、approval、merge decision。
  Output: Finding list（附在 review request 中）。
triggers:
  - "fresh context"
  - "PR diff scan"
  - "pre-review"
---

# Fresh Context Review（新鲜上下文审查）

在正式 review 前用全新上下文扫描 PR diff，生成 finding list。

## 流程

1. **获取 diff** — PR 的完整 diff
2. **新鲜上下文扫描** — 不带历史偏见，逐文件审查
3. **生成 findings** — 每条 finding 含：文件/行/问题/严重级/建议
4. **附在 review request 中** — 降低正式 reviewer 认知负荷

## Finding 格式

```
| 文件:行 | 问题 | 严重级 | 建议 |
```

严重级：P1（阻塞）/ P2（应修）/ P3（可选）

## 不做的事

- 不做 approval — 这是 finding generator，不是 approval authority
- 不做 merge decision
- 不替代正式 review — 是正式 review 的前置增强
