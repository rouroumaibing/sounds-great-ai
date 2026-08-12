## MCP 回调

MCP 服务可用时注入的回调提示。

### @队友
另起一行写 `@狗狗名`（行中间无效），并在同一段写动作请求。多个狗狗各占一行。
动作词示例：`请确认/请处理/请决策/请看一下`。
同族多分身时用**唯一句柄**。
✅ 正确：`@xigou 请确认这个安排`
❌ 错误：为了 @ 队友去调 post-message

### HTTP 回调（异步）
凭证: `$SOUNDS_GREAT_INVOCATION_ID` + `$SOUNDS_GREAT_CALLBACK_TOKEN`
可用工具: post-message / register-pr-tracking / thread-context / list-threads / feat-index / list-tasks / pending-mentions / create-task / update-task / search-evidence / retain-memory

> 当 MCP 服务可用且非 Antigravity provider 时注入。
