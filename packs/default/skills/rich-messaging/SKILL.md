---
name: rich-messaging
description: >
  富媒体消息发送：语音、图片、卡片、清单、代码 diff、交互选择。
  Use when: 发语音、发图、发卡片、展示结构化信息、长结构化汇报、庆祝。
  Not for: 纯文字聊天、技术讨论、日常回复。
  Output: rich block 附着在消息上。
triggers:
  - "发语音"
  - "发图"
  - "发卡片"
  - "rich block"
---

# Rich Messaging（富媒体消息）

用 `create_rich_block` 创建富媒体块，附着在当前消息上。

## 支持类型

| kind | 用途 |
|------|------|
| card | 状态/决策卡片 |
| diff | 代码变更 |
| checklist | inline todos |
| file | 已有文档/音频/视频 |
| media_gallery | 图片集 |
| audio | 语音 |
| interactive | 用户选择/确认 |
| html_widget | 自定义 HTML |

## 流程

1. 先调 `get_rich_block_rules` 加载 schema
2. 构造 block JSON（`kind` / `v: 1` / `id` 必填）
3. 调 `create_rich_block(block)` 创建

## 不做的事

- 纯文字聊天不需要 rich block
- 技术讨论用普通 markdown
