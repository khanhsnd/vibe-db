# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## What this is

`vibe-db` is a desktop database client in the Navicat Premium mould — a Go backend in a Wails v2
shell (OS WebView2, no bundled Chromium) with a React 19 + TypeScript frontend. MySQL is the only
engine implemented; the driver layer is a port so others are added as adapters.

## Rules that must not be broken

1. **The dependency rule is one-way:**
   `internal/app` → `session / profile / secret` → `internal/driver` → `internal/domain`.
   Business rules live in `domain`, not in the API layer. `internal/app` speaks `dto.go` wire types,
   never domain types. `frontend/src/bridge` is the only frontend directory allowed to import
   `frontend/wailsjs/**`.
2. **Never return a raw error from a bound method** — wrap it with `wrapError`. **Never match on
   message text in the UI**; switch on `code`.
3. **Bound methods must not take `context.Context`** — Wails v2 would expose it as a real
   JavaScript argument. `ConnectionAPI` holds the startup context instead.
4. **Do not hardcode colour or type values in components.** They live only in
   `shared/theme/tokens.css`.
5. **Zustand selectors must return referentially stable values**, or the app renders a blank screen
   in an infinite loop.
6. **pnpm is the package manager** (`wails.json` invokes it); do not switch to npm. There is **no
   frontend test runner installed** — do not write tests assuming vitest or Testing Library exists.

## Where to read before changing something

| Task | Read |
|---|---|
| Build, test, run against a real database | [docs/development.md](docs/development.md) |
| Touch a layer boundary, or add an engine | [docs/architecture.md](docs/architecture.md) |
| Add or change a failure mode | [docs/errors.md](docs/errors.md) |
| Change UI behaviour, or find out what exists | [docs/features.md](docs/features.md) |
| Touch the SQL editor or completion | [docs/editor.md](docs/editor.md) |
| Write a component, a store, or a style | [docs/frontend.md](docs/frontend.md) |
| Wonder why something looks wrong | [docs/decisions.md](docs/decisions.md) |
| Claim anything about memory | [docs/memory.md](docs/memory.md) |

A new failure mode needs all four of: a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table in [docs/errors.md](docs/errors.md).

Commands, in short: `wails dev`, `wails build`, `go test ./...`, `go vet ./...`,
`gofmt -l internal main.go`, and `pnpm typecheck` in `frontend/`.
