---
name: collaborative-thinking
description: >
  单犬或多犬的创意探索、独立思考、讨论收敛。
  Use when: brainstorm、多犬独立思考、讨论结束需要收敛、方向性问题需要多视角。
  Not for: 已有明确 spec 直接写代码、单犬执行已定方案。
  Output: 收敛报告（共识/分歧/行动项）+ 三件套沉淀检查。
triggers:
  - "brainstorm"
  - "讨论"
  - "多视角"
  - "收敛"
---

# Collaborative Thinking（协作思考）

## 流程

1. **独立思考** — 每只犬先独立给出视角，不互相影响
2. **交叉碰撞** — 分享视角，找分歧和盲区
3. **收敛** — 共识/分歧/行动项分类
4. **沉淀检查** — 有没有可复用的方法论/教训/决策

## 收敛报告格式

```
## 共识
- ...

## 分歧
- A 认为 X，理由 ...
- B 认为 Y，理由 ...

## 行动项
- [ ] 需要验证：...
- [ ] 需要决策：...（附 Decision Packet）
```

## 多犬模式

用 `multi_mention(targets, question, callbackTo)` 并行收集独立视角，然后收敛。
