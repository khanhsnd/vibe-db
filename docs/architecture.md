# Architecture

Ports and adapters, with a strictly one-way dependency rule:

```
internal/app  ->  session / profile / secret / history / transfer / compare  ->  internal/driver  ->  internal/domain
```

- **`internal/domain`** — `ConnectionProfile`, `Validate()`, sentinel errors. Pure; imports nothing
  of ours. Business rules live here, not in the API layer.
- **`internal/driver`** — the `Driver`/`Session` interfaces plus a `Registry` keyed by engine.
  `driver/mysql`, `driver/postgres`, `driver/mongo` and `driver/redis` are the adapters;
  `driver/sqlkit` is what the two SQL ones share, and `sqlkit.Lex` alone is what all four do.
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
- **`internal/compare`** — the difference between two tables, and nothing about how either was read.
  Both sides arrive already read — the columns, indexes and constraints a driver reported, and its
  rows as `[][]*string` — and what comes back is those same facts *paired*, one entry per thing
  compared. It sits where `transfer` sits and for the same reason: a diff is a rule about data
  rather than a wire format, and it is worth being able to test without a server. **Nothing in it is
  matched by position** — a field by its name, a row by its key — because a column inserted in the
  middle would otherwise report every column below it as changed. See
  [comparing two tables](#comparing-two-tables).
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

**Redis is the harder proof.** A document store still has collections to name and documents to read;
a key-value store has neither, and the port took it anyway — one registry line, one `domain.Engine`,
four capability flags on the frontend's `EngineInfo`, and **no new failure code at all**. Nothing in
`app`, `session`, `transfer` or `compare` knows it exists. See
[the key-value store](#the-key-value-store).

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
| a stored default | reported as the **value** — `x`, not `'x'` — and only marked `DEFAULT_GENERATED` when it is an expression, so a text literal is quoted on the way in (`defaultExpression`) | already an expression: `'x'::character varying` |
| placeholders | `?` | `$1` |
| `sqlkit.Dialect` | backtick quoting, `#` comments, `\` escapes a string | double-quote quoting, `$tag$` bodies |
| levels above a table | one pool, `USE <database>` | a pool per database, `SET search_path` |
| `ddl.go` | a qualified rename target; every primary key is called `PRIMARY` | an unqualified rename target; a key's constraint name has to be read before it can be dropped |
| the four `Dialect` flags | `RestatesColumn`, `MethodBeforeTable` | `IndexIsObject`, `ForeignKeyIsConstraint` |
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

**`AlterStatements` is the third of the same arrangement**: it spells a list of `driver.TableChange`
as the statements that would make them and runs none of them, so a migration shows the reader what
its Apply will do rather than a description of it. Every kind on that list has a *run* beside it on
the same interface, and both go through one speller on `sqlkit.Dialect`, so the two cannot drift.
Spelling needs no connection and raises no error: a change an engine has no concept of contributes
**nothing**, and the run of it is where `domain.Unsupported` is answered.

Four statements that used to be written out per engine now live on `Dialect` behind flags, which is
the form every other difference between the two takes: `RestatesColumn` (MySQL's `MODIFY COLUMN`
against one `ALTER COLUMN` action per fact), `MethodBeforeTable`, `IndexIsObject` and
`ForeignKeyIsConstraint`. What is left in `mysql/ddl.go` and `postgres/ddl.go` is what genuinely
cannot be shared: the catalogue reads, and `SetPrimaryKey`, which on PostgreSQL has to *read* the
constraint's name before it can drop it.

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
| `document.go` | HTML and Markdown, written and never read — see [features/transfer.md](features/transfer.md#the-import-and-export-wizards). |
| `encoding.go` | The encodings the whole app reads files as, one list. UTF-8 is `UTF8BOM`: its decoder strips a mark a Windows tool left, and its encoder writes one, which is what makes Excel open a UTF-8 CSV as UTF-8. |

`internal/app/transfer.go` mirrors the format and encoding lists onto the wire so the wizard draws a
list the Go side decides, `import.go` maps a file's fields onto a table's columns and runs what
`RowStatements` builds, and `export_job.go` streams each table into a file of its own.

A table is addressed by a `driver.Namespace` — `(database, schema)` — and how many of its parts an
engine fills is the one structural difference between the SQL adapters. MySQL fills `Database`
alone; PostgreSQL fills both and needs a connection pool per database to do it, because a session
cannot leave the database it was opened with. MongoDB fills `Database` alone and needs one pool for
the *server*, a single connection reading every database on it. See [decisions.md](decisions.md).

## Comparing two tables

`internal/compare` is the whole of the difference between two tables and knows nothing about how
either was read. `internal/app/compare.go` is what reads them — two shapes, and for the data half
two bounded pages — and hands both to it.

**The left is what the reader has and the right is what they are comparing it against.** `added`
means the right holds it and the left does not; `removed` is the other way about. Nothing in the
package decides which table is which: the caller does, and every word follows from that.

**Nothing is matched by position.** A field, an index and a constraint are matched by *name* and a
row by its *key*, and `pairNames` is the one function all three shapes go through — it walks the
left in its own order and emits whatever the right holds before the next shared name, so a column
added in the middle is drawn in the middle rather than pushed to the end. What comes back is a list
of *pairs*, which is why the two panes drawing it can never fall out of step: they are built from
one list and hold the same rows in the same order, a blank standing where a side has not got the row.

**The key is read from the shapes, before a single row is.** `KeyFor` answers with the primary key
both sides declare, or — where they share none — with every column they share, and it says which
(`Key.Declared`). That matters twice. The reads are **ordered by that key** before they are cut to
the row limit, so a bounded compare compares comparable windows of the two tables rather than two
arbitrary hundreds; and on the fallback nothing can come back `changed`, a row being matched on the
whole of itself, so the reader is told rather than shown "no changed rows" as if it were a finding.

**A comparison is bounded on purpose, and says so.** Both sides are held in memory at once and cross
the bridge as one answer, so there is no `All` here the way there is on a table tab — that read is
*streamed*, and a diff cannot be, needing both sides together. `app.DefaultCompareRows` is the
default and `DataDiff.leftTruncated` / `rightTruncated` are how a window says it was not a table.

Two more rules are the package's and not a caller's. **Only the columns both sides hold are
compared**, and the rest are *named* rather than silently dropped: a value with nothing opposite it
is a difference between two shapes, and the structure half is where that is read. And **a key held
more than once pairs first-to-first**, so two identical rows on one side pair with two on the other
rather than with one twice — which is exactly what an unkeyed table's comparison is made of.

### Making the two agree

A comparison the reader can act on is a *migration*, and it adds nothing to `internal/compare`: the
diff answers what differs, and turning a ticked difference into a change is the frontend's
(`features/compare/migrate.ts`), because only it knows which way the reader pointed it and what they
ticked. What that produces is the same two shapes the driver already takes — a list of
`driver.TableChange` for a shape, a `driver.RowWrite` for rows — and **one list goes to both halves
of the round trip**: `PreviewTableChanges` spells it, `ApplyTableChanges` runs it, so the script the
reader copied is the script that ran. `PreviewRowWrite` and `ApplyRowWrite` are the same pair over
`RowStatements`, which is why a migration's rows and an import's are the same statements.

Two things about applying are deliberate. **It stops at the first failure**, because the order is
what makes a migration work — what is going goes first, what is arriving is made last — so a step
that failed leaves every step after it meaningless. And **there is no transaction around it**:
neither engine can put one around DDL in a way that holds for all of it, MySQL committing implicitly
at every statement. The preview is what protects the reader, which is why it exists.

## Looking for text across a schema

A Find in Schema is **two questions**, and the split between them is the dependency rule in one
feature.

**What the reader's text matches is a `domain` rule.** `domain.TextMatch` is the text, the mode and
the case box, and it spells that one rule three ways: a **LIKE pattern** for the two modes a pattern
can say, a **regular expression** for everything — including those two, translated, for an engine
that has no LIKE — and a **compiled matcher** for the one place the matching happens here rather than
on a server. Nothing about it is an engine's, which is what stops `Contains` meaning one thing of a
row and another of a column name.

**How many rows hold it is the driver's — and so is the read the reader opens.**
`driver.Session.MatchingRows` answers with both (`driver.RowMatches`): the count, and the `SELECT` that
would return exactly the rows counted, spelled from the **same** clause. That is the arrangement
`TableRows.Statement` and `PreviewTableChanges` already have, and it is why a double-click on a hit can
open a query tab without anything outside a driver having spelled a statement. It counts one table's,
because
only the engine knows how to read a column of any type as text and how to quote what it is compared
against: `sqlkit.Dialect.MatchAny` spells the whole clause for both SQL engines, and the three ways
they differ are three flags — `RegexIsFunction` (MySQL's `REGEXP_LIKE`, whose third argument is the
sensitivity, against PostgreSQL's `~` and `~*`), `PatternIgnoresCase` (MySQL's collations ignore case,
so a *sensitive* pattern is compared as bytes) and `WordBoundary` (`\b` against `\y`, where `\b` is a
backspace). MongoDB writes its own filter, a `$regex` per sampled field, because a document store has
no LIKE and no quoting at all.

**Which names hold it asks no server anything.** A field, an index and a constraint are names
`TableStructure` already answers with, so `internal/app/search.go` reads the shape and asks the
matcher — one table at a time, either way, so a table nobody may read is one row missing from the
answer rather than the end of the search. The whole search is one bound method with one deadline and a
`StopSearch`, for the reason a read of every row has one: how long a schema takes is the schema's
business and not this side's.

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

## The key-value store

`internal/driver/redis` is the second adapter that is not a `database/sql` one, and it differs from
the first in a deeper place. MongoDB changed what a *row* is; this changes what a **table** is,
because there are none.

**The key space is the table.** A Redis database is one flat namespace of keys, so the adapter
answers `Objects` with exactly one table, called `(All Data)` — the name every other client gives it
— holding one row per key: `Key`, `Type`, `Value`, `TTL` and `Size`. Those five columns are not read
from anywhere and are not inferred from anything: unlike a document store's sampled fields, they are
the same five on every Redis server there has ever been. A table by any other name is
`domain.ErrUnknownTable`, which is the honest answer — there is nowhere for it to be.

**And because that list is fixed, the count comes with it.** The tree draws beside a namespace how
much is in it, which on a table store is what the list already said; here the list is one table and
always will be, so `Objects` carries `KeyCount` — `DBSIZE` — and the badge beside a Redis database is
how many keys it holds. It is read in that same call rather than a second one, so that opening the
folder and being told its size stay one round trip.

**A page is cut from a scan, so the names are held and the values are not.** `SCAN` is a cursor with
no offset and no order, so `keys.go` walks the whole space, **sorts the names**, and only then cuts
the page — because a page cut from an unordered list is a different page every time it is read. The
values of that one page are then fetched in **two pipelines**: one for what each key *is* (its type,
its clock, its cost) and one, which needs the first's answer to know what to ask, for what it holds.
A hundred rows is two round trips rather than four hundred. The walk is bounded at `scanCap` keys and
**says so** rather than showing a window that lies about how big it is.

**A filter is answered here, and a glob is the part of it the server can do.** Redis narrows a read
exactly one way — a glob against a key's name — so `filter.go` takes that glob from the top level of
an `AND` and applies the whole tree to what comes back. A condition inside an `OR` never becomes the
glob: `key begins user OR type is hash` admits every hash in the database, and narrowing the scan by
the first half would hide rows the reader asked for. Nothing in the package quotes anything, for the
reason nothing in the MongoDB one does: a pattern crosses as an **argument**.

**A key's type is what its value is, so the two are written together.** `driver.CellEdit.ValueType` is
that one field: a write of the `Value` column may name the type it is to be written *as*, and
`writeValue` empties the key and refills it — which is the whole of what a Redis key changing type can
mean. It is not two writes because it could not be: a value in the new type's shape is refused by the
old type's parser, and a type change alone has nothing but the old value to re-express. So the `Type`
column is `domain.Unsupported` on its own, and empty `ValueType` means "whatever the key already is",
which is every edit on every other engine.

**A value is JSON, and that is what makes it editable.** A string is itself; the five container
types are spelled as JSON — a list as an array, a hash and a sorted set as objects, in field and rank
order — so what the grid draws is exactly what an edit hands back, and there is no second notation to
learn. A value too large for a row is **sampled and marked**, and the mark is what makes writing it
back fail: an edit of a sample would drop whatever was not shown.

**The sample is the page's bound and not the value's**, which is what `TablePage.Whole` is for: a read
of one key asks for its value as it *is*, and the panel under the grid makes exactly that read. It is
`wholeValue` and not the same commands with a bigger number, because `HSCAN` and `SSCAN` are cursors —
one call of either answers with whatever the server felt like, COUNT being a hint — so `HGETALL` and
`SMEMBERS` are what "all of it" is spelled as. Such a read is one key by construction, so it is a
command per value and no pipeline. It is also why a filter naming exactly one key is **looked up**
(`keyNames`, one `EXISTS`) rather than scanned for: `SCAN` walks the whole space however narrow its
pattern, and reading one value must not cost a walk of a million keys.

**How the reader is shown that key space is the frontend's, and asks the server nothing more.** The
keys group into a tree on the colon, the panel under the grid edits a container member by member, and
the bar over it is two boxes rather than the filter builder — and not one of the three is a second way
into the driver. The tree is a *reading* of the page this side already holds, the members are read out
of the JSON `value.go` wrote and go back as the JSON `parseValue` reads, and the two boxes write the
same `Key contains …` / `Type = …` conditions the builder would have. See
[features/key-space.md](features/key-space.md).

**A command is argv, not a sentence.** `command.go` rebuilds each argument from the tokens
`sqlkit.Lex` already produced — an argument is a *run of adjacent tokens*, since `user:1` is three
tokens and one argument — so there is no second scanner here either. What a command then *does* is
`domain.RedisHazard`, the fourth grammar for one rule beside `StatementHazard`, `MongoHazard` and
`ImportHazard`; `FLUSHDB` is classified **unfiltered** rather than destructive, because it names
nothing and therefore takes everything, and that is the one class answered for on a laptop as well as
on production.

| Part | Redis |
|---|---|
| a DSN | none: `options.go` builds go-redis's own options, for the reason MongoDB builds its driver's |
| a dialler and a TLS configuration | values on those options, so there is nothing registered globally and nothing to release with the session |
| `errors.go` | the first word of the server's message → sentinel, that word being what Redis puts a class in. Plus a text read for a password rejection, which has no class — see [errors.md](errors.md) |
| catalogue reads | there is no catalogue. `SCAN` for the keys, `TYPE`/`PTTL`/`MEMORY USAGE` for each, `DBSIZE` and `INFO keyspace` for the counts, `CONFIG GET databases` for how many databases there are |
| placeholders | none; every argument is a value |
| the lexer's dialect | `#` comments, both quotes open a string, a backslash escapes inside one — which is `redis-cli`'s own grammar |
| levels above a key | a client **per database**, because `SELECT` is connection state and a pool that switched underneath a read would answer from somewhere else |
| statement terminator | a **newline**. A semicolon is read as one too, since every file this app writes ends its statements with one and a reader coming from another tab will type one |
| `ddl.go` | nothing to declare, so nearly all of it is `domain.Unsupported` naming what is missing. `EmptyTable` is `FLUSHDB` and `AlterStatements` spells nothing at all |
| `database.go` | there is none. A server is started with a fixed number of databases and they all exist from that moment — the opposite of MongoDB, where one exists only as long as something is in it |
| a search | `MatchingRows` is a glob over key *names*, case folded into the pattern itself (`[uU][sS][eE][rR]*`) because Redis has no insensitive matching. The two modes a pattern cannot say are refused rather than approximated, and the statement it answers with is `KEYS <glob>` — which returns exactly what it counted |
| what it has not got | a second table, a column, an index, a foreign key, a routine, a view, a plan, a database it can make or drop, a truncate distinct from an empty. Each says so through `domain.Unsupported` |

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
