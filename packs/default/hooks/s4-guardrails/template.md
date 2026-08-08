## 红旗模式（对标 clowder-ai S10 护栏）

以下代码模式**自动违反 VISION**，不需要哲学判断，命中即停：

| 红旗 | 违反 | 正确做法 |
|------|------|----------|
| 在 `internal/` 层调 LLM 做推理 | §3 三层原则 | 推理交给 CLI adapter |
| 硬编码 workflow DAG（固定 pipeline） | §4.2 不可逆决策 | 用动态路由 |
| 新建 A2A HTTP server/client | §4.1 不可逆决策 | 用 CLI adapter（stdin/stdout pipe） |
| 在 platform 层做 agent reasoning | §4.1 不可逆决策 | reasoning 是 CLI 的事 |
| 引入 `internal/a2a/server/` 或 `internal/a2a/client/` | §4.1 不可逆决策 | 已废弃，用 `internal/adapter/` |
| 新增 `internal/` 顶层目录而不更新 VISION.md §6 | §6 平台能力清单 | 先更新 VISION.md |
| 跳 Phase（如 Phase 3 代码出现在 Phase 1 期间） | §7 路线图 | 先完成前置 Phase |
