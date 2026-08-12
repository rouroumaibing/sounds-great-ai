## MCP 工具列表

以下 MCP 工具可通过 CLI agent 调用。工具由 Platform 层的 MCP Bridge (`internal/mcp/`) 管理。

### 文件操作

| 工具 | 描述 |
|------|------|
| Read | 读取文件内容 |
| Write | 写入文件 |
| Edit | 编辑文件（搜索/替换） |
| Glob | 文件模式匹配 |
| Grep | 正则搜索文件内容 |

### 代码搜索

| 工具 | 描述 |
|------|------|
| SearchCodebase | 语义代码搜索（基于嵌入模型） |

### 执行

| 工具 | 描述 |
|------|------|
| RunCommand | 执行终端命令（受安全审查约束） |

### 知识检索

| 工具 | 描述 |
|------|------|
| RAGSearch | 向量库知识检索 |
| RAGStore | 存储知识到向量库 |

> **注意**：MCP 工具配置由 `internal/mcp/` 注册表管理。不同 CLI adapter 的 MCP 支持程度不同（Claude/Codex 原生支持，Gemini 不支持）。
