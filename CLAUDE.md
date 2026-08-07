# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## What this is

`vibe-db` is a desktop database client in the Navicat Premium mould — a Go backend in a Wails v2
shell (OS WebView2, no bundled Chromium) with a React 19 + TypeScript frontend. MySQL, PostgreSQL,
MongoDB and Redis are implemented; the driver layer is a port so others are added as adapters,
`internal/driver/sqlkit` holds everything about a `database/sql` adapter that does not vary by
engine, `internal/driver/mongo` is the proof the port is not secretly a SQL one, and
`internal/driver/redis` is the proof it does not secretly assume tables either.

## Rules that must not be broken

1. **The dependency rule is one-way:**
   `internal/app` → `session / profile / secret / history / transfer / compare` → `internal/driver`
   → `internal/domain`.
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
12. **A difference between two tables is `internal/compare`'s, and nothing there is matched by
    position.** A field is matched by its name and a row by its key, because a column inserted in the
    middle would otherwise report every column below it as changed. It answers with *pairs* — one
    entry per thing either side holds — which is what lets two panes hold the same rows in the same
    order, and is the only reason a diff can be read across. A comparison is **bounded on purpose**:
    both sides are held at once, so there is no `All` the way a table tab has one, and the answer
    says which side it did not see the whole of.
13. **A statement a migration shows is the statement it runs.** One list of `driver.TableChange`
    goes to `PreviewTableChanges` to be spelled and to `ApplyTableChanges` to be run, and every kind
    on it has a *run* beside its spelling on `driver.Schema`, both going through one speller on
    `sqlkit.Dialect`. Nothing outside a driver has ever spelled a statement and nothing may start:
    what differs between the engines is a **flag** on `Dialect`, never a second spelling. An Apply
    **stops at the first failure** and there is no transaction around it — the preview is what
    protects the reader, which is why it has to be exact.
14. **What a search matches is a `domain` rule; how many rows hold it is the driver's.**
    `domain.TextMatch` is the one rule — a pattern, an expression and a compiled matcher spelled from
    the same text — so `Contains` means the same thing of a row as of a column name. The data half is
    `driver.Session.MatchingRows`, and what differs between the SQL engines is a **flag** on
    `sqlkit.Dialect` (`RegexIsFunction`, `PatternIgnoresCase`, `WordBoundary`), never a second
    spelling. The structure half asks **no server anything**: those names are what a catalogue read
    already answered with.

15. **A key-value store has one table, and it is not declared.** `internal/driver/redis` answers
    with exactly one table per database — `(All Data)`, and five fixed columns that are the same on
    every Redis server there is — because a Redis database is a flat key space; any other table name
    is `ErrUnknownTable`. A page is cut from a **sorted** scan and only that page's values are read,
    a glob is the only narrowing the server will do and is taken from an `AND` alone, and a container
    value is spelled as **JSON** so that what the grid draws is exactly what an edit hands back — a
    value it could only sample is marked and refuses to be written. What a command *does* is
    `domain.RedisHazard`, the fourth grammar for one rule, and everything a key space has not got is
    `domain.Unsupported` naming which. **How that key space is read is the frontend's, and asks the
    server nothing extra**: `features/table` groups the page's keys on the colon into an
    [outlined column](docs/grid.md), and replaces Filter & Sort with two boxes that write the same
    `Key contains …` / `Type = …` conditions the builder would have. Never a second read, and never a
    second notation — **the value is read and written as the driver's own JSON, in `ValueViewer`, not
    as a table of fields**. Those rows are picked and never typed: the panel below is where a key is
    edited, and a click on a branch edits nothing, a prefix being a folder.

## Where to read before changing something

| Task | Read |
|---|---|
| Build, test, run against a real database | [docs/development.md](docs/development.md) |
| Touch a layer boundary, or add an engine | [docs/architecture.md](docs/architecture.md) |
| Touch anything MongoDB | [docs/architecture.md](docs/architecture.md#the-document-store) |
| Touch anything Redis | [docs/architecture.md](docs/architecture.md#the-key-value-store) |
| Add or change a failure mode | [docs/errors.md](docs/errors.md) |
| Change UI behaviour, or find out what exists | [docs/features.md](docs/features.md) |
| Touch a file format, an import or an export | [docs/architecture.md](docs/architecture.md#carrying-a-table-to-a-file) |
| Touch a comparison of two tables | [docs/architecture.md](docs/architecture.md#comparing-two-tables) |
| Touch a search across a schema | [docs/architecture.md](docs/architecture.md#looking-for-text-across-a-schema) |
| Touch any table, grid, cell or selection | [docs/grid.md](docs/grid.md) |
| Touch the SQL editor or completion | [docs/editor.md](docs/editor.md) |
| Write a component, a store, or a style | [docs/frontend.md](docs/frontend.md) |
| Wonder why something looks wrong | [docs/decisions.md](docs/decisions.md) |
| Claim anything about memory | [docs/memory.md](docs/memory.md) |

A new failure mode needs all four of: a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table in [docs/errors.md](docs/errors.md).

A new engine capability the UI has to know about is a field on `EngineInfo` in
`frontend/src/bridge/connections.ts` — `hasForeignKeys`, `hasRoutines`, `documents`, `keys`,
`createsTables`, `createsDatabases`, `hasViews`, `needsUser` — and never a comparison against an
engine's name in a component.

Commands, in short: `wails dev`, `wails build`, `go test ./...`, `go vet ./...`,
`gofmt -l internal main.go`, and `pnpm typecheck` in `frontend/`.
