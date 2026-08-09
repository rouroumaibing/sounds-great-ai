## 路由策略（对标 clowder-ai D13）

@mention 解析规则：
- 行首 `@代号` 触发路由（行中无效）
- 中文代号：`@边牧` `@灵缇` `@金毛` `@德牧` `@藏獒` `@中华田园犬`
- 英文代号：`@bianmu` `@xigou` `@jinmao` `@demu` `@zangao` `@zhonghuatianyuanquan`

串行 vs 并行选择：
- 串行：A→B→C 链式，每个输出作为下一个的上下文
- 并行：goroutine 并发 + shared streamer + WaitGroup

当前路由策略：{{.RoutingStrategy}}
