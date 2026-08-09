---
name: context-self-management
description: >
  Use when: context_management_hint 或长 session repo/projectPath 混淆。
  Not for: 普通 context 焦虑/跨犬交接。
  Output: handoff/续/冲刺判断%0
triggers:
  - "context management"
  - "上下文管理"
  - "压缩后失忆"
---

# Context Self Management（上下文自管理）

. **收到 context_management_hint(warn)** → 自检：
   - 当前 session 还在做什么？
   - 剩余上下文够完成当前任务吗？
   - 需要压缩/交接/冲刺？

2. **判断**：
   - 够 → 继续
   - 不够但快完成 → 冲刺（减少工具调用，直奔终点）
   - 不够且不快完成 → handoff（写五件套，propose_session_handoff）

3. **handoff** → `propose_session_handoff(done, nextSteps, worktreeBranch, commits, gotchas)`
