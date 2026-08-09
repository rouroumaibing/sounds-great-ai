---
name: image-generation
description: >
  AI 图片生成：原生 tool call 或浏览器自动化。
  Use when: 需要 AI 生成概念图、UI 参考、像素画素材、架构图、信息图。
  Not for: 已有图片的展示（用 media_gallery rich block）。
  Output: 生成图片自动发布，或作为素材进入后续交付。
triggers:
  - "生成图"
  - "画个图"
  - "image generation"
  - "AI生图"
---

# Image Generation（AI 图片生成）

## 路径选择

| 路径 | 适用 | 说明 |
|------|------|------|
| 原生 tool call | Codex / Antigravity | 直接调用 image generation tool |
| 浏览器自动化 | Gemini / ChatGPT | 通过浏览器操作 AI 生图服务 |

## 流程

1. 确定生成路径（看当前 CLI adapter 能力）
2. 构造 prompt（描述要生成的图片）
3. 生成图片
4. 图片自动发布或作为素材进入后续交付

## 不做的事

- 已有图片的展示 → 用 `media_gallery` rich block
- 可编辑/native text 的图表 → 用 HTML/PPT 管线
