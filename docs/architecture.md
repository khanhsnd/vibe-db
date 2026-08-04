# Architecture

Ports and adapters, with a strictly one-way dependency rule:

```
internal/app  ->  session / profile / secret / history  ->  internal/driver  ->  internal/domain
```

- **`internal/domain`** — `ConnectionProfile`, `Validate()`, sentinel errors. Pure; imports nothing
  of ours. Business rules live here, not in the API layer.
- **`internal/driver`** — the `Driver`/`Session` interfaces plus a `Registry` keyed by engine.
  `driver/mysql` and `driver/postgres` are the adapters, and `driver/sqlkit` is what they share.
- **`internal/app`** — the Wails boundary and the only package bound to JavaScript. Speaks `dto.go`
  wire types, never domain types, and maps every error to a stable code (see
  [errors.md](errors.md)).
- **`internal/history`** — the statements the reader has run, so one can be found and run again.
  A port with `List` / `Append` / `Clear` and a `jsonfile` adapter, exactly as `profile` is. `Append`
  is its only write besides `Clear`, because what a log holds after one more statement — newest
  first, one entry per statement, capped — is `domain.Remember`'s rule and not a caller's.
- **`internal/jsonstore`** — one JSON document on disk: read whole, written whole, and put in place
  by a rename, so a crash mid-write leaves the document that was there rather than half of a new
  one. `File[T].Update` does the read-change-write under one lock, and writes nothing if the change
  fails, which is how a rejected delete leaves the file alone. The connection profiles and the query
  history are each one of these, which is why neither adapter spells atomic writing out.
- **`internal/sshtunnel`** — a database connection carried over SSH. It knows nothing about
  databases: it dials a bastion and hands back a function that opens connections *from* it, which is
  exactly the shape `driver.Dialer` has, and is why neither engine adapter has ever heard of SSH.
  Nothing is listened on locally — there is no forwarded port to collide with anything, and the
  address the engine asks for is resolved on the far side. It is the one package in the app that
  reads an error's *message*, because `x/crypto/ssh` gives no code to switch on; see
  [errors.md](errors.md).
- **`internal/cloudauth`** — the password for a connection that authenticates by a cloud identity
  rather than by something the reader knows: an RDS IAM token, signed here with SigV4 from the
  machine's AWS environment, and a Google OAuth access token from its application default
  credentials. Nothing here is ever stored — a credential good for a quarter of an hour is not a
  secret worth keeping — and no SDK is pulled in for either: one is a documented signing algorithm
  and the other is a documented token exchange.
- **`internal/fonts`** — the font families the host machine has, read from the Windows font registry
  (both the machine-wide hive and the reader's own, since a per-user install is registered only
  there). Bound through `SystemAPI`, the one API that answers about the *machine* rather than about a
  server. It returns no error: a list that cannot be read is not a failure a reader can act on, and
  the picker falls back to the families the stylesheet ships with.
- **`frontend/src/bridge`** — the only frontend directory permitted to import `frontend/wailsjs/**`.
  Wails regenerates those bindings on every build, so wrapping them once keeps a signature change
  to a one-file fix.

## Adding a database engine

Add `internal/driver/<engine>` implementing `driver.Driver` and `driver.Session`, translate its
native errors to `domain` sentinels, add the engine to `domain.Engine` and to `ENGINES` in
`frontend/src/bridge/connections.ts`, and register it in `internal/app/app.go`. Nothing between
those two ends should need to change — if it does, an abstraction leaked.

Reach for `internal/driver/sqlkit` before writing anything: row scanning, result-column
reflection, statement classification, **the lexer** and everything written over it — script
splitting, **laying a script out**, **reviewing it** — source parsing, identifier and string
quoting, **building a `*tls.Config` from a profile's certificate files**, the transport errors and
**the DDL statements both engines spell the same way** are all
there already, and an engine differs from another only in

| Part | MySQL | PostgreSQL |
|---|---|---|
| `dsn.go` | go-sql-driver config, `tls=…` | libpq keyword/value, `sslmode=…` |
| a dialler and a TLS configuration | registered **globally by name** before the DSN can refer to them, and deregistered with the session (`transport.go`) | set on the parsed `pgx.ConnConfig`, which `stdlib` then takes by name — the certificate *files* need none of it, being libpq keywords already |
| `errors.go` | errno → sentinel | SQLSTATE → sentinel |
| catalogue reads | `information_schema` | `pg_catalog`, for `format_type` |
| placeholders | `?` | `$1` |
| `sqlkit.Dialect` | backtick quoting, `#` comments, `\` escapes a string | double-quote quoting, `$tag$` bodies |
| levels above a table | one pool, `USE <database>` | a pool per database, `SET search_path` |
| `ddl.go` | `MODIFY COLUMN` restates a column whole; a qualified rename target; `DROP INDEX` on the table; every primary key is called `PRIMARY`; `USING` comes before the table | one `ALTER COLUMN` action per fact; an unqualified rename target; an index is an object of the schema; a key's constraint name has to be read before it can be dropped; `USING` comes after the table |
| `database.go` | a database is a namespace with a character set | a database is an object with an owner, a template, a locale, a tablespace and a connection limit, and can carry extensions and a comment |

**`sqlkit.Lex` is the one place a script is read character by character.** Splitting, formatting and
reviewing are all written over its tokens, so what counts as a string, a comment or a `$tag$` body
is settled once per engine instead of three times — and a fourth thing built on it inherits that for
free.

**`driver.Text` is the half of an engine that needs no connection**: `Split`, `Format`, `Review`.
It hangs off the `Driver` rather than off a `Session`, because a reader lays out and is warned about
a script whether or not anything is dialled — `sqlkit.Dialect` is the whole implementation, and both
adapters return theirs from one line.

**Three things happen above the adapters, before either is handed anything**, because none of them
varies by engine: the profile is validated, the bastion is stood up, and a cloud password is minted.
All three are in `session.Manager.Open`. The tunnel arrives at the adapter as a `driver.Dialer` and
the token as an ordinary `driver.Credentials.Password`, so neither adapter knows either exists. The
tunnel then lives exactly as long as the session and is closed after its pools, since every pool is
carried over it.

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
   of every row of a table streams its rows as `app.EventRows` batches, its count as
   `app.EventRowsCounted`, and answers with the columns alone (`app/events.go`, `driver.RowStream`,
   `sqlkit.ScanRows`). Events are the only place the app layer speaks to the frontend without being
   asked, and `bridge/` is still the one side that knows an event's name.

Wails v2 is intentional over v3, which is still alpha. See [decisions.md](decisions.md) for the
rest of the choices that should not be "fixed".
