# Architecture

Ports and adapters, with a strictly one-way dependency rule:

```
internal/app  ->  session / profile / secret / history  ->  internal/driver  ->  internal/domain
```

- **`internal/domain`** — `ConnectionProfile`, `Validate()`, sentinel errors. Pure; imports nothing
  of ours. Business rules live here, not in the API layer.
- **`internal/driver`** — the `Driver`/`Session` interfaces plus a `Registry` keyed by engine.
  `driver/mysql`, `driver/postgres` and `driver/mongo` are the adapters; `driver/sqlkit` is what the
  two SQL ones share, and `sqlkit.Lex` alone is what all three do.
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
- **`internal/transfer`** — the file formats a table travels as, and nothing about either end of the
  journey. See [carrying a table to a file](#carrying-a-table-to-a-file).
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

**MongoDB is the proof of that.** It is not a `database/sql` adapter, has no SQL, no declared columns
and no foreign keys, and adding it changed nothing in `app`, `session`, `profile`, `secret`, `history`
or `jsonstore` beyond one registry line and one new failure code. What it *did* need is set out in
[a section of its own](#the-document-store) below.

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

It is not only SQL's. `sqlkit.Shell` is a `Dialect` carried by the MongoDB adapter **for the lexer
alone**: where a command ends, what a string is and what a comment is are the same questions in a
document store's shell, so they are answered by the same code rather than by a second scanner. Nothing
else in `sqlkit` is meant for it — see [the document store](#the-document-store).

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
and what runs cannot drift; a SQL export calls `RowStatements` for the same reason. Each returns
`[]driver.DatabaseStatement` or `[]string` rather than one string, because a create is several
statements and some of them — an extension — can only be run inside the database just made.

**A dump is those two methods and nothing else.** `CreateStatements` is its structure half and
`RowStatements` its data half, and both are the driver's because only the engine knows how it
declares a table and quotes a value. The one difference between them is that a declaration has to be
*read*, so an engine that can answer for itself is **asked** rather than described: MySQL's whole
dump is one `SHOW CREATE TABLE`, storage engine and charset and constraints included, which no read
of `information_schema` puts back together. PostgreSQL has no such statement — `pg_dump` is another
program — so it describes the shape the tree already reads, through the same `Dialect`; the one thing
a description has to translate rather than copy is a `serial`, which is an `integer`, a sequence and
a default reading from it, and would otherwise name a sequence the dump never created. MongoDB writes
`createCollection` and a `createIndex` per index, and nothing about the fields, because a collection
declares none. `internal/app/dump.go` owns only the file: it opens it, asks for those statements, and
**streams** the rows into it batch by batch, so a dump of a million rows costs one batch of memory.

**`RowStatements` is one method and not five.** A SQL export, the data half of a dump and every mode
of an import are the same question asked with different `driver.RowWrite`s: an export is `WriteAppend`
with a batch of one, and an import is whichever mode the wizard picked with a batch of two hundred.
Only three things about it vary by engine, and all three are on `sqlkit.Dialect` — the verb
(`INSERT IGNORE` on MySQL, unchanged on PostgreSQL), the clause after the values (`ON DUPLICATE KEY
UPDATE` against `ON CONFLICT (…) DO UPDATE SET`), and whether the conflicting columns are named at
all. MongoDB writes its own: `insertOne`/`insertMany`, and `updateOne` with `upsert` for a merge,
because a document store's merge is a *command* and not a clause.

Two rules about it are not the driver's and must not migrate there. **A row with nothing to find it by
matches nothing** — `WHERE 1 = 0`, and a filter no document satisfies — because the alternative is a
`DELETE` with no `WHERE`. And **whether the run has to be answered for first is `domain`'s**:
`ImportHazard` is the third grammar for one rule, beside `StatementHazard` and `MongoHazard`.

## Carrying a table to a file

`internal/transfer` is the file formats and nothing else. It knows no databases: a row here is
`[]*string` — the shape a driver hands rows over in — and `nil` is a NULL in every format that can
tell one from an empty string. What a row *means* to a server is the driver's, and how it is quoted
into a statement is `sqlkit.Dialect`'s.

**Both halves stream.** A `Reader` hands rows over as it reads them and a `Writer` takes them as they
arrive, so an import of a million rows and an export of one each cost a batch of memory rather than a
table's worth — the same bargain `dump.go` already struck.

| | |
|---|---|
| `text.go` | Delimited and fixed-width, one scanner for both. The record delimiter is **detected** — LF, CRLF and CR all end a record — because a file written on one machine and read on another is the normal case and a wrong answer there reads the file as a single row. |
| `json.go`, `xml.go` | An array of objects, and a root of row elements. Neither declares its fields, so they are **inferred from a sample** of the first hundred — the same rule `mongo.fields` follows, and for the same reason. |
| `xlsx.go`, `xlsxwrite.go` | A workbook, written and read by hand. **No dependency is taken**, for the reason `internal/cloudauth` takes none: SpreadsheetML is a documented format and a grid of cells uses a small, well-trodden part of it. What is written is deliberately narrow — inline strings, one sheet, no styles, no shared-string table, which is what lets the writer stream. What is **read** is wider, because a workbook to import is one Excel wrote: shared strings, formula results, booleans, and a number that a style says is a date. |
| `document.go` | HTML and Markdown, written and never read — see [features.md](features.md#the-import-and-export-wizards). |
| `encoding.go` | The encodings the whole app reads files as, one list. UTF-8 is `UTF8BOM`: its decoder strips a mark a Windows tool left, and its encoder writes one, which is what makes Excel open a UTF-8 CSV as UTF-8. |

`internal/app/transfer.go` mirrors the format and encoding lists onto the wire so the wizard draws a
list the Go side decides, `import.go` maps a file's fields onto a table's columns and runs what
`RowStatements` builds, and `export_job.go` streams each table into a file of its own.

A table is addressed by a `driver.Namespace` — `(database, schema)` — and how many of its parts an
engine fills is the one structural difference between the SQL adapters. MySQL fills `Database`
alone; PostgreSQL fills both and needs a connection pool per database to do it, because a session
cannot leave the database it was opened with. MongoDB fills `Database` alone and needs one pool for
the *server*, a single connection reading every database on it. See [decisions.md](decisions.md).

## The document store

`internal/driver/mongo` is the first adapter that is not a `database/sql` one. Three things about it
are genuinely different, and everything else is the same adapter every other engine is.

**A collection declares no fields.** What a catalogue read answers for a table, `fields` infers from
the documents themselves: the first `sampleDocs` of the collection, unioned, `_id` always among them
because every document has one. It is a *sample* and says so — a field appearing in none of them is
not drawn until a read that returns it is. A **bounded** read then merges the documents it actually
returned into that list, so nothing a page holds goes unshown; a **streamed** read cannot, its header
having crossed the bridge before its first batch. See `documents.go` and [grid.md](grid.md).

**A filter is a document, not a clause.** `filter.go` builds a `bson.D`, so there is no quoting in the
package at all: a name and a value reach the server as *values*, and nothing a reader typed can be
read as anything else. It refuses what `sqlkit.Dialect.Where` refuses and for the same reasons — a
field the collection has never held, an operator this side does not know, half a `BETWEEN`.

`coerce` is what stands where a SQL engine's own cast stands, and it reads a value three ways in one
order that matters. The **field's own type** first, so `36` against an `int` is a number and against a
`string` is the text of one. Then **the shell's grammar**, so a cell the grid drew as `ObjectId("…")`
can be edited in the notation it was drawn in — without which no document could be addressed by its
own `_id`. And for a field the sample never saw — the first document of an empty collection — the
grammar alone, because there is nothing else to go on and `36` there is the number a reader meant.

**The verb is a method, not a first word.** `mql.go` reads a whole command — `db.users.find({…})`,
its Extended-JSON-ish arguments and the cursor methods bounding it — over the same tokens
`sqlkit.Lex` produces. It is a *reader of one command* and not an interpreter of a language: there
are no variables, no arithmetic and no control flow, because a client that half-implemented
JavaScript would be a client whose answers could not be trusted. Anything beyond the list `query.go`
understands is written `db.runCommand({…})` and reaches the server verbatim. What a command then
*does* is still a `domain` rule — `domain.MongoHazard`, beside `StatementHazard`, reading a different
grammar for the same answer.

| Part | MongoDB |
|---|---|
| a DSN | none: `options.go` builds the driver's own options, so nothing is written as text only to be parsed back |
| a dialler and a TLS configuration | values on those options, so there is no global registration and nothing to release with the session. A tunnelled connection is pinned `Direct`, or topology discovery would hand back addresses that resolve only on the far side of the bastion |
| `errors.go` | server code → sentinel, the shape MySQL's is in. Plus a text read for a handshake failure, which carries no code — see [errors.md](errors.md) |
| catalogue reads | the documents themselves, plus `listCollections` and `$collStats`. `TableList` is one read per collection, there being no catalogue table to join against; a `system.` name is the server's own bookkeeping and is not listed |
| placeholders | none; everything is a value in a document |
| the lexer's dialect | `sqlkit.Shell`: `//` comments, both quotes open a string, no identifier quote to take one of them |
| levels above a collection | one pool for the server; a single connection reads every database on it |
| `ddl.go` | a field is not a column, so a change to one is a change to the documents that carry it: `$set` to add, `$rename`, `$unset`, and a pipeline `$convert` to alter. `CreateTable` creates the collection and *only* the collection — see [decisions.md](decisions.md) for why a `$jsonSchema` validator built from the declared fields is a trap |
| `database.go` | there is no `CREATE DATABASE`. A database exists exactly as long as it holds a collection, so making one is making its first — which is why `DatabaseSpec.FirstCollection` exists and why the dialog asks |
| what it has not got | a foreign key, a primary key other than `_id`, a stored routine, a truncate distinct from an empty. Each says so through `domain.Unsupported` rather than being approximated or left to the server to refuse |

## Two Wails v2 constraints that shaped the design

1. **Bound methods must not take `context.Context`.** Wails exposes it as a real JavaScript
   argument rather than injecting it, which poisons the frontend API. `ConnectionAPI` instead holds
   the startup context (`useContext`, called from `App.Startup`) and derives per-call timeouts. Every
   API that raises one of the host's own dialogs needs that context too, which is why `SystemAPI` —
   which otherwise holds no state at all — embeds `apiContext` for `ChooseFile`.
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
