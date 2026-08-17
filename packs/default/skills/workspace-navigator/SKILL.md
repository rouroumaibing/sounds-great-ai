---
name: workspace-navigator
description: >
  狗狗把"打开文档、代码或日志"等模糊意图解析成本地绝对路径或 worktree 相对路径。
  Use when: 想说"文件在 X 路径"、operator 问"打开 X"。
  Not for: HTTP 链接、localhost app（用 browser-preview）。
  Output: applied/queued/blocked/unconfirmed 投递状态。
triggers:
  - "打开文件"
  - "workspace navigate"
  - "文件在"
---

# Workspace Navigator（工作区导航）

用 `workspace_navigate(path, action)` 打开或揭示文件。

## 流程

1. 解析路径：绝对路径 or worktree 相对路径
2. 选择 action：`reveal`（目录/不确定）或 `open`（文件）
3. 调用 `workspace_navigate`
4. 检查 `deliveryStatus`：applied / queued / blocked / unconfirmed

## 注意

- `ok: true` 只表示请求被接受，不表示文件已可见
- 只有 `applied` 证明 Hub 客户端实际改变了 Workspace 状态
