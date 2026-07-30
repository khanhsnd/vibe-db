# Architecture

Ports and adapters, with a strictly one-way dependency rule:

```
internal/app  ->  session / profile / secret  ->  internal/driver  ->  internal/domain
```

- **`internal/domain`** — `ConnectionProfile`, `Validate()`, sentinel errors. Pure; imports nothing
  of ours. Business rules live here, not in the API layer.
- **`internal/driver`** — the `Driver`/`Session` interfaces plus a `Registry` keyed by engine.
  `driver/mysql` and `driver/postgres` are the adapters, and `driver/sqlkit` is what they share.
- **`internal/app`** — the Wails boundary and the only package bound to JavaScript. Speaks `dto.go`
  wire types, never domain types, and maps every error to a stable code (see
  [errors.md](errors.md)).
- **`frontend/src/bridge`** — the only frontend directory permitted to import `frontend/wailsjs/**`.
  Wails regenerates those bindings on every build, so wrapping them once keeps a signature change
  to a one-file fix.

## Adding a database engine

Add `internal/driver/<engine>` implementing `driver.Driver` and `driver.Session`, translate its
native errors to `domain` sentinels, add the engine to `domain.Engine` and to `ENGINES` in
`frontend/src/bridge/connections.ts`, and register it in `internal/app/app.go`. Nothing between
those two ends should need to change — if it does, an abstraction leaked.

Reach for `internal/driver/sqlkit` before writing anything: row scanning, result-column
reflection, statement classification, source parsing, identifier quoting and the transport
errors are all there already, and an engine differs from another only in

| Part | MySQL | PostgreSQL |
|---|---|---|
| `dsn.go` | go-sql-driver config, `tls=…` | libpq keyword/value, `sslmode=…` |
| `errors.go` | errno → sentinel | SQLSTATE → sentinel |
| catalogue reads | `information_schema` | `pg_catalog`, for `format_type` |
| placeholders | `?` | `$1` |
| `sqlkit.Dialect` | backtick quoting, `#` comments | double-quote quoting |

`driver.Session` calls the thing a table is qualified with a *database*, because MySQL does.
PostgreSQL cannot cross databases inside a session, so its adapter reads that field as a
**schema** — see [decisions.md](decisions.md).

## Two Wails v2 constraints that shaped the design

1. **Bound methods must not take `context.Context`.** Wails exposes it as a real JavaScript
   argument rather than injecting it, which poisons the frontend API. `ConnectionAPI` instead holds
   the startup context (`useContext`, called from `App.Startup`) and derives per-call timeouts.
2. **`frontend/wailsjs/` is generated on every build and is committed**, so a fresh clone
   type-checks without first running a Go build. It is excluded from `tsconfig.app.json`'s
   `include` — TypeScript resolves the `.d.ts` through the `@wailsjs/*` path alias, because
   including the generated `.js` makes `tsc` fail on it.

Wails v2 is intentional over v3, which is still alpha. See [decisions.md](decisions.md) for the
rest of the choices that should not be "fixed".
