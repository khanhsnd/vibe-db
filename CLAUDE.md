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
   `session / profile / secret / history / snippet / transfer / compare / backup / dbtool / schedule` →
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
17. **A declared object is edited as the statement that declares it, and that
    statement is the engine's own words.** There are seven kinds — a view, a function, a procedure, a
    trigger, a **materialized view**, a **sequence** and an **event** — and they are one machinery
    because they differ in one word, the one that drops them (`SourceKind.Keyword`). The three at
    the end exist on some engines and not others, and an engine without one answers
    `domain.Unsupported` naming it, exactly as it does for a routine. `driver.Schema.Source` is a *read* wherever the engine will
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
    started with, the handful of those numbers worth **watching move**, and the two statements that
    end one. Each read is the **engine's own** — a process list is a catalogue read like a table list,
    in that engine's own columns, and two of them are not
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
    **A chart is that same server read a fifth way, and it is one read.** `ServerMetrics` is a method
    of its own rather than a filter over the status, because on three of these engines the numbers are
    not in one catalogue — PostgreSQL keeps its sessions in `pg_stat_activity`, SQL Server its bytes in
    `dm_io_virtual_file_stats` — and a session count from one moment beside a rate from another would
    be two servers on one screen. **The four panels are `domain`'s and the series inside them are the
    engine's own** (`domain.MetricPanel`, `driver.Metric`), which is `ProcessList`'s arrangement and is
    here for a sharper reason: MySQL counts bytes on the wire and PostgreSQL counts pages off the disk,
    so a vocabulary insisting the six were one number would be this app inventing a fact — what a
    series is *called* is the adapter's, exactly as a column heading is. Whether a number has been
    adding up is the driver's to say as well (`domain.MetricKind`), a gauge and a counter sitting side
    by side in one status list; the frontend divides a counter by the seconds between two readings and
    never by a clock of its own, which is why the moment of a read is stamped where the read happened —
    and why a point is drawn **at its own instant** rather than at its place in the run, a reader who
    changes the clock or looks away otherwise having ten minutes and forty seconds drawn the same
    width. A counter this server's version has not got is **left out**, never sent as a zero: a flat
    line and a line the server will not draw are opposite answers. A **panel** is one kind, one unit
    and at most six series — it takes its axis from the first series it holds, so each of the three
    fails silently into a chart that looks perfectly ordinary, which is why `driver.PanelFault` states
    them and every adapter's own test holds its table to them.


23. **A column's description and a table's partitions are the two facts about a table's shape that no
    statement of this app ever reads.** A **comment** on a column is documentation the catalogue keeps
    beside it, and what differs between the engines is which of two shapes it takes — a *clause of the
    column* (`ColumnCommentInline`) or a `COMMENT ON COLUMN` *statement of its own*
    (`CommentsOnObject`) — never a second speller, and an engine with neither draws no box rather than
    one whose contents a Save would throw away (`EngineInfo.hasColumnComments`). A **partition** is a
    named piece of one table's rows, so it is drawn under that table beside its fields and its indexes
    and never in a folder of the namespace; the three grammars for changing one are a flag
    (`sqlkit.PartitionStyle`), the bounds are the reader's own words and travel verbatim the way a
    column's type does, and each of the three changes sits beside its spelling on `driver.Schema` —
    rule 13 again. What an engine has not got is `domain.Unsupported` naming it and
    `EngineInfo.hasPartitions` saying so to the frontend.

24. **A routine is the one declared object that is *called*, and one speller writes every call.**
    What it takes is a catalogue read like a table's shape (`driver.Schema.RoutineParams`) and never
    a reading of its body — a body is text, and a parameter list picked out of one would be a second
    parser. What differs between five SQL engines is a **flag** (`sqlkit.CallStyle`): a `CALL`, an
    `EXEC` or a PL/SQL block, a function read plainly or read in the `FROM` clause, and an `OUT`
    value handed back in a **user variable**, in the row `CALL` answers with, in a variable declared
    in the same batch, or not at all. Never a second speller. `CallStatements` previews exactly what
    `CallRoutine` runs — rule 13 again, and here what runs is the reader's *own program* — and an
    argument is written in as a **string literal**, for the reason a filter's value is: a call that
    bound its arguments could not be shown as the statement it is. The statements run on **one
    pinned connection** (`sqlkit.DDL.Pin`), because a user variable belongs to the connection and
    because rule 17's reason applies unchanged; so a call inside an open transaction joins it. What
    an engine has not got — a routine at all, or a value it will only hand to a bound variable — is
    `domain.Unsupported` naming which.


25. **What carries a connection is stood up above every adapter, and the outer hop is dialled first.**
    A proxy and a bastion are two hops and one shape — a function that opens a connection to an
    address, which is `driver.Dialer` — so `internal/netproxy` and `internal/sshtunnel` both hand one
    back and **no adapter has heard of either**. A proxy is how this machine reaches the network at
    all, so the order is fixed: the bastion is dialled *through* the proxy and the database through
    the bastion, and what the adapter is given is the innermost of them. A failure on an outer hop
    travels back **untouched** — a proxy that would not carry the connection to a bastion is a proxy's
    failure and never a tunnel's, because the reader fixes each in a different box. No secret is an
    argument here either: a proxy's password is the credential store's, under `<id>:proxy`.

26. **What a connection may do is decided once, and refusing is one wrapper.** `domain.Access` is the
    rule — `Environment.NeedsConfirmation`'s counterpart, reading the same `Hazard` and answering
    "not at all" rather than "are you sure" — and `session.readOnly` is the only thing that acts on
    it: the session handed out is already wrapped, so a schema change, an account change, a kill, a
    cell edit and a typed statement are all guarded by one seam and there is no second one to go
    round. A statement is decided by the engine's own `driver.Text.Review`, so the guard and the
    editor's underline cannot disagree, and **a refusal defaults to refusing where a confirmation
    defaults to allowing** — which is why `WITH` is refused and not merely flagged. It is a guard on
    this app and the docs say so: nothing reaches the server, so a read-only *account* is what
    protects the database. The frontend never re-derives it — a session says `readOnly`, a menu item
    says `writes`, and `withoutWrites` is the one place the two meet.

27. **A job on a clock fires the bound method the button fires, and there is no second one.**
    `internal/schedule` is a task, a cadence and what the last few firings came to — it runs
    **nothing**, and knows how to take a backup exactly as much as `internal/history` knows how to run
    a statement. What fires is `RunBackup`, `RunExport`, `RunToolDump` or the two `Compare` reads, so
    a backup taken at two in the morning is byte for byte the one `New Backup` would have written; a
    second speller here would be rule 13's failure with nobody watching. Only **four** kinds exist and
    every one of them *reads*: a restore and an import are not scheduled, because a job that ran while
    nobody was looking is the wrong place to find out a table was overwritten — which is also why a
    read-only connection carries all four. The clock runs **only while the app is open** and the docs
    say so; a moment that went by while it was shut is the reader's answer (`CatchUp`) and never a
    default, decided once when the scheduler starts so that opening a laptop twice cannot fire a job
    twice. A firing is an **outcome and not an error** — there is nobody in front of a dialog to throw
    one to — and `differs` is the third of them, because a comparison that found two sides apart did
    what it was asked. A job dials **its own** session and never borrows the reader's, a firing inside
    somebody's open transaction being exactly the accident rule 20 makes possible.

28. **What a server already ran *slowly* is read from six different catalogues, and one speller turns
    the recording on.** `driver.Profiler` is the fifth half of a session and rule 22's arrangement
    applied to the half hour between what is in flight and what has been counted since startup:
    `mysql.slow_log`, `pg_stat_statements`, `sys.dm_exec_query_stats`, `v$sqlarea`, `system.profile`
    and `SLOWLOG GET` are each that engine's own read in that engine's own columns — one row is a
    *run* on two of them and a *statement* on the rest — while `sqlkit.Profile` spells and runs
    **every** change there is. What differs between five SQL engines is a flag on `Dialect`
    (`ProfileOnSQL`, `ProfileOffSQL`, `ProfileThresholdSQL`, `ProfileResetSQL`, and the one threshold
    kept in seconds), never a second speller; `ProfileStatements` previews exactly what
    `ApplyProfileChanges` runs — rule 13 again — and a threshold is checked as a number first, for
    the reason a process id is. **What a reader takes away is spelled by the driver too**: a slow log
    records the statement and the database it ran in *apart*, so `SlowQueryRef.Runnable` puts them
    back together as something that will run elsewhere — and entering a database is a flag
    (`EnterDatabaseSQL`) because four engines spell it four ways and PostgreSQL has no statement for
    it at all. `Complete` says it would run as it stands, which two engines cannot promise, recording
    only what they normalized; that is a *driver's* fact and never a hunt for a `?` in the text, and
    it is also why MySQL is read from its slow log rather than from a digest no server would take
    back. There is deliberately no Explain button — Explain in this app runs the statement, and a row
    of a slow log is as likely to be a `DELETE`, so a press there would be one press from running
    something somebody only wanted to understand. What is dangerous is a `domain` rule
    (`domain.ProfileHazard`, the seventh grammar): turning it on or off is a write, throwing the
    recording away is destructive, and neither is ever unfiltered because there is nothing here to
    aim. An engine that records nothing is `domain.Unsupported` naming it and
    `EngineInfo.hasProfiler` says so to the frontend; whether it can be switched at all is
    `profilerConfigurable`, and whether the recording belongs to a database rather than to the server
    is `profilerPerDatabase`.

29. **An object store is a second port beside `internal/driver`, and its bytes never cross the
    bridge.** A bucket has no catalogue, no namespace, no statement and nothing that is a row, so
    `internal/objectstore` sits *beside* the driver rather than inside it — and what the two share is
    only what is not about a database: `domain`, `tlscert`, `neterr`, `sshtunnel`, `netproxy`. The
    port is the **protocol alone** — sign a request, list a page, read a range, write a part — and
    walking a prefix, searching it, downloading a folder in parallel and resuming what was interrupted
    are each written **once above it**, which is `sqlkit.DDL`'s arrangement applied to a service.
    There is **one adapter** and one is the honest number: seven services speak the same protocol, so
    what differs is a **preset** (`domain.StoreProvider`) deciding an endpoint, whether the bucket is
    named in the host or the path, and what a region means to a service that has none. **No SDK is
    taken** — the signature is `internal/sigv4` and the answers are six XML documents — for the reason
    `internal/transfer` takes none for a spreadsheet.
    **An object's bytes go from the socket to the file and back, a fixed buffer at a time**, so what
    crosses the bridge is a *number*: a preview is therefore a **byte range** and never the object, a
    folder download is a **walk streamed into the queue** and never a listing held whole, and a copy
    is one request naming both keys and so not a transfer at all. What a bucket makes expensive is
    reading it, so **the service does every narrowing it can**: browsing is a folder at a time with a
    delimiter, a search hands its literal front over (`PruneTo`), and **there is no index** — one would
    be a copy of somebody's data that answers confidently about an object that has gone. A prefix is
    consequently **not a node of the tree** but a row of the grid, for the reason a key space's colons
    are. What is dangerous is a `domain` rule (`domain.ObjectHazard`, the eighth grammar) and the three
    actions that name **nothing** — a folder delete, an empty, a bucket drop — are `unfiltered`; the
    whole matrix is answered once per session and the frontend never re-derives it. A read-only store
    is one wrapper (`session.readOnlyStore`) and needs no reviewer, an object store having no statement
    to read. No secret is in `object-stores.json`.
    **What a service keeps about one object is seven panes and each is read only when it is opened** —
    clicking a row must cost one request and not four. Two of the seven are not what they look like and
    the app says so rather than pretending: there is **no request that sets a header**, so changing one
    is a copy of the object onto itself with `REPLACE` — six headers are settable, the rest are the
    service's and are drawn locked, the set is replaced entire so it is staged and applied in one
    press, and the object is *rewritten* (a new tag, a new version). And a `Log` pane is **this app's
    own record**, not the service's access log: that is a bucket setting writing objects elsewhere, and
    a client showing one would be showing somebody else's gigabytes.

## Where to read before changing something

| Task | Read |
|---|---|
| Build, test, run against a real database | [docs/development.md](docs/development.md) |
| Touch a layer boundary, or add an engine | [docs/architecture.md](docs/architecture.md) |
| Find out what an engine has, or has not | [docs/features/engines.md](docs/features/engines.md) |
| Touch a SQL adapter, or a `Dialect` flag | [docs/architecture.md](docs/architecture.md#the-five-sql-engines) |
| Touch anything MongoDB | [docs/architecture.md](docs/architecture.md#the-document-store) |
| Touch anything Redis | [docs/architecture.md](docs/architecture.md#the-key-value-store), [docs/features/key-space.md](docs/features/key-space.md) |
| Touch an object store — a bucket, a prefix, the S3 adapter, a signature | [docs/architecture.md](docs/architecture.md#buckets-full-of-objects), [docs/features/object-storage.md](docs/features/object-storage.md) |
| Change what is shown about one object — a pane, a header, an ACL, a version | [docs/features/object-panel.md](docs/features/object-panel.md) |
| Change how an object is found in a bucket | [docs/features/object-search.md](docs/features/object-search.md) |
| Change a download, an upload or the transfer queue | [docs/features/object-download.md](docs/features/object-download.md) |
| Add or change a failure mode | [docs/errors.md](docs/errors.md) |
| Find out what exists at all | [docs/features.md](docs/features.md) — the index, one file per feature under [docs/features/](docs/features/) |
| Change how a connection is made or guarded — a proxy, a bastion, TLS | [docs/features/connections.md](docs/features/connections.md) |
| Turn a connection read-only, or change what a read-only one refuses | [docs/features/read-only.md](docs/features/read-only.md) |
| Change the tree, the table list or a schema change | [docs/features/schema-tree.md](docs/features/schema-tree.md), [docs/features/schema-changes.md](docs/features/schema-changes.md) |
| Change a view's, a routine's or a trigger's definition — or a materialized view, a sequence, an event | [docs/features/definitions.md](docs/features/definitions.md) |
| Touch a partition of a table | [docs/features/partitions.md](docs/features/partitions.md) |
| Run a routine, or touch what one takes | [docs/features/routine-run.md](docs/features/routine-run.md) |
| Change a query tab, a console or a file run | [docs/features/query-tab.md](docs/features/query-tab.md), [docs/features/script.md](docs/features/script.md), [docs/features/console.md](docs/features/console.md) |
| Turn autocommit off, or touch Begin / Commit / Rollback | [docs/features/transaction.md](docs/features/transaction.md) |
| Touch a user, a role or a privilege | [docs/architecture.md](docs/architecture.md#who-may-do-what), [docs/features/accounts.md](docs/features/accounts.md) |
| Touch a process list, a kill, a server status or a variable | [docs/architecture.md](docs/architecture.md#what-a-server-is-doing), [docs/features/monitor.md](docs/features/monitor.md) |
| Touch a live chart, or what a server is asked to count for one | [docs/features/metrics.md](docs/features/metrics.md) |
| Touch a slow query list, or what a server records about one | [docs/architecture.md](docs/architecture.md#what-a-server-already-ran-slowly), [docs/features/profiler.md](docs/features/profiler.md) |
| Change what is kept about a statement that has run | [docs/features/log.md](docs/features/log.md), [docs/features/results.md](docs/features/results.md#the-history-pane) |
| Touch a named snippet, the library file, or what completion offers by name | [docs/features/snippets.md](docs/features/snippets.md), [docs/editor.md](docs/editor.md#what-completion-offers) |
| Change a table tab, its paging, filtering or values | [docs/features/table-tab.md](docs/features/table-tab.md), [docs/features/row-limit.md](docs/features/row-limit.md), [docs/features/filter-sort.md](docs/features/filter-sort.md), [docs/features/value-viewer.md](docs/features/value-viewer.md) |
| Touch a file format, an import or an export | [docs/architecture.md](docs/architecture.md#carrying-a-table-to-a-file), [docs/features/transfer.md](docs/features/transfer.md) |
| Touch a backup or a restore | [docs/architecture.md](docs/architecture.md#backing-a-database-up), [docs/features/backup.md](docs/features/backup.md) |
| Touch mongodump, mysqldump, pg_dump — or anything this app *runs* | [docs/architecture.md](docs/architecture.md#running-the-engines-own-programs), [docs/features/tools.md](docs/features/tools.md) |
| Put a job on a clock, or change what one fires | [docs/architecture.md](docs/architecture.md#running-a-job-on-a-clock), [docs/features/automation.md](docs/features/automation.md) |
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

**An object store has no `EngineInfo`, and that is the point**: what its connection may do arrives on
the *session* as `ObjectRules` — one entry per action, each saying what it is called, how dangerous it
is, whether it is confirmed and whether it is refused — because those are facts about the **profile**
and not about the provider. What a *provider* decides is three fields of a form
(`bridge/objects.ts`'s `PROVIDERS`), which is per-provider data in the sense
`features/schema/dataTypes.ts` is per-engine data, and never a comparison against a provider's name
in a component.

A new engine capability the UI has to know about is a field on `EngineInfo` in
`frontend/src/bridge/connections.ts` — `hasForeignKeys`, `hasRoutines`, `hasTriggers`, `documents`,
`keys`, `isFile`, `createsTables`, `createsDatabases`, `hasViews`, `hasSource`, `hasBackups`, `hasTransactions`,
`hasAccounts`, `accountLabel`, `hasRoles`, `hasMonitor`, `cancelsStatement`, `closesConnection`,
`processLabel`, `hasProfiler`, `profilerConfigurable`, `profilerPerDatabase`,
`hasMaterializedViews`, `hasSequences`, `hasEvents`, `hasPartitions`,
`hasColumnComments`,
`dumpTool`, `restoreTool`, `needsUser`, `needsDatabase`, `databaseLabel` — and never a comparison
against an engine's name in a component. What a *connection* is rather than what its engine is goes
on the **session** instead (`readOnly`), for the reason `confirm` travels on a statement.
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
