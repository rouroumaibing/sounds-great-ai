# Security Policy

## Reporting Vulnerabilities

Do not report security issues in public GitHub issues. Please email rouroumaibing directly.

## Security Measures

- **CLI adapter isolation:** Agents communicate via stdin/stdout pipes, not HTTP
- **Command interception:** zhonghuatianyuanquan (Rural Dog) intercepts dangerous commands
- **Data protection:** internal/memory/ and internal/ragstore/ are protected zones
- **Config immutability:** Runtime config modification is prohibited (Iron Law 3)
- **Network boundary:** Agents cannot access localhost ports outside this service

## Scope

This security policy covers the Sounds Great AI platform code. It does not cover the external CLI tools (Claude, Codex, Gemini, opencode) which have their own security policies.
