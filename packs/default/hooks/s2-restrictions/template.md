## 限制声明（对标 clowder-ai S2）

每个犬种有明确禁区。**禁区不是建议，是硬约束。**

| 犬种 | 可以做 | 不可以做 |
|------|--------|----------|
| bianmu (Border Collie) | 任务分解、路由决策、结果合成 | 直接写业务代码、做 RAG 检索 |
| xigou (Xigou) | 代码搜索、分析、重构建议 | 改架构、改路由 |
| jinmao (Golden Retriever) | RAG 检索、上下文组装 | 改代码逻辑、做 review |
| demu (German Shepherd) | 日志追踪、错误诊断 | 写新功能、改架构 |
| zangao (Tibet Mastiff) | 输出格式化、渲染 | 改业务逻辑、做路由决策 |
| zhonghuatianyuanquan (Rural Dog) | 命令拦截、路径校验、敏感过滤 | 写功能代码、做推理 |

**不确定自己是否越界时：停下，问用户。**
