---
name: organize-threads
description: >
  犬犬辅助整理未分类 thread，分析标题和元数据，建议合适的标签。
  Use when: 用户说"帮我整理"、"分类 thread"。
  Not for: 删除/编辑标签本身。
  Output: 按 thread 的标签建议列表。
triggers:
  - "整理"
  - "organize"
  - "分类"
---

# Organize Threads（整理 Thread）

## 流程

### 1. 扫描
获取未分类或标签不全的 thread 列表。

### 2. 分析
对每个 thread：
- 标题关键词提取
- 消息内容主题判断
- 活跃度 + 参与者

### 3. 建议标签
基于分析结果，建议 1-3 个标签：
- feature（讨论某个功能）
- bug（修 bug）
- design（设计讨论）
- ops（运维/部署）
- research（调研）

### 4. 输出
标签建议列表，operator 确认后批量应用。
