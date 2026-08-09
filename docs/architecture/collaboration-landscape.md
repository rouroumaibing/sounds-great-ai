# Sounds Great AI 协同全景

> 对标 clowder-ai `collaboration-landscape.md`。人 & 犬 & 犬的协作是怎么发生的。

## 协作模式

### 1. 单犬独立

用户 → 单只犬 → 交付。犬在自己职责范围内自决完成。

### 2. 串行协作

用户 → A → B → C → 交付。每只犬的输出作为下一只的上下文。

```
@xigou 搜索代码 → xigou 输出 → @jinmao 检索相关知识 → jinmao 输出 → @bianmu 合成
```

### 3. 并行协作

用户 → A + B + C 同时 → 汇聚 → 交付。goroutine 并发 + shared streamer + WaitGroup。

### 4. 跨 thread 协作

Thread 1 的犬 → cross_post → Thread 2 的犬 → 结果回来。

## @mention 路由

- 行首 `@代号` 触发路由（行中无效）
- 中文 + 英文代号均支持
- 路由后球权转移给目标犬

## 球权流转

```
用户发消息 → 犬 A 持球 → A @B → B 持球 → B 完成 → B @A 或 @C 或 @leader
```

- 球权只有第一人称
- 唯一凭据是 @ 或 hold_ball 动作本身
- 状态描述不是球权声明

## 决策权限

| 层 | 谁拍板 | 触发条件 |
|----|--------|----------|
| 宏观 | operator | 不可逆 / 愿景级 |
| 中间 | 犬犬讨论 | 架构 / 选型 |
| 细节 | 犬犬自治 | 可逆 + 不碰硬排除 |

详见 `docs/decision-matrix.md`。
