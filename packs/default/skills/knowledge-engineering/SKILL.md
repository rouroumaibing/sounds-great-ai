---
name: knowledge-engineering
description: >
  犬犬指导外部项目文档重构 — AI FDE 知识工程方法论。
  Use when: 犬犬部署到外部项目、用户项目缺少结构化文档、冷启动理解业务。
  Not for: sounds-great-ai 项目自身开发、已有完善 docs/ 结构的项目。
  Output: 文档现状诊断 + 路径选择 + 三层知识注入建议。
triggers:
  - "知识工程"
  - "knowledge engineering"
  - "文档重构"
---

# Knowledge Engineering（知识工程）

## 流程

### 1. 现状诊断
- docs/ 结构是否清晰？
- 真相源是否明确？
- 知识是否可检索？

### 2. 路径选择
- 轻量补文档（结构 OK，内容缺）
- 重构文档结构（结构乱）
- 从零建文档（无文档）

### 3. 三层知识注入
- L1: 概览层（README / VISION）
- L2: 治理层（SOP / decisions / lessons）
- L3: 执行层（API / cli / architecture）

### 4. 输出
文档骨架模板 + 注入建议 + 优先级排序
