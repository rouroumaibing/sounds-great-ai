---
name: browser-preview
description: >
  Hub 内嵌浏览器预览 localhost 应用。
  Use when: 写前端代码、跑 dev server、需要看页面效果、调 UI。
  Not for: 后端纯 API 开发、不涉及页面的工作。
  Output: 前端页面在 Hub browser panel 中实时预览。
triggers:
  - "看看效果"
  - "browser preview"
  - "预览"
  - "打开页面"
---

# Browser Preview（浏览器预览）

用 `preview_open(port, path)` 在 Hub Browser panel 中打开 localhost 应用。

## 流程

1. 确认 dev server 已启动（如 `npm run dev` → localhost:5173）
2. 调用 `preview_open(port, path)`
3. Hub Browser panel 自动打开

## 注意

- 先验证 dev server 正在运行
- 路径默认 `/`
- 可选传 `worktreeId` / `threadId` / `catId`
