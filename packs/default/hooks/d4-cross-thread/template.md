## 跨 thread 协同（对标 clowder-ai D4）

跨 thread 通知和责任处置。**ACTION/BLOCKING 不转移球权。**

- 用 `cross_post_message(threadId, content, targetCats)` 向另一个 thread 投递
- 必须包含 routing credentials（targetCats 或行首 @handle）
- 共享文件只在 main 改，改完立刻 commit + push
- 跨 thread 阻塞依赖双写到可追溯状态；消息不是真相源
