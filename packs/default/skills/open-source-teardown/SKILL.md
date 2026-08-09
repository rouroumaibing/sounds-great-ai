---
name: open-source-teardown
description: >
  明星开源项目拆解：从宣传进入源码，验证真实架构、算法含量、营销水分。
  Use when: operator要求拆解热门 GitHub 项目、竞品 agent/runtime。
  Not for: 普通资料搜索（用 deep-research）。
  Output: 代码证据报告 + 对比结论 + 候选 lesson。
triggers:
  - "拆解"
  - "teardown"
  - "deep dive"
---

# Open Source Teardown（开源项目拆解）

## 铁律
不许只看 README 下判断。每个明星特性必须追到代码路径。

## 流程

### 1. 宣传面
- README / blog / PPT 声称什么特性？
- 哪些是 marketing 哪些是 real？

### 2. 源码验证
- 找到每个声称特性的代码路径
- 状态突变点在哪？
- 反馈闭环在哪？
- 算法输入输出是什么？

### 3. 评估
- 真本事 vs 水分
- 可学习点
- 不 follow 的 tradeoff

### 4. 输出
代码证据报告 + 对比结论 + 候选 lesson/skill
