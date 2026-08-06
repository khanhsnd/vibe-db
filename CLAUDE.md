# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## What this is

`vibe-db` is a desktop database client in the Navicat Premium mould — a Go backend in a Wails v2
shell (OS WebView2, no bundled Chromium) with a React 19 + TypeScript frontend. MySQL, PostgreSQL
and MongoDB are implemented; the driver layer is a port so others are added as adapters,
`internal/driver/sqlkit` holds everything about a `database/sql` adapter that does not vary by
engine, and `internal/driver/mongo` is the proof the port is not secretly a SQL one.

## Rules that must not be broken

1. **The dependency rule is one-way:**
   `internal/app` → `session / profile / secret / history` → `internal/driver` → `internal/domain`.
   Business rules live in `domain`, not in the API layer. `internal/app` speaks `dto.go` wire types,
   never domain types. `frontend/src/bridge` is the only frontend directory allowed to import
   `frontend/wailsjs/**`. **What is dangerous to run is a `domain` rule**
   (`Environment.NeedsConfirmation`); the frontend receives its answer and never re-derives it.
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
7. **Anything that shows rows and columns is `ResultGrid`** — never a `<table>` or a second grid.
   Its look and behaviour are fixed by [docs/grid.md](docs/grid.md); add features to the grid, and
   update that file in the same change.
8. **Reading a script is the driver's, never the frontend's.** Where a statement ends, how it is
   laid out, and what it is going to do are all lexical and therefore the engine's — they reach the
   app through `driver.Text`, and **`sqlkit.Lex` is the only scanner there is**. The SQL engines put
   the whole of `driver.Text` on `sqlkit.Dialect`; MongoDB writes its own `Format` and `Review` (a
   document store has no clauses and no keywords) but still reads its tokens from `sqlkit.Lex`
   through `sqlkit.Shell`. Never write a second scanner, in Go or in TypeScript. **A data file is
   not a script** — `internal/transfer` reads CSV, JSON, XML and XLSX and is not covered by this:
   nothing it reads is ever a statement, and nothing in it reaches a server except as a value.
11. **A row leaves for a server through one method.** `driver.Schema.RowStatements` is what a SQL
    export, the data half of a dump and every mode of an import all call — there is no second way to
    turn rows into statements, and nothing outside a driver ever quotes a value. What is *dangerous*
    about a mode is a `domain` rule (`domain.ImportHazard`), and the wizard receives its answer.
9. **A document store is not a table store, and the app says so rather than pretending.** A field
   is inferred from the documents, never declared (`mongo.fields`, and see [docs/grid.md](docs/grid.md)
   for what a sample costs); a filter is a `bson.D` and never text, so nothing in that package quotes
   anything; and what an engine simply has not got — a foreign key, a primary key other than `_id`,
   a stored routine — is `domain.Unsupported` and its own error code, never a statement left for the
   server to refuse. What a *command* does is still a `domain` rule: `domain.MongoHazard`.
10. **No secret goes in `connections.json`.** A password, a bastion's pass phrase and a cloud token
    all live in the credential store — the profile's id for the first, `<id>:ssh` for the second, and
    the third is never stored at all. A DTO says *that* one exists (`hasPassword`,
    `hasSshSecret`), never what it is.

## Where to read before changing something

| Task | Read |
|---|---|
| Build, test, run against a real database | [docs/development.md](docs/development.md) |
| Touch a layer boundary, or add an engine | [docs/architecture.md](docs/architecture.md) |
| Touch anything MongoDB | [docs/architecture.md](docs/architecture.md#the-document-store) |
| Add or change a failure mode | [docs/errors.md](docs/errors.md) |
| Change UI behaviour, or find out what exists | [docs/features.md](docs/features.md) |
| Touch a file format, an import or an export | [docs/architecture.md](docs/architecture.md#carrying-a-table-to-a-file) |
| Touch any table, grid, cell or selection | [docs/grid.md](docs/grid.md) |
| Touch the SQL editor or completion | [docs/editor.md](docs/editor.md) |
| Write a component, a store, or a style | [docs/frontend.md](docs/frontend.md) |
| Wonder why something looks wrong | [docs/decisions.md](docs/decisions.md) |
| Claim anything about memory | [docs/memory.md](docs/memory.md) |

A new failure mode needs all four of: a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table in [docs/errors.md](docs/errors.md).

A new engine capability the UI has to know about is a field on `EngineInfo` in
`frontend/src/bridge/connections.ts` — `hasForeignKeys`, `hasRoutines`, `documents` — and never a
comparison against an engine's name in a component.

Commands, in short: `wails dev`, `wails build`, `go test ./...`, `go vet ./...`,
`gofmt -l internal main.go`, and `pnpm typecheck` in `frontend/`.
