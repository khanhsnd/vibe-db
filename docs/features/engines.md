# Engines

[← Features and UI behaviour](../features.md)

Seven engines, and the app is the same app on all of them. This page is the one place that says
**what each has and what each has not** — everything else in the docs describes a feature once and
links here for where it is missing.

Nothing on this page is a comparison against an engine's name in a component. Every row of it is
either a field on `EngineInfo` in `frontend/src/bridge/connections.ts`, a rule on `domain.Engine`,
or a flag on `sqlkit.Dialect`. See [architecture.md](../architecture.md#adding-a-database-engine).

## What a connection asks for

| Engine | Default port | Database box holds | User |
|---|---|---|---|
| MySQL | 3306 | a database to start in, optional | required |
| PostgreSQL | 5432 | the database the session is bound to, **required** | required |
| SQLite | — | **a path to a file**, required | none |
| SQL Server | 1433 | a database to start in, optional | required |
| Oracle | 1521 | the **service** the listener is asked for, required | required |
| MongoDB | 27017 | a database to start in, optional | none |
| Redis | 6379 | a database *number*, blank is `0` | none |

**A file engine has no server, so the dialog does not draw one.** SQLite's General tab is a single
`Database file` box with a `Browse` beside it: no host, no port, no user, no password — not disabled
but *absent*, because a greyed-out Password would suggest the file might have one. The file has to
be there already; SQLite would otherwise make an empty one at the first touch and a mistyped path
would open as a database with no tables in it rather than as the mistake it is.

**An SSH tunnel is refused for it** rather than quietly ignored. A reader who configured one expects
the file to be somewhere else, and opening a local file of the same name instead would be the worst
possible answer.

## How deep the tree goes

**Two engines have a level between the connection and the tables, and they are not the two you would
guess.** PostgreSQL and SQL Server both name a table by a schema inside a database. MySQL, SQLite
and Oracle each have one level — and Oracle's is not a database at all: an Oracle connection asks
the listener for a *service*, and everything inside it is owned by a user, so what the tree lists
under an Oracle connection are **schemas**.

The difference that follows from it is where the pools go. A PostgreSQL session is bound to its
database for life, so browsing a second one means dialling it — a pool per database, opened lazily.
SQL Server names a table by its database *and* its schema, so a three-part name reaches any database
on the server and **one pool reads all of them**.

## What each engine can be asked to do

A blank is not a gap to be filled in later. It is a thing the engine has not got, and asking for it
answers [`UNSUPPORTED`](../errors.md) naming which — never a statement left for the server to refuse.

| | MySQL | PostgreSQL | SQLite | SQL Server | Oracle | MongoDB | Redis |
|---|---|---|---|---|---|---|---|
| Make and drop a database | ✓ | ✓ | | ✓ | | ✓ | |
| Make, rename, duplicate and drop a table | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Change a column | ✓ | ✓ | | ✓ | ✓ | ✓ | |
| Set a primary key | ✓ | ✓ | | ✓ | ✓ | | |
| Add and drop an index | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Add and drop a foreign key | ✓ | ✓ | | ✓ | ✓ | | |
| Truncate as well as empty | ✓ | ✓ | | ✓ | ✓ | | |
| Views | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Stored routines | ✓ | ✓ | | ✓ | ✓ | | |
| Read and write a definition | ✓ | ✓ | ✓ | ✓ | ✓ | | |
| Explain a statement | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Back up and restore | ✓ | ✓ | ✓ | ✓ | ✓ | | |
| Its vendor's own dump program | ✓ | ✓ | | | | ✓ | |

Four of those blanks are worth the sentence:

- **SQLite cannot change a column, set a key, or add a constraint.** A SQLite database is the *text*
  of the statements that made it, kept and re-parsed, and there is no `ALTER TABLE … ALTER COLUMN`.
  It is not approximated by rebuilding the table behind the reader's back: a rebuild drops the
  triggers, the views and the constraints that pointed at it, which is not what "change this column"
  means. A migration therefore shows an **empty script** rather than one that would fail — see
  [schema-changes.md](schema-changes.md).
- **SQLite has no TRUNCATE.** It is not a faster DELETE there, it is nothing at all: the
  optimisation that makes `DELETE FROM t` drop the pages wholesale happens by itself. Offering both
  would be offering one command under two names.
- **SQLite has no stored routine.** A function there belongs to the program that opened the file —
  this app registers one, `regexp`, and it is gone the moment the file is closed. Its *views* are
  read and written like any other engine's: the file stores the statement that made each one.
- **MongoDB has no definition to read.** A view there is a collection declared by a *document* —
  the collection it reads and the pipeline over it — rather than by a statement, so there is
  nothing for an editor to open and nothing for a Save to put back. See
  [definitions.md](definitions.md#what-a-definition-is).
- **Three engines have no dump program of their own, and MongoDB has one without having a backup.**
  A SQLite database is one file, so copying it is the dump; SQL Server's backup is a statement the
  *server* runs, writing where the server can see; Oracle's Data Pump runs inside the instance. And
  a backup here is a file of *statements*, which a document store's commands are not — but its
  vendor ships mongodump, which carries the BSON itself. See [tools.md](tools.md).
- **Oracle makes no schema.** A schema there *is* a user, and making one is `CREATE USER … IDENTIFIED
  BY …` — an account with a password, a quota and a set of privileges. That is administration rather
  than the New Database dialog's business, and there is nowhere on the dialog for a password to go.
  Dropping one is `DROP USER … CASCADE`, which removes an account and everything it ever owned: the
  same answer, rather more emphatically.

## What differs inside a statement

Every one of these is a flag on `sqlkit.Dialect` and never a second speller. The table is here
rather than in the code because it is the shape of the *set* — what a reader adding an eighth engine
has to answer.

| | MySQL | PostgreSQL | SQLite | SQL Server | Oracle |
|---|---|---|---|---|---|
| A name is quoted | `` `x` `` | `"x"` | `"x"` | `[x]` | `"x"` |
| A bound argument | `?` | `$1` | `?` | `@p1` | `:1` |
| A page is cut | `LIMIT … OFFSET …` | same | same | `OFFSET … FETCH NEXT …` | same as SQL Server |
| A row already there | `ON DUPLICATE KEY` | `ON CONFLICT` | `ON CONFLICT` | guarded insert | guarded insert |
| Several rows to one INSERT | ✓ | ✓ | ✓ | ✓ | one per row |
| An altered column | restated whole | one action per fact | — | restated, default apart | restated whole |
| An index is dropped | through its table | by its own name | by its own name | by name **and** table |by its own name |
| A table is copied | `LIKE` | `LIKE` in the column list | `AS SELECT` | `SELECT … INTO` | `AS SELECT` |

**A page needs an order on exactly one of them.** SQL Server refuses `OFFSET` without an `ORDER BY`,
so a read of a table with no key and no chosen sort is given `ORDER BY (SELECT NULL)` — an order that
orders nothing. A column picked here instead would silently decide which rows the reader sees.

**Two of them have a `MERGE` statement and neither uses it.** A row that may already be there is
written as an insert guarded by `WHERE NOT EXISTS`, with an update in front of it for an upsert —
two ordinary statements saying what a page of `MERGE` syntax says. On Oracle the guarded insert
reads its literals `FROM dual`, a `SELECT` there needing a table.

## What a search matches

[What the reader's text matches is one `domain` rule](search.md); how it is spelled is the engine's,
and the five SQL engines spell it four ways.

| Engine | A pattern | An expression | Case-sensitive |
|---|---|---|---|
| MySQL | `LIKE` | `REGEXP_LIKE(…, 'c'\|'i')` | the pattern cast to `BINARY` |
| PostgreSQL | `LIKE` | `~` and `~*` | already is |
| SQLite | — | `REGEXP`, flags in the pattern | — |
| SQL Server | `LIKE` | `REGEXP_LIKE(…, 'c'\|'i')` | `COLLATE Latin1_General_BIN2` |
| Oracle | `LIKE` | `REGEXP_LIKE(…, 'c'\|'i')` | already is |

**SQLite spells every mode as an expression, and that is the better answer rather than a workaround.**
Its own `LIKE` ignores case for ASCII and for nothing else, which would make the Case Sensitive box
true of `a` and false of `á`. So the adapter registers `regexp` — the operator SQLite's grammar has
always named and its library has never implemented — over **Go's own** regular expressions. On this
engine alone, the rows of a search and the *names* of one are matched by literally the same matcher,
`domain.Matcher` being compiled from the same package.

**`REGEXP_LIKE` on SQL Server needs a server that has one**, which means SQL Server 2025 or Azure SQL.
An older server answers the two modes a pattern cannot say — Whole Word and Regular Expression — with
its own "not a recognized built-in function", which names the missing piece exactly. It is left to
the server rather than gated on a version here: a client that guessed the version wrong would refuse
a search the server would have answered.

## What a dump is made of

`CreateStatements` is the structure half of a dump, and an engine that can answer for itself is
**asked** rather than described.

- **MySQL** is asked: one `SHOW CREATE TABLE` carries the storage engine, the charset, every index
  and every constraint.
- **SQLite** is asked, and its answer is exact: the file *stores* the statement that made each
  object, so a dump is the declaration the reader wrote.
- **PostgreSQL, SQL Server and Oracle** are described, each having no such statement. Oracle *can*
  be asked — `DBMS_METADATA.GET_DDL` — and is not: what it answers with is the storage as well as the
  shape, the tablespace and the extents and the logging of the server it came from, none of which is
  the reader's and most of which the server they replay it on will not have.

What a description cannot carry is named where it is written: a PostgreSQL index over an expression,
a SQL Server identity or computed column, an Oracle function-based index. That is the honest cost of
an engine that will not answer for itself.
