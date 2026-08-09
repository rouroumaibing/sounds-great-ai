---
name: writing-skills
description: >
  创建/修改 skill 的元技能（含质量标准、范本、发布）。
  Use when: 写新 skill、修改现有 skill、验证 skill 质量。
  Not for: 使用 skill（直接触发对应 skill）。
  Output: 新/更新的 SKILL.md + manifest 条目。
triggers:
  - "写skill"
  - "writing skills"
  - "skill质量"
---

# Writing Skills（写 Skill 的元技能）

## 质量门禁

### 1. 价值门禁
- 这个 skill 解决什么问题？
- 没有这个 skill 用户会怎样？
- 模型已知通用教程不算价值

### 2. 格式标准
- YAML frontmatter 完整（name, description, triggers）
- description 含 Use when / Not for / Output
- body 有流程步骤，不是概念堆砌

### 3. 触发词
- 触发词具体且不与其他 skill 冲突
- 中英文触发词都列

## 流程

1. 确认 skill 价值（过门禁）
2. 写 SKILL.md（参考范本）
3. 注册到 manifest
4. 测试触发是否正确
