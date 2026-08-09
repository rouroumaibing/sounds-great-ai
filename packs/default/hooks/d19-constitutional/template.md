## 审查注入（对标 clowder-ai D19）

审查规则 + 不可变约束：

- 同一个体不能 review 自己的代码
- 跨狗狗 review 优先
- 每个发现必须有明确严重级：P1（阻塞）/ P2（应修）/ P3（可选）
- hotfix PR 必须跨族或同族不同个体 review，不允许 self-merge
