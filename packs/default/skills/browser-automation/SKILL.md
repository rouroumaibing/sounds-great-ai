---
name: browser-automation
description: >
  浏览器工作流总路由：为外部网站浏览、登录态流程、浏览器自动化选择合适后端。
  Use when: 需要操作外部网站、登录页、JS 重页面、证据采集。
  Not for: localhost 页面预览（用 browser-preview）、简单网页抓取。
  Output: 选定浏览器后端 + 执行路径 + 证据/结果。
triggers:
  - "浏览器"
  - "browser"
  - "自动化"
---

# Browser Automation（浏览器自动化）

## 路由决策

| 场景 | 后端 | 原因 |
|------|------|------|
| 简单网页抓取 | webfetch | 轻量、快 |
| JS 重页面 | browser tool | 需要 JS 执行 |
| 登录态流程 | browser tool | 需要保持 session |
| 证据采集 | browser tool + screenshot | 需要可视化证据 |

## 流程

1. 判断页面类型（静态/动态/需登录）
2. 选择后端
3. 执行操作
4. 收集证据（screenshot / DOM / network）
5. 输出结果
