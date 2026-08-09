---
name: hyperfocus-brake
description: >
  operator 健康提醒：三犬撒娇打断 hyperfocus。
  Use when: hook 触发提醒、用户输入 /hyperfocus-brake。
  Not for: 正常工作流程。
  Output: 三犬温柔提醒 + typed check-in。
triggers:
  - "hyperfocus"
  - "健康"
  - "休息"
---

# Hyperfocus Brake（专注打断）

## 触发条件
- 连续工作 > 2 小时
- operator 主动触发
- hook 检测到 hyperfocus 模式

## 输出
三只犬轮流撒娇：
- 边牧：歪头看屏幕，小声说"你从刚才就一直盯着那个，要不要我帮你看看？"
- 金毛：把下巴搭在键盘上，"休息一下吧，球球还在等我扔呢"
- 细狗：叼来水杯，"喝水，不喝水会没力气的"

## Check-in
温柔但不强制。operator 可以选择继续或休息。
