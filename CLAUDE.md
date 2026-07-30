# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`vibe-db` is a desktop database client in the Navicat Premium mould — a Go backend in a Wails v2
shell (OS WebView2, no bundled Chromium) with a React 19 + TypeScript frontend. MySQL is the only
engine implemented; the driver layer is a port so others are added as adapters.

Read `README.md` for the error-code table, the measured memory baselines, and current feature status.

## Commands

The Wails CLI is not vendored. Install it once, and note `$GOPATH/bin` may not be on PATH:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
export PATH="$PATH:$(go env GOPATH)/bin"   # PowerShell: $env:PATH += ";$(go env GOPATH)\bin"
wails doctor
```

| Task | Command |
|---|---|
| Dev build, hot reload | `wails dev` (from repo root) |
| Production build | `wails build` → `build/bin/vibe-db.exe` |
| Go tests | `go test ./...` |
| Single Go test | `go test ./internal/driver/mysql -run TestBuildDSNTLSModes -v` |
| Go vet + format | `go vet ./...` and `gofmt -l internal main.go` |
| Frontend typecheck | `pnpm typecheck` (in `frontend/`) |
| Frontend build | `pnpm build` (in `frontend/`) — runs `tsc -b` then `vite build` |

pnpm is the package manager (`wails.json` invokes it); do not switch to npm.

There is **no frontend test runner installed**. Do not write frontend tests assuming vitest or
Testing Library exists — add the dependency first, or verify frontend behaviour by running the app.

### Verifying against a real database

`wails dev` serves the frontend *with* the Wails runtime and bindings on `http://localhost:34115`,
so `window.go.app.ConnectionAPI` is callable from an ordinary browser. This is the most effective
way to exercise the backend end-to-end without driving the native window. Use
`wails dev -browser=false` to suppress auto-opening a browser.

A plain `pnpm dev` (Vite alone) does **not** expose the bindings — the app renders but every
backend call fails.

## Architecture

Ports and adapters, with a strictly one-way dependency rule:

```
internal/app  ->  session / profile / secret  ->  internal/driver  ->  internal/domain
```

- **`internal/domain`** — `ConnectionProfile`, `Validate()`, sentinel errors. Pure; imports nothing
  of ours. Business rules live here, not in the API layer.
- **`internal/driver`** — the `Driver`/`Session` interfaces plus a `Registry` keyed by engine.
  `driver/mysql` is the sole adapter.
- **`internal/app`** — the Wails boundary and the only package bound to JavaScript. Speaks `dto.go`
  wire types, never domain types.
- **`frontend/src/bridge`** — the only frontend directory permitted to import `frontend/wailsjs/**`.

### Adding a database engine

Add `internal/driver/<engine>` implementing `driver.Driver` and `driver.Session`, translate its
native errors to `domain` sentinels, and register it in `internal/app/app.go`. Nothing above the
driver layer should need to change — if it does, an abstraction leaked.

### Errors crossing the bridge

A Go `error` reaching JavaScript degrades to a bare string. `internal/app/errmap.go` therefore
serialises every error as a JSON envelope (`code`, `message`, `field`, `detail`), and
`frontend/src/bridge/errors.ts` rehydrates it into a typed `AppError`.

**Never return a raw error from a bound method — wrap it with `wrapError`. Never match on message
text in the UI; switch on `code`.** New failure modes need a `domain` sentinel, a mapping in
`errmap.go`, an entry in the `ErrorCode` union, and a row in the README table. `errmap_test.go`
guards the JSON contract the frontend depends on.

### Two Wails v2 constraints that shaped the design

1. **Bound methods must not take `context.Context`.** Wails exposes it as a real JavaScript
   argument rather than injecting it, which poisons the frontend API. `ConnectionAPI` instead holds
   the startup context (`useContext`, called from `App.Startup`) and derives per-call timeouts.
2. **`frontend/wailsjs/` is generated on every build and is committed**, so a fresh clone
   type-checks without first running a Go build. It is excluded from `tsconfig.app.json`'s
   `include` — TypeScript resolves the `.d.ts` through the `@wailsjs/*` path alias, because
   including the generated `.js` makes `tsc` fail on it.

### Frontend conventions

- Path aliases: `@/*` → `frontend/src/*`, `@wailsjs/*` → `frontend/wailsjs/*`. Declared in both
  `vite.config.ts` and `tsconfig.app.json`; changing one requires changing the other.
- **Colour and type live only in `shared/theme/tokens.css`.** Raw values sit on `:root`, are
  overridden under `:root[data-theme='light']`, and are mapped into Tailwind utilities via
  `@theme inline` — the `var()` indirection is what makes runtime theme switching work without a
  rebuild. Do not hardcode hex values in components.
- Row height is shared between the object tree and the result grid via `shared/theme/density.ts`
  (the virtualizers need it as a number).
- `shared/ui/Split.tsx` wraps `react-resizable-panels`, which renamed its entire public surface
  between v3 and v4. Import splitters from that wrapper, never from the library directly. Note its
  `Panel` inner div carries `overflow: auto`, which surfaces small child overflows as scrollbars.
- TypeScript runs with `strict`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`.
  The last one means an optional prop must be typed `foo?: T | undefined` to accept an explicit
  `undefined`.

### Zustand selectors

Selectors must return referentially stable values. A selector that builds a new object or array
per call makes React's snapshot comparison always fail and produces an infinite render loop with a
blank screen. Derive such shapes in `useMemo` from primitive slices instead — `buildTree` in
`stores/connections.ts` is the reference example.

## Deliberate choices — do not "fix" these

- **`multiStatements` stays `false`** in the MySQL DSN. Statements are split client-side, which
  removes an injection surface and allows per-statement timing.
- **Connection pools are capped at 4 open / 2 idle** per session. A GUI needs no more, and it is
  half of the memory story.
- **Result sets are capped** (1 000 rows by default) rather than streamed unbounded into the UI.
  This cap is the mechanism keeping memory claims true; do not remove it for convenience.
- **Passwords never enter `connections.json`.** Profile metadata goes to
  `%APPDATA%/vibe-db/connections.json`, written atomically via temp-file + rename so a failed write
  leaves the previous file intact. The secret goes to the OS credential store via `internal/secret`.
  `repo_test.go` asserts both. The repo requests mode `0600`, which is only meaningful on POSIX —
  on Windows the file relies on inherited ACLs, so the file is not a safe place for secrets
  regardless.
- Wails v2 is intentional over v3, which is still alpha.

## Memory expectations

WebView2 *is* Chromium. Wails buys a ~11 MB binary and an engine shared with the OS, **not** a
small resident set: an idle window measures roughly 116–143 MB private / 399–418 MB total working
set across 7 processes. Tauri would measure the same. Do not repeat a claim that this shell is
lightweight on RAM. Re-measure after each milestone and record it in the README table, reading the
private column and treating sub-30 MB differences as noise.
