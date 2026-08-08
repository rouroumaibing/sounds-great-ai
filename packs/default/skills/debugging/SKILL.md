---
name: debugging
description: >
  系统化 bug 定位：根因调查 → 模式分析 → 假设验证 → 修复。
  Use when: 遇到 bug、测试失败、unexpected behavior。
  Not for: 新功能开发、重构、已知原因的简单修复。
  Output: Bug report + 根因 + 修复（含回归测试）。
triggers:
  - "bug"
  - "报错"
  - "test failure"
  - "unexpected behavior"
---

# Debugging（系统性调试）

随机尝试修复浪费时间，症状修复掩盖真正问题。

**铁律：没有根因分析，不能提出修复方案。**

## 流程

1. **复现** — 确认 bug 可稳定复现，记录复现步骤
2. **定位** — 从症状向根因追踪，不跳步
3. **假设** — 形成根因假设，设计验证实验
4. **验证** — 实验确认或否定假设
5. **修复** — 修复根因（非症状），补回归测试
6. **确认** — 回归测试通过，原复现步骤不再触发
