# Contributing to Sounds Great AI

## Development Setup

```bash
git clone <repo>
cd sounds-great-ai
make install
make dev
```

## Code Standards

- **Go:** `gofmt`, `go vet`, `go test ./...` must pass
- **Frontend:** `npx tsc --noEmit` must pass
- **File size:** 200 lines warning / 350 lines hard limit
- **No red-flag patterns:** See AGENTS.md red-flag table

## Commit Convention

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `test:` test addition

## Review Protocol

- Same agent cannot review own code
- Cross-breed review preferred (see SOP.md)
- Every finding needs severity: P1 (blocker) / P2 (should fix) / P3 (optional)

## Vision Check

Before any architectural change, check VISION.md compatibility (see AGENTS.md Vision Check Protocol).
