---
name: guide-authoring
description: >
  标准引导流程设计 SOP：场景识别 → YAML 编排 → 标签 → 注册 → 测试。
  Use when: 新建引导流程、添加场景引导、编写引导 YAML。
  Not for: 使用引导（用户侧）、视觉设计。
  Output: Flow YAML + tag-manifest + registry 注册。
triggers:
  - "写引导"
  - "E"guide authoring"
  - "flow design"
---

# Guide Authoring（引导流程设计）

## 流程

### 1. 场景识别
- 用户会在什么情况下需要引导？
- 引导的起点和终点是什么？

### 2. YAML 编排
- steps 按顺序排列，每步含 title + body + 可选 action
- 支持条件分支（condition）

### 3. 标签标注
- category: onboarding / config / feature-tour
- priority: high / medium / low

### 4. 注册 + 测试
- 注册到 guide registry
- 测试流程是否走通
