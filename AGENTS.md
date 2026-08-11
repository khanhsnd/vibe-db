# Repository Guidelines

## Project Structure & Module Organization

`main.go` starts the Wails desktop application. Backend code is in `internal/`: `app` is the Wails API boundary, service packages such as `session`, `profile`, and `history` implement application concerns, and `driver/` contains the database port plus engine adapters. Put shared SQL-adapter behaviour in `internal/driver/sqlkit`, not in a specific engine.

The React 19 frontend is in `frontend/src/`; use `bridge/` as the sole boundary to generated Wails bindings in `frontend/wailsjs/`. Documentation is intentionally detailed: begin with `docs/architecture.md` for boundaries and `docs/development.md` for the toolchain. Keep feature behaviour documented under `docs/features/`.

## Build, Test, and Development Commands

- `wails dev` — run the desktop app with hot reload.
- `wails build` — produce `build/bin/vibe-db.exe`.
- `go test ./...` — run all offline backend tests.
- `go vet ./...` and `gofmt -l internal main.go` — check Go quality and formatting.
- `pnpm typecheck` (from `frontend/`) — check TypeScript without emitting output.
- `pnpm build` (from `frontend/`) — type-check and create the Vite bundle.

Use pnpm 10; do not substitute npm. `wails dev`, unlike plain `pnpm dev`, supplies Wails bindings.

## Coding Style & Naming Conventions

Follow `gofmt` for Go and the existing TypeScript style: two-space indentation, single quotes, semicolons omitted, and PascalCase component files such as `ResultGrid.tsx`. Keep packages small and focused; prefer clear, direct code following SOLID, DRY, and KISS.

Respect the one-way dependency rule: `app` → services → `driver` → `domain`. Business rules belong in `domain`; bound APIs use DTOs and stable error codes. Frontend components must not import `wailsjs` directly, hard-code theme values, or create unstable Zustand selector results.

## Testing Guidelines

Place Go tests beside their code as `*_test.go`; use `TestXxx` names that describe observable behaviour. Add or update focused tests for backend changes and run `go test ./...` before opening a PR. There is no frontend test runner: type-check, build, and verify UI changes through `wails dev`. Test adapter changes against the relevant real database when practical; documented MongoDB and Redis live tests are opt-in via environment variables.

## Commits, Pull Requests, and Documentation

Write concise imperative commit subjects, e.g. `Implement restore with table selection`; use a conventional prefix when it clarifies scope, such as `docs:` or `feat:`. PRs should state the user-visible change, testing performed, and any required database or configuration steps; include screenshots for UI changes. Update the owning documentation in the same commit whenever behaviour changes
