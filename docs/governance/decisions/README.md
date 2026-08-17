# Architecture Decision Records

ADR-style decision records. Each file follows the format:

```
# ADR-XXX: Title

## Status
Accepted | Superseded by ADR-YYY | Deprecated

## Context
Why this decision was needed.

## Decision
What was decided.

## Consequences
What happened as a result.
```

## Index

- [不可逆决策（Irreversible Decisions）](./irreversible-decisions.md) — 原 VISION §4，锁定决策 / ADR（CLI adapter / 动态路由 / 球权账本 / carrier 四档 transport 等）。
- [ADR-002: CLI Carrier 四档 Transport + 持久进程池 + PTY + Redis 健康度](./ADR-002-cli-carrier.md) — §4.6 的展开 ADR：carrier 抽象、降级链、warm 池、PTY、Redis 健康度的决策背景、代价与回滚。

