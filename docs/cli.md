# CLI 安装指南

## 安装 CLI 工具

### Claude Code

```bash
npm install -g @anthropic-ai/claude-code@latest
```

配置文件：`~/.claude/8/CLAUDE.md`

登录：启动 `claude` → `/login` → 选择 "Claude account with subscription" → `/exit`

### OpenCode

```bash
npm install -g opencode-ai@latest
```

配置文件：`~/.config/opencode/AGENTS.md`

### Codex

```bash
npm install -g @openai/codex@latest
```

配置文件：`~/.codex/AGENTS.md`

登录：启动 `codex` → `/login` → "Sign in with ChatGPT" → `/exit`

### Gemini CLI

```bash
npm install -g @google/gemini-cli@latest
```

配置文件：`~/.gemini/GEMINI.md`

登录：启动 `gemini` → `/auth signin` → "Sign in with Google"

### Kimi Code

```bash
npm install -g @moonshot-ai/kimi-code@latest
```

登录：启动 `kimi` → `/login` → "Kimi Code (OAuth)" → `/exit`

### Qianwen CLI

```bash
npm install -g @qianwenai/qianwen-cli@latest
```

登录：启动 `qianwen` → `auth login` → `exit`

## 配置文件汇总

| CLI | 配置文件路径 | 说明 |
|-----|-------------|------|
| Claude | `~/.claude/CLAUDE.md` | Claude 身份和指令 |
| OpenCode | `~/.config/opencode/AGENTS.md` | OpenCode agent 指令 |
| Codex | `~/.codex/AGENTS.md` | Codex agent 指令 |
| Gemini | `~/.gemini/GEMINI.md` | Gemini 身份和指令 |

## 初始化配置文件

```bash
touch ~/.claude/CLAUDE.md
touch ~/.config/opencode/AGENTS.md
touch ~/.codex/AGENTS.md
touch ~/.gemini/GEMINI.md
```
