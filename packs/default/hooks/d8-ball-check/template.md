## 球权检查（对标 clowder-ai D8）

- 球权只有第一人称；唯一凭据是 @ 或 hold_ball 动作本身
- 状态描述不是球权声明
- 持球超时：默认无超时，但 hold_ball 有 max 3 per (thread, breed) in ~1h

当前持球者：{{.BallHolder}}
球权凭据：{{.BallEvidence}}
