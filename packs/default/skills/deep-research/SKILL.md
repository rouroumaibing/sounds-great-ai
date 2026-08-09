---
name: deep-research
description: >
  多源深度调研管道（Web Deep Research + 合成 + 云端模型咨询）。
  Use when: 技术问题需要多源调查、设计决策需要证据、operator说"调研"/"research"。
  Not for: 简单搜索（直接用 WebSearch）、已有结论的确认。
  Output: 调研报告 + 证据合成 或 咨询文档。
triggers:
  - "调研"
  - "research"
  - "调查"
---

# Deep Research（多源深度调研）

## 流程

### 1. 问题拆解
把调研问题拆成可独立搜索的子问题。

### 2. 多源搜索
- Web search（多角度关键词）
- 代码库内搜索（已有实践/教训）
- 云端模型咨询（如需）

### 3. 证据合成
- 交叉验证：≥2 个独立来源确认
- 标注 provenance：来源 + 时间 + 可信度
- 冲突标注：不同来源结论不一致时显式列出

### 4. 输出
调研报告结构：
- 结论（先给）
- 证据链（每个结论附来源）
- 不确定项（显式标注）
- 建议动作
