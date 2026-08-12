# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## What this is

`vibe-db` is a desktop database client in the Navicat Premium mould — a Go backend in a Wails v2
shell (OS WebView2, no bundled Chromium) with a React 19 + TypeScript frontend. MySQL, PostgreSQL,
SQLite, SQL Server, Oracle, MongoDB and Redis are implemented; the driver layer is a port so others
are added as adapters, `internal/driver/sqlkit` holds everything about a `database/sql` adapter that
does not vary by engine — including `sqlkit.DDL` and `sqlkit.Data`, which are the whole of a schema
change and the whole of a read or write of rows — `internal/driver/mongo` is the proof the port is
not secretly a SQL one, and `internal/driver/redis` is the proof it does not secretly assume tables
either. What each engine has and has not got is [docs/features/engines.md](docs/features/engines.md).

## Rules that must not be broken

1. **The dependency rule is one-way:**
   `internal/app` →
   `session / profile / secret / history / snippet / transfer / compare / backup / dbtool` →
   `internal/driver` → `internal/domain`.
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
   nothing it reads is ever a statement, and nothing in it reaches a server except as a value. **A
   saved snippet is not a script either, because nothing reads it at all** — `internal/snippet` keeps
   a body byte for byte and never splits, lays out or checks one; it becomes a statement when the
   editor it is put into runs it, on the path every statement takes.
16. **A SQL adapter is its catalogue reads, its error map, how it is dialled, and a `Dialect`** —
    and nothing else. `sqlkit.DDL` spells and runs every schema change, `sqlkit.Data` builds every
    read and write of a row, and **an adapter that writes a `SELECT`, an `UPDATE`, an `INSERT` or a
    `DELETE` of its own has gone wrong**. What differs between five SQL engines is a *flag* on
    `Dialect` — where a page's bounds go, how an argument is numbered, whether an index is dropped
    by its own name, what an empty insert is spelled as — never a second speller. An override on an
    adapter is for a difference in **shape**: a rename that is a procedure, a key whose constraint
    name has to be read first, a thing the engine has not got at all.
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
17. **A view, a routine and a trigger are edited as the statement that declares them, and that
    statement is the engine's own words.** `driver.Schema.Source` is a *read* wherever the engine will
    answer for itself — `SHOW CREATE`, `pg_get_functiondef`, `pg_get_triggerdef`, `sqlite_master`,
    `sys.sql_modules`, `all_source` — for the reason `CreateStatements` is one, and where an engine
    answers with the body alone the `CREATE` in front of it is the **adapter's** to spell. The buffer
    travels back **verbatim**: a definition is one statement however many semicolons a body holds, so
    nothing re-lexes it and no reader ever says `DELIMITER`. A save **drops before it creates**, there
    is no transaction around the two, and `SourceStatements` previews exactly what `SaveSource` runs —
    rule 13 applied where it matters most. A definition is also the one thing here run **inside** its
    namespace rather than named into one (`sqlkit.DDL.Pin`), because its body is the reader's text and
    an unqualified name in it resolves wherever the connection is. What names one is a whole
    `driver.SourceRef` and never a name: PostgreSQL tells two functions apart by their arguments and
    finds a trigger by its **table**, which is a flag (`TriggerOnTable`). The skeleton a *new* one
    starts from is a flag on `sqlkit.Dialect` (`Starters`) and never a template in a component. An
    engine whose objects are not declared by a statement — MongoDB's view, a Redis key space — answers
    `domain.Unsupported` and says so to the frontend as `hasSource`.

18. **An engine's own programs are run by `internal/dbtool`, and no secret is ever an argument.**
    mongodump, mysqldump and pg_dump are the *vendor's* — this app opens no connection for them,
    reads not one byte of what they write, and is not on the machine's hook for installing them.
    Which pair an engine has and which of the four options that pair offers are `domain` rules
    (`Engine.Toolkit`), so a box the dialog does not draw is a thing the program has not got. What
    each program is *called* is `command.go`'s alone — one request, three spellings, the
    `sqlkit.Dialect` arrangement applied to programs. A command line is readable by every process on
    the machine, so a password goes in a file written at `0600` and removed when the run ends, which
    is what lets the log open with the command line itself. A **tunnelled** profile and a **cloud
    identity** are `domain.Unsupported` naming which, never run against the wrong host.

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
    [outlined column](docs/grid.md), and replaces Filter & Sort with two boxes that write ordinary
    `Key …` / `Type = …` conditions — the name matched three ways, one operator each, every one of
    them spelled as the glob the scan is narrowed by. Never a second read, and never a
    second notation — a **hash** and a **sorted set** are laid out as the pairs they hold and every
    other value is read whole in `ValueViewer`, both out of the driver's own JSON. Those rows are
    picked and never typed: the panel below is where a key is edited, and a click on a branch edits
    nothing, a prefix being a folder.

19. **A diagram draws the catalogue, and a line on it is a constraint the server holds.** A box names
    a table; its fields, its key marks and every line between the boxes are read from the same
    catalogue cache the tree and the designer read, so nothing there can show a key that is not
    there. Drawing a line therefore **declares** one — through the bound method the tree's own Add
    Foreign Key calls, never a second way of spelling it — and the line appears because the catalogue
    now says so. What is the *reader's* is the arrangement alone, which is stored per namespace
    beside the code that reads it and goes to no server; so `Delete` there takes a box off the
    picture, and dropping a table stays a schema change asked for where every other one is.

20. **Autocommit is off for the whole session or for none of it.** A manual transaction is *one
    connection* held open — that is the whole of what a transaction is — so while one is open
    **every** statement the session runs goes inside it: a table tab's reads, its cell edits, a
    query tab's statements and a schema change from the tree alike. `sqlkit.Txn` holds that
    connection and is the whole of `driver.Transaction` for the five SQL engines; the two seams that
    ask it before reaching for a pool are `sqlkit.DDL.exec` and `sqlkit.Data.queryer`, and a
    statement that went round either would be one `Rollback` could not undo. Nothing here spells
    `BEGIN` — it is `database/sql`'s to send — so there is no `Dialect` flag and no adapter
    override, only the same `pinned` a definition is run on. What an engine has not got is
    `domain.Unsupported` naming it and `EngineInfo.hasTransactions` saying so to the frontend.

21. **An account is the server's, and one speller grants everything.** `driver.Security` is the third
    half of a session: who a server knows, what each of them has been granted, and the changes that
    alter it. A privilege is **read** wherever the engine will answer for itself — `SHOW GRANTS`,
    `information_schema.*_privileges`, `sys.fn_builtin_permissions`, `dba_sys_privs`, `usersInfo`,
    `ACL LIST` — and every statement that changes one is spelled by `sqlkit.Auth` alone; what differs
    between five SQL engines is a **flag** on `Dialect` (an account that *is* `name@host` and so is
    two string literals, a login separate from a user, a third verb `DENY`, `WITH ADMIN OPTION`
    against `WITH GRANT OPTION`), never a second speller. `AccountStatements` previews exactly what
    `ApplyAccountChanges` runs — rule 13 again — with **one deliberate exception: a password is never
    spelled into a preview**, because a preview is what reaches a screen, the History Log and a saved
    history; the speller takes it as an argument, so the shown and the run still come from one
    function. What is dangerous is a `domain` rule (`domain.GrantHazard`, the fifth grammar), and a
    grant that **named nothing** is unfiltered in the sense a `DELETE` with no `WHERE` is — confirmed
    on a laptop as well as on production. An engine with no accounts answers `domain.Unsupported`
    naming it and `EngineInfo.hasAccounts` says so to the frontend; what one is *called* is
    `accountLabel`, never a comparison against an engine's name.

22. **What a server is doing is read, and one speller ends it.** `driver.Monitor` is the fourth half of
    a session: the processes a server is running, what it has counted since it started, what it was
    started with, and the two statements that end one. Each read is the **engine's own** — a process
    list is a catalogue read like a table list, in that engine's own columns, and two of them are not
    name-and-value pairs at all (PostgreSQL has no `SHOW STATUS`; a document store's is one nested
    document) — while `sqlkit.Watch` spells and runs **every** kill there is: what differs between five
    SQL engines is a flag on `Dialect` (`CancelStatementSQL`, `CloseConnectionSQL`, and Oracle's session
    being the pair `sid,serial#` in a string literal), never a second speller. `ProcessStatements`
    previews exactly what `KillProcesses` runs — rule 13 again, and here the preview *is* the
    confirmation, there being no Changes pane. **A process id is checked as a number before it reaches
    a statement**: a kill binds no arguments, so this is the one place a value off the wire would
    otherwise be pasted into SQL. What is dangerous is a `domain` rule (`domain.ProcessHazard`, the
    sixth grammar), and ending the connection **this app is reading on** is unfiltered in the sense a
    `DELETE` with no `WHERE` is — confirmed on a laptop as well as on production — which is why a
    reference carries the driver's own `Own` mark and nothing re-derives it. A **variable is read and
    never written**: setting one is five grammars, two of which need a restart. An engine with no server
    to watch answers `domain.Unsupported` naming it and `EngineInfo.hasMonitor` says so to the frontend;
    which kill it has is `cancelsStatement` and `closesConnection`, and what one is *called* is
    `processLabel`, never a comparison against an engine's name.


## Where to read before changing something

| Task | Read |
|---|---|
| Build, test, run against a real database | [docs/development.md](docs/development.md) |
| Touch a layer boundary, or add an engine | [docs/architecture.md](docs/architecture.md) |
| Find out what an engine has, or has not | [docs/features/engines.md](docs/features/engines.md) |
| Touch a SQL adapter, or a `Dialect` flag | [docs/architecture.md](docs/architecture.md#the-five-sql-engines) |
| Touch anything MongoDB | [docs/architecture.md](docs/architecture.md#the-document-store) |
| Touch anything Redis | [docs/architecture.md](docs/architecture.md#the-key-value-store), [docs/features/key-space.md](docs/features/key-space.md) |
| Add or change a failure mode | [docs/errors.md](docs/errors.md) |
| Find out what exists at all | [docs/features.md](docs/features.md) — the index, one file per feature under [docs/features/](docs/features/) |
| Change how a connection is made or guarded | [docs/features/connections.md](docs/features/connections.md) |
| Change the tree, the table list or a schema change | [docs/features/schema-tree.md](docs/features/schema-tree.md), [docs/features/schema-changes.md](docs/features/schema-changes.md) |
| Change a view's, a routine's or a trigger's definition | [docs/features/definitions.md](docs/features/definitions.md) |
| Change a query tab, a console or a file run | [docs/features/query-tab.md](docs/features/query-tab.md), [docs/features/script.md](docs/features/script.md), [docs/features/console.md](docs/features/console.md) |
| Turn autocommit off, or touch Begin / Commit / Rollback | [docs/features/transaction.md](docs/features/transaction.md) |
| Touch a user, a role or a privilege | [docs/architecture.md](docs/architecture.md#who-may-do-what), [docs/features/accounts.md](docs/features/accounts.md) |
| Touch a process list, a kill, a server status or a variable | [docs/architecture.md](docs/architecture.md#what-a-server-is-doing), [docs/features/monitor.md](docs/features/monitor.md) |
| Change what is kept about a statement that has run | [docs/features/log.md](docs/features/log.md), [docs/features/results.md](docs/features/results.md#the-history-pane) |
| Touch a named snippet, the library file, or what completion offers by name | [docs/features/snippets.md](docs/features/snippets.md), [docs/editor.md](docs/editor.md#what-completion-offers) |
| Change a table tab, its paging, filtering or values | [docs/features/table-tab.md](docs/features/table-tab.md), [docs/features/row-limit.md](docs/features/row-limit.md), [docs/features/filter-sort.md](docs/features/filter-sort.md), [docs/features/value-viewer.md](docs/features/value-viewer.md) |
| Touch a file format, an import or an export | [docs/architecture.md](docs/architecture.md#carrying-a-table-to-a-file), [docs/features/transfer.md](docs/features/transfer.md) |
| Touch a backup or a restore | [docs/architecture.md](docs/architecture.md#backing-a-database-up), [docs/features/backup.md](docs/features/backup.md) |
| Touch mongodump, mysqldump, pg_dump — or anything this app *runs* | [docs/architecture.md](docs/architecture.md#running-the-engines-own-programs), [docs/features/tools.md](docs/features/tools.md) |
| Touch a comparison of two tables | [docs/architecture.md](docs/architecture.md#comparing-two-tables), [docs/features/compare.md](docs/features/compare.md) |
| Touch the diagram: a box, a line, a note, the canvas | [docs/features/diagram.md](docs/features/diagram.md) |
| Touch a search across a schema | [docs/architecture.md](docs/architecture.md#looking-for-text-across-a-schema), [docs/features/search.md](docs/features/search.md) |
| Touch any table, grid, cell or selection | [docs/grid.md](docs/grid.md) |
| Touch the SQL editor or completion | [docs/editor.md](docs/editor.md) |
| Write a component, a store, or a style | [docs/frontend.md](docs/frontend.md) |
| Wonder why something looks wrong | [docs/decisions.md](docs/decisions.md) |
| Claim anything about memory | [docs/memory.md](docs/memory.md) |

A new failure mode needs all four of: a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table in [docs/errors.md](docs/errors.md).

A new engine capability the UI has to know about is a field on `EngineInfo` in
`frontend/src/bridge/connections.ts` — `hasForeignKeys`, `hasRoutines`, `hasTriggers`, `documents`,
`keys`, `isFile`, `createsTables`, `createsDatabases`, `hasViews`, `hasSource`, `hasBackups`, `hasTransactions`,
`hasAccounts`, `accountLabel`, `hasRoles`, `hasMonitor`, `cancelsStatement`, `closesConnection`,
`processLabel`,
`dumpTool`, `restoreTool`, `needsUser`, `needsDatabase`, `databaseLabel` — and never a comparison
against an engine's name in a component.
The two exceptions are the lists that *are* per-engine data rather than a capability: the types a
designer offers (`features/schema/dataTypes.ts`) and the dialect the editor highlights with
(`features/editor/sqlLanguage.ts`).

## How the docs are written

A change to behaviour is a change to the document that describes it, in the **same commit**. Which
document that is follows five rules:

1. **One feature to a file, named after the feature.** [docs/features.md](docs/features.md) is an
   index and holds no behaviour of its own; every feature lives in its own file under
   [docs/features/](docs/features/).
2. **About 250 lines is the ceiling.** A feature document past it is holding more than one feature:
   split it by feature, name the parts, and add their rows to the index — never add a fourth `##`
   to a file that is already long.
3. **A document that is one contract stays whole**, however long it runs.
   [docs/grid.md](docs/grid.md) is the grid's specification and [docs/errors.md](docs/errors.md) is
   the error table; splitting either would mean two files to read before one change, which is the
   cost these rules exist to avoid.
4. **One fact in one place, reached by a link.** A fact written in two files is a fact that will
   disagree with itself. The file that owns it is the one whose feature it is, and everything else
   links to `features/<file>.md#the-heading` — a file *and* a heading, so a reader lands on the
   paragraph rather than at the top of a document. From inside `docs/features/`, a sibling document
   is `../grid.md`.
5. **A new feature is a new file, a row in the index, and — if it is a place a change starts — a row
   in the table above.** Not a section appended to the nearest existing document.

Commands, in short: `wails dev`, `wails build`, `go test ./...`, `go vet ./...`,
`gofmt -l internal main.go`, and `pnpm typecheck` in `frontend/`.
