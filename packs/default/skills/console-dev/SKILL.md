---
name: console-dev
description: >
  Console 前端交付范式：4 道门禁驱动的前端开发流程。
  Use when: 新增前端能力、新增页面、重构布局。
  Not for: 小样式点改、纯后端 API。
  Output: 通过 Product/Design/Implementation/Verification gate 的前端代码。
triggers:
  - "前端"
  - "console"
  - "frontend"
---

# Console Dev（前端交付范式）

## 4 道门禁

### Gate 1: Product
- 这个 UI 解决什么用户问题？
- 交互流程是否清晰？
- 边界条件是否考虑？

### Gate 2: Design System
- 使用现有 design token？
- 组件复用 vs 新建？
- 响应式适配？

### Gate 3: Implementation
- TypeScript 类型完整
- 组件结构合理
- 状态管理清晰

### Gate 4: Verification
- 构建通过
- 视觉验证（browser preview）
- 交互验证
