---
name: merge-gate
description: >
  合入 main：按行为/数据/安全/契约/不可逆风险选择 targeted 或 full gate，
  并消费一个或多个有客观触发理由的独立 review source。
triggers:
  - "merge"
  - "合入"
  - "merge gate"
  - "合入main"
---

# Merge Gate（合入门禁）

合入 main 前的风险路由。**强制力跟着风险走，不跟着动作类型走。**

## 风险路由

### 五轴风险判断

| 风险轴 | 命中信号 | gate 类型 |
|--------|----------|-----------|
| 行为面 | 用户可见行为、runtime 逻辑、bug 回归 | targeted |
| 数据 | 持久化数据、迁移、存储语义 | full gate |
| 安全 | auth、权限、secret、注入、命令边界 | full gate + 安全扫描 |
| 契约 | API schema / MCP 工具签名 / CLI 输出格式 | full gate + 契约测试 |
| 不可逆 | 删除、force push、close feat、圣域 | 先拿用户授权 |

五轴都未命中且改动可逆、无外部副作用 → 最小安全动作。

### 元风险强制升档

diff 触碰 `internal/sop/` 门禁逻辑或 VISION.md 不可逆决策时，直接进入 high-assurance，由非作者跨狗狗 reviewer 覆盖。

## Review Source 选择

**默认选择一个合适的独立验证源**，且必须是非作者：

| Source | 适用场景 | 不适用 |
|--------|----------|--------|
| local peer | 家里语境、治理/skill/SOP、实现语义 | 已选择 cloud 仍固定叠一层 |
| cloud review | 安全、数据、契约或陌生跨包代码 | 普通 test / PR 载体本身 |
| 愿景守护 | 用户可见或愿景变化的 feature close | 每个 PR、纯机械内部 change |

**去叠加**：只有不同风险面确实需要不同视角时才叠加，并分别写明触发理由。

## 合入前检查

- [ ] 五轴风险判断完成
- [ ] 选定的 review source 已消费（非作者）
- [ ] review 发现的 P1 全部解决
- [ ] `go build ./...` + `go test ./...` 通过
- [ ] 红旗模式检查通过
- [ ] 不碰圣域（`internal/memory/`、`internal/ragstore/`）或已获授权

## 合入后

- 通知相关狗狗（如改了 adapter，通知所有使用该 adapter 的狗狗）
- 更新 SOP stage（如有）
- 教训沉淀（如踩了新坑）
