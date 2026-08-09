---
name: source-audit
description: >
  外部高风险 claim 与研究贡献审计。
  Use when: 数字、benchmark、因果、趋势、模型能力、外部论文或会进入 docs/ADR 的结论。
  Not for: 低风险常识、只读官方原文且不外推。
  Output: claim ledger + source/non-triviality/decision-fit 三轴 verdict + provenance。
triggers:
  - "audit"
  - "审计"
  - "source audit"
---

# Source Audit（来源审计）

## 三轴评估

### 1. Source 轴
- 一手来源 vs 二手转述
- 利益冲突检查
- 时效性（数据/结论的适用时间窗口）
- 对象适用性（结论针对的对象 vs 我们的对象）

### 2. Non-triviality 轴
- 结论是否 trivially true（不需要引用也成立）
- 是否提供了非显而易见的洞察

### 3. Decision-fit 轴
- 结论是否影响我们的决策
- 是否需要降级/升级决策权重

## 输出

```
Claim: <原始 claim>
Source: <来源 + provenance>
Verdict: verified | mismatch | insufficient
Note: <审计说明>
```
