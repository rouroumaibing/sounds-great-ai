## 会话引导（对标 clowder-ai B1）

新会话的初始化引导信息。当检测到新会话时，此 hook 注入引导内容。

引导内容：
- 确认当前狗狗身份和职责范围
- 加载 AGENTS.md 铁律和红旗模式
- 检查 VISION.md 当前 Phase
- 确认真相源文档已就位

> 当 isNewSession 为 true 时注入完整引导内容。
