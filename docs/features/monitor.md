# Server monitoring

[← Features and UI behaviour](../features.md)

## What it is

The `Server Monitor` row under a connection opens one tab with four panes: what the server is doing
**right now**, what it has already run [**slowly**](profiler.md), what it has been counting since it
started, and what it was started **with**.

This document is the three that are about the server as a whole. The Slow Queries pane is its own —
it reads a different catalogue on every engine, it can be turned on and off, and it hands a statement
to a query tab — and it is [features/profiler.md](profiler.md).

**It hangs off the connection and not off a database**, which is the only honest place for it: a
process belongs to the server, and the tree's other levels are all namespaces. It is the second row
in the tree that does, the [`Users` folder](accounts.md) being the first — and unlike that one it
holds **nothing**, because what a server is doing changes while nobody is touching it and a list of
branches nobody could keep up with is worse than no list at all.

Every pane is a [`ResultGrid`](../grid.md) in whatever columns that engine's own catalogue keeps,
exactly as [the table list](schema-tree.md) and [the account list](accounts.md) are, and for the same
reason: no two engines report the same facts about a connection, and a column neither could fill would
say nothing.

| Pane | Where it comes from |
|---|---|
| Processes | `information_schema.processlist`, `pg_stat_activity`, `dm_exec_sessions` with `dm_exec_requests`, `v$session`, `currentOp`, `CLIENT LIST` |
| Status | `SHOW GLOBAL STATUS`, `pg_stat_database`, `dm_os_performance_counters`, `v$sysstat`, `serverStatus`, `INFO all` |
| Variables | `SHOW GLOBAL VARIABLES`, `pg_settings`, `sys.configurations`, `v$parameter`, `getParameter`, `CONFIG GET *` |

**What a process is called is the engine's word.** MySQL has processes, PostgreSQL **backends**,
SQL Server and Oracle **sessions**, MongoDB **operations**, Redis **clients**. That is
`EngineInfo.processLabel`, and no component compares against an engine's name to decide it.

**Reading any of it is privileged on every one of these engines**, and each answers a reader without
the privilege in its own way: MySQL shows them their own threads and no error, PostgreSQL blanks
somebody else's statement text, and the rest refuse in the server's own words. None of it is a broken
pane — the failure stands under the rows already drawn, exactly as [a table view's does](table-tab.md).

**SQLite has none of this.** A SQLite database is a file: nobody is connected to it, nothing is in
flight, and a pragma is a fact about this program's own handle rather than a server's state under
another name. There is no `Server Monitor` row under a SQLite connection and nothing to open — see
[`hasMonitor`](engines.md#what-each-engine-can-be-asked-to-do).

## Reading it

Which pane is being read is the leftmost control on the toolbar, and it is the same
[`Segmented`](../frontend.md) a comparison switches its two halves with: the pane you are on wears the
accent, rather than being one of four buttons with a shade on it.

**Changing what is being asked reads it at once**, and the rows already drawn go while it does — a
spinner, and then the answer. Switching pane is one such change, and so is switching the database on
the [Slow Queries](profiler.md) pane, which is the one thing here that belongs to a database rather
than to the server. Nothing has to be pressed afterwards.

That is the *question* changing. A **re-read of the same question** is the opposite case and keeps its
rows where they are, which is what makes a hiccuping server watchable — see the failed re-read below.
`useServerReport` is handed the question rather than the read itself, because the read is a closure
over the session and the pane and would otherwise be a new function on every render.

The pane also **re-reads itself on a clock** — off, or every 2, 5, 10 or 30 seconds, and five when it
opens. That interval belongs to the tab rather than to the view, so switching away and back does not
silently stop a monitor the reader left running.

**An answer that arrives late is dropped.** A reader moving through three databases faster than a busy
server answers has three reads in flight, and the slowest would otherwise land last and win — leaving
the pane showing a database nobody is looking at any more.

> **The clock stops while a row is picked.** A list that moved under a pick would leave the reader's
> selection pointing at somebody else's process, and the next press would end *that* one. It is the
> one mistake a monitor must not make, so the toolbar says why it has gone still rather than being
> clever about it.

**Value** puts a read-only [`CellEditor`](../grid.md#how-it-behaves) under the grid, on whichever cell
is focused. Every pane here holds cells far longer than a column is wide — a statement in flight, a
`currentOp` document, a `pg_settings` description — and reading one otherwise meant copying it out of
the app. It is read-only on all four panes: what a server reported is not something there is anywhere
to write back to.

**The toolbar scrolls sideways** rather than letting its controls run off the edge. This tab carries
more of them than any other surface with one, and every control keeps its own width — nothing is
squeezed to fit.

The box on the right narrows the rows to the ones holding the text in **any** cell. It narrows what is
already in memory rather than asking again, for the reason [the key space's prefix
tree](key-space.md) does: `pg_settings` has four hundred rows and
`dm_os_performance_counters` some thousands, neither is a paged read, and a monitor that asked the
server again per keystroke would be asking a busy server the very question the reader is watching it
answer. **The references travel with the rows**, so a narrowed list is acted on by the right ones.

A re-read that fails leaves the rows already drawn where they are and puts the reason beside them: a
monitor that blanked itself the moment a server hiccoughed would be one nobody could watch.

## Ending one

Two things can be ended, and they are not the same thing:

- **Cancel Statement** stops what the process is running and leaves the connection open. What the
  client sees is a failed query.
- **End \<process\>** ends the connection itself, and with it whatever transaction was open on it.

### What each engine can end

| Engine | Cancel a statement | End a connection |
|---|---|---|
| MySQL | `KILL QUERY` | `KILL CONNECTION` |
| PostgreSQL | `pg_cancel_backend` | `pg_terminate_backend` |
| SQL Server | | `KILL` |
| Oracle | | `ALTER SYSTEM KILL SESSION 'sid,serial' IMMEDIATE` |
| MongoDB | `killOp` | |
| Redis | | `CLIENT KILL ID` |

A blank is a thing the engine has not got, and asking for it answers
[`UNSUPPORTED`](../errors.md) naming which — never a statement left for the server to refuse. The
button is not drawn for it either, so the refusal is a backstop rather than the reader's first hint.

Four of those blanks are worth the sentence:

- **SQL Server has `KILL` and nothing narrower.** `KILL <spid> WITH STATUSONLY` only reports how far a
  rollback has already got, so there is nothing that stops one statement and leaves its session.
- **Oracle's cancel arrived in 18c** and is addressed by the *statement's* id rather than by the
  session. An app that guessed at it would either aim at the wrong thing or be refused by every older
  server, so this one does not claim it: what it offers is ending the session.
- **MongoDB is the other way round** — the one engine with the cancel and not the close. `killOp`
  stops an operation; a MongoDB client is disconnected by the client, and there is no server command
  that closes somebody else's.
- **A Redis server runs one command at a time and to completion**, so there is nothing in flight for a
  second connection to cancel. The two exceptions have commands of their own — `SCRIPT KILL` and
  `FUNCTION KILL` — and a running script is not a row of `CLIENT LIST`: both are typed into a tab.

**An Oracle session is the pair `sid,serial#`**, which is why a process carries a serial at all: a sid
is handed to the next session the moment this one ends, so a statement naming one alone would end
whoever came after. That is `Dialect.ProcessIsPair`, and it is the only process name in the app that
is a string literal rather than a bare number.

### The statement is shown, and the id is checked

[The statement a preview shows is the statement it runs](schema-changes.md). There is no Changes pane
here, so the confirmation itself carries the statements — `PreviewProcessKills` spells exactly what
`KillProcesses` runs, through one speller on `sqlkit.Dialect`.

**A process id is checked as a number before it reaches a statement.** A kill binds no arguments, so
the id travels as text, and this is the one place in the whole port where a value off the wire would
otherwise be pasted into SQL. A reference that is not numbers spells **nothing** and is refused by the
run — see `sqlkit.Watch.supports`, and `sqlkit/monitor_test.go`, which is about the quoting rather
than about tidiness.

### What has to be answered for

Three things, and the middle one is why this exists at all:

| What | Class | Confirmed |
|---|---|---|
| Ending **this app's own** connection | unfiltered | **everywhere**, laptop included |
| Ending anybody else's connection | destructive | on a [guarded environment](connections.md#what-gets-confirmed-before-it-runs) |
| Cancelling a statement | write | on a guarded environment |

Ending the connection this app is reading on is unfiltered in exactly the sense `DELETE` with no
`WHERE` is: it is not a statement that does too much, it is one aimed at the wrong thing entirely, and
it takes the reader's own session down with the transaction they had open on it. That is
`domain.ProcessHazard`, the sixth grammar for one rule beside the ones [a statement](script.md), [an
import](transfer.md), [a grant](accounts.md), a shell command and a Redis command are read by — and
the frontend **receives its answer** rather than deriving one.

**A batch is answered for by its worst row.** Picking three connections and pressing End runs all
three, so one of them being this app's own is what the question is about.

### Which row is ours

The driver marks it, and the reader is told in the confirmation. It is read from a second scalar
question asked beside the list — `CONNECTION_ID()`, `pg_backend_pid()`, `@@SPID`,
`SYS_CONTEXT('USERENV','SID')`, `CLIENT ID`, and on MongoDB `whatsmyuri` against the operation's own
client.

**It says "one of ours" rather than "the only one".** A session holds a small pool — four connections,
and one per database on PostgreSQL and per database index on Redis — and which of them answered a
given read is `database/sql`'s business rather than anything this app chooses. Marking one is worth it
anyway, because the mistake it prevents is the one nothing else could catch.

## What is not here

**A statement that has already finished is not in any of these three.** A process list holds what is
in flight, and a status holds a number counted since the server started; the statement that was slow
ten minutes ago is in neither. That is [the Slow Queries pane](profiler.md), which is why it exists.

**A variable cannot be changed from this pane.** Reading one is the same question on every engine and
setting one is not: `SET GLOBAL` takes effect at once, `ALTER SYSTEM SET` needs a reload and rewrites
a file the server owns, `sp_configure` is followed by `RECONFIGURE` and takes a subset of the list,
Oracle's wants a scope, and `CONFIG SET` outlives the connection that made it and is
[classified destructive for exactly that reason](script.md). A grid of editable cells over five
grammars, two of which do not take effect until a restart, would be a grid that lies about what
pressing Enter did. A variable is changed in a [query tab](query-tab.md) or a
[console](console.md), where the reader can see which server they are on and what the server said back.

**An idle MongoDB connection is not listed.** `currentOp` answers with operations, and an idle
connection is not one — which is the honest reading of "what is this server doing" on that engine
rather than a gap in this pane.

**Nothing here is kept.** Nothing is cached in a store, and the pane holds one read at a time — a
count in the sidebar or a row remembered from a minute ago would be a number about a server that has
moved on. A kill is not in the [History Log](log.md) either, that log holding what **Execute** sent and
nothing the app did of its own accord — the same reason a schema change from the tree is not in it. The
statement is shown wherever the reader is asked to answer for it, which is what the table above says
and why the confirmation carries it in full.
