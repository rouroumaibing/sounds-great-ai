---
name: worktree
description: >
  为代码、脚本、API 创建隔离 Git worktree。
  Use when: 需要隔离开发环境、并行实验、不影响主分支。
  Not for: 简单改动（直接在主 worktree 改）。
  Output: 隔离的 Git worktree + 配置。
triggers:
  - "worktree"
  - "隔离"
  - "isolate"
---

# Worktree（隔离工作区）

## 流程

### 1. 创建
```bash
git worktree add ../sounds-great-ai-feature -b feature/xxx
```

### 2. 隔离配置
- 独立 Redis 端口（避免数据冲突）
- 独立配置文件
- 独立 build 产物

### 3. 工作
在隔离 worktree 中开发，不影响主分支。

### 4. 清理
完成后：
- merge 回主分支
- `git worktree remove`
- 清理隔离配置

## 注意
- 共享文件只在 main 改，改完立刻 commit + push
- worktree 间不共享状态
