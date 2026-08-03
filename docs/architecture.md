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
reflection, statement classification, **script splitting**, source parsing, identifier and string
quoting, the transport errors and **the DDL statements both engines spell the same way** are all
there already, and an engine differs from another only in

| Part | MySQL | PostgreSQL |
|---|---|---|
| `dsn.go` | go-sql-driver config, `tls=…` | libpq keyword/value, `sslmode=…` |
| `errors.go` | errno → sentinel | SQLSTATE → sentinel |
| catalogue reads | `information_schema` | `pg_catalog`, for `format_type` |
| placeholders | `?` | `$1` |
| `sqlkit.Dialect` | backtick quoting, `#` comments, `\` escapes a string | double-quote quoting, `$tag$` bodies |
| levels above a table | one pool, `USE <database>` | a pool per database, `SET search_path` |
| `ddl.go` | `MODIFY COLUMN` restates a column whole; a qualified rename target; `DROP INDEX` on the table; every primary key is called `PRIMARY`; `USING` comes before the table | one `ALTER COLUMN` action per fact; an unqualified rename target; an index is an object of the schema; a key's constraint name has to be read before it can be dropped; `USING` comes after the table |
| `database.go` | a database is a namespace with a character set | a database is an object with an owner, a template, a locale, a tablespace and a connection limit, and can carry extensions and a comment |

`driver.Schema` is the DDL half of a session, and `sqlkit.DDL` supplies the part of it that does not
vary: an adapter embeds it, hands it one `Run(ctx, database, statement)`, and writes only the
statements above plus its catalogue reads. `Run` is also where the adapter's pool choice and its
error map live, so nothing in `sqlkit` has to know about either. DDL takes no arguments to bind — a
type, a default, an access method and a referential action reach the statement verbatim, and only
identifiers are quoted and only free text (`Dialect.QuoteString`) is written as a literal.

**A statement the UI shows is asked for rather than spelled twice.** The New Database dialog's SQL
Preview calls `CreateDatabaseSQL`, which is exactly the list `CreateDatabase` runs, so what is shown
and what runs cannot drift; a SQL export calls `InsertStatements` for the same reason. Each returns
`[]driver.DatabaseStatement` or `[]string` rather than one string, because a create is several
statements and some of them — an extension — can only be run inside the database just made.

A table is addressed by a `driver.Namespace` — `(database, schema)` — and how many of its parts an
engine fills is the one structural difference between the two adapters. MySQL fills `Database`
alone; PostgreSQL fills both and needs a connection pool per database to do it, because a session
cannot leave the database it was opened with. See [decisions.md](decisions.md).

## Two Wails v2 constraints that shaped the design

1. **Bound methods must not take `context.Context`.** Wails exposes it as a real JavaScript
   argument rather than injecting it, which poisons the frontend API. `ConnectionAPI` instead holds
   the startup context (`useContext`, called from `App.Startup`) and derives per-call timeouts.
2. **`frontend/wailsjs/` is generated on every build and is committed**, so a fresh clone
   type-checks without first running a Go build. It is excluded from `tsconfig.app.json`'s
   `include` — TypeScript resolves the `.d.ts` through the `@wailsjs/*` path alias, because
   including the generated `.js` makes `tsc` fail on it.
3. **A bound method's result reaches the WebView as one JavaScript string**, which puts a practical
   ceiling on how much one call may answer with. Anything larger is pushed as events instead: a read
   of every row of a table streams its rows as `app.EventRows` batches and answers with the columns
   alone (`app/events.go`, `driver.RowBatch`, `sqlkit.ScanRows`). Events are the only place the app
   layer speaks to the frontend without being asked, and `bridge/` is still the one side that knows
   the event's name.

Wails v2 is intentional over v3, which is still alpha. See [decisions.md](decisions.md) for the
rest of the choices that should not be "fixed".
