---
name: writing-plans
description: >
  将 spec/需求拆分为可执行的分步实施计划。
  Use when: 有 spec 或需求，准备动手前需要拆分步骤。
  Not for: trivial 改动（≤5 行）、已有详细计划。
  Output: 分步实施计划（含 TDD 步骤和检查点）。
triggers:
  - "写计划"
  - "implementation plan"
  - "拆分步骤"
---

# Writing Plans

将 spec/需求拆分为分步实施计划。写清楚每步改哪些文件、代码、测试、怎么验证。DRY. YAGNI. TDD. Frequent commits.

## Straight-Line Check

**Before splitting steps, do this first:**

1. **Pin the finish line**: one-sentence definition + acceptance criteria + "what we're NOT building"
2. **Define terminal schema**: interfaces / types / data structures of the final form
3. **Every step passes three questions:**
   - Will this step's output stay in the final system as-is? → Yes = on the line
   - What can we demo/test after this step? (no verifiable evidence = detour)
   - If we remove this step, what specific cost does it add? (can't articulate = detour)
