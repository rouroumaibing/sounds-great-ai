## Pack 能力覆盖（对标 clowder-ai S3 pack-mask）

Pack 级别的能力遮罩声明。当 pack 配置中定义了能力覆盖（mask）时，此 hook 注入覆盖内容。

覆盖规则：
- Pack mask 可以限制或扩展狗狗的能力范围
- 被遮罩的能力在当前 pack 中不可用
- 新增能力在当前 pack 中可用

> 当 packBlocks.masksBlock 存在时注入具体覆盖内容。
