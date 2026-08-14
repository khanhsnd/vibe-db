# Slow queries

[← Features and UI behaviour](../features.md)

## What it is

The `Slow Queries` pane of the [`Server Monitor`](monitor.md) tab is what a server has **already run
slowly**. It sits between the two questions the other panes answer — what is running right now, and
what has been counted since the server started — and it is the one a reader opens when something was
slow half an hour ago and is not slow any more.

It replaces the thing that was otherwise typed by hand on every engine: `SET GLOBAL
slow_query_log = ON`, `db.setProfilingLevel(1, 100)`, a `SELECT` over `pg_stat_statements`,
`SLOWLOG GET`. Each of those is a different grammar, and two of them have to be turned off again
afterwards.

Like every other pane of that tab it is a [`ResultGrid`](../grid.md) in whatever columns that
engine's own catalogue keeps, and for the same reason: no two engines record the same facts about a
statement, and a column neither could fill would say nothing.

## Where each engine keeps it

| Engine | Read from | What a row is |
|---|---|---|
| MySQL | `mysql.slow_log` | one **run** of a statement, newest first |
| PostgreSQL | `pg_stat_statements` | one **statement**, with every run of it counted, costliest in total first |
| SQL Server | `sys.dm_exec_query_stats` with `sys.dm_exec_sql_text` | one **plan** in the cache, costliest in total first |
| Oracle | `v$sqlarea` | one **cursor** in the shared pool, costliest in total first |
| MongoDB | `system.profile` | one **operation**, slowest first |
| Redis | `SLOWLOG GET` | one **command**, newest first |

**SQLite has none of this.** A SQLite database is a file: a statement runs inside this program's own
process and is over, and there is nothing that would have written it down. There is no `Slow Queries`
pane under a SQLite connection — see [`hasProfiler`](engines.md#what-each-engine-can-be-asked-to-do).
What that engine has instead is `EXPLAIN QUERY PLAN` on a statement the reader types, which is the
Explain button on a [query tab](query-tab.md).

**Two of them record per database rather than per server.** A MongoDB profile is a collection of one
database and a PostgreSQL extension is installed into one, so those two draw a database selector on
the toolbar and everything else does not — a control that changed nothing would be worse than none.
Picking one [reads it at once](monitor.md#reading-it), rather than leaving the reader to press
Refresh at a pane still showing the database they just left.

`pg_stat_statements` is the exception worth naming: the **view** is the cluster's and holds a row per
statement per database, so the read narrows it to the one being looked at. A pane that did not would
show a reader statements from databases they are not connected to.

**MySQL is read from its slow log and not from `performance_schema`.** The digest summary there
would answer "which statement costs this server the most" in one read, but it has already replaced
every value with a `?` — so a statement copied out of it is not one any server will take. The slow
log keeps the statement as it ran. That choice is why turning the recording on sets **two**
settings: MySQL writes its slow log to a file until `log_output` says `TABLE`, and a file on the
server's own disk is not something this app can open.

## Reading it

The toolbar is [the monitor's](monitor.md#reading-it) — the same `Segmented`, the same clock, and the
same box, conditions and sort, all three of which
[narrow what is already in memory](monitor.md#narrowing-and-ordering) rather than asking the server
again. **Profiling…** is this pane's own, and is drawn only where the engine has a switch.

Ordering is worth a sentence here, because this is the pane a reader sorts most. What the driver reads
is already ordered by what matters most on that engine — total time on four of them, and *newest
first* on the two that record one row per run — so a sort here is the reader asking a second question
of an answer that already had a sensible one.

There is deliberately no Explain button here, and the reason is worth writing down because it will be
suggested again. In this app **Explain runs the statement** and asks about it as well — [see what
Explain shows](query-tab.md#what-explain-shows) — and a row of a slow log is as likely to be a
`DELETE` as a `SELECT`. A button on this pane would be one press away from running a statement
somebody only wanted to understand.

### Taking a statement away

Every row carries two buttons in [a column pinned to the right edge](../grid.md#the-pinned-action-column).
It is the one pane in the app with such a column, and it has one because its cells are the widest
there are: a statement runs off the side, and a button that scrolled away with it would be a button
nobody could reach.

**Copy** puts the statement on the clipboard as something that will **run somewhere else** — another
client, a script, a colleague's terminal. It is not the cell's text. The cell holds what the server
wrote down, which on most of these engines is a fragment: the statement and the database it ran in
are recorded apart, and an unqualified name in it resolves nowhere until one is entered. What goes
around it is the engine's own knowledge, so the **driver** spells it, exactly as it spells every
other statement in this app.

| Engine | What Copy puts in front of the statement |
|---|---|
| MySQL | ``USE `shop`;`` |
| SQL Server | `USE [shop];` and `GO`, its batch separator — without which the statement runs in the database the reader left |
| Oracle | `ALTER SESSION SET CURRENT_SCHEMA = "shop";`, this engine having no database below its service |
| PostgreSQL | a comment naming the database, there being no statement that enters one — a connection here is bound to its database for life |
| MongoDB | `db.getSiblingDB("shop").runCommand({ … })`, since a shell's `db` is whichever database the reader is pointed at |
| Redis | nothing — a Redis database is a number, and the recorded command runs as it stands |

**Where the engine records only what it normalized, the copied text says so** in two comment lines
before the statement. PostgreSQL writes `$1` where the value was and Oracle keeps its bind variables,
because that is how each groups one statement's thousand runs into a single row — and a reader who
pastes `$1` into a terminal and is refused has been told nothing by the refusal.

A row whose statement the server did not record copies **nothing**, and its button says so rather
than handing over a `USE` with a blank line after it.

**Test** is a place kept and nothing more. It is drawn disabled and says so, rather than being left
out and moving the Copy button sideways the day it arrives.

### Reading a long value without leaving the app

**Value** puts [the `CellEditor` panel](../grid.md#how-it-behaves) under the grid, on whichever cell
is focused, and it is drawn read-only: a slow list, a process list and a server's own counters are
all reads, and a panel that took an edit it would then throw away is worse than one that never
opened.

It is the answer to the shape of these panes rather than a convenience. Every one of them holds cells
far longer than a column is wide — a statement, a `currentOp` document, a `CLIENT LIST` line — and
reading one otherwise meant copying it out of the app. The toggle is on this surface's own toolbar
rather than in the grid's head strip, because one table must never carry two strips.

### An empty pane is two different answers

A list with nothing in it means **nothing has run slowly**, which is the happy answer. A server that
is not recording at all is a different thing entirely, and it is said by the dialog rather than by
the grid, because only a read of the *setting* can tell them apart:

- A PostgreSQL server without the extension installed says so, and whether it is one statement away
  or needs `shared_preload_libraries` and a restart — which this app cannot do and does not pretend
  to.
- A MySQL server writing its slow log to a file says so, and that turning the recording on here will
  move it to a table.

## Changing what is recorded

`Profiling…` reads the server **every time it opens**, never what this app set last: another client
may have moved either setting, and a switch drawn from memory would be a switch that lies about the
server in front of the reader.

| Engine | On and off | Threshold | Clear |
|---|---|---|---|
| MySQL | `slow_query_log`, with `log_output` | `long_query_time` | `TRUNCATE TABLE mysql.slow_log` |
| PostgreSQL | `CREATE` / `DROP EXTENSION pg_stat_statements` | | `pg_stat_statements_reset()` |
| SQL Server | | | `DBCC FREEPROCCACHE` |
| Oracle | | | |
| MongoDB | `setProfilingLevel` | `slowms` | drop `system.profile` |
| Redis | `slowlog-log-slower-than` | the same setting | `SLOWLOG RESET` |

A blank is a thing the engine has not got, and asking for it answers
[`UNSUPPORTED`](../errors.md) naming which — never a statement left for the server to refuse. The
control is not drawn for it either, so the refusal is a backstop rather than the reader's first hint.

Four of those blanks are worth the sentence:

- **SQL Server and Oracle record whether or not anybody asked.** A plan cache and a shared pool hold
  a statistic per statement as a matter of course, so there is nothing to turn on. That is
  `ProfileConfig.Configurable` false, and it draws no dialog rather than one whose every control is
  dead.
- **PostgreSQL has no threshold.** `pg_stat_statements` counts every statement and the reader picks
  the slow end out of the list. The setting that *does* take one, `log_min_duration_statement`,
  writes to a file on the server's disk that this app cannot read — so a box for it would be a number
  that changed nothing on this pane.
- **Oracle has no reset.** The only thing that would empty `v$sqlarea` is a flush of the whole
  instance's parsed statements, after which every session on it reparses from cold. That is a thing
  to do to a server deliberately, not from a button beside a list.
- **Redis's switch and threshold are one setting.** `-1` records nothing and any other number is how
  slow is slow enough, which is why turning it off is not the same as setting it to zero — zero on
  that engine records *everything*.

**The reader is asked in milliseconds on every engine.** MySQL keeps `long_query_time` in seconds and
Redis keeps its threshold in microseconds; both are converted where the statement is spelled, so two
servers can be read side by side without the reader converting in their head. That is
`Dialect.ProfileThresholdInSeconds`, a flag and never a second speller.

### The statement is shown, and the number is checked

[The statement a preview shows is the statement it runs](schema-changes.md). There is no Changes pane
here, so the confirmation itself carries the statements — `PreviewProfileChanges` spells exactly what
`ApplyProfileChanges` runs, through one speller on `sqlkit.Profile` for the five SQL engines.

**A threshold is checked as a number before it reaches a statement.** A `SET GLOBAL` binds no
arguments, so the number travels as text — the same reason [a process id is
checked](monitor.md#the-statement-is-shown-and-the-id-is-checked) — and it is checked on both sides
of the wire.

**Clearing MongoDB's is three commands, not one.** `system.profile` is capped, so its documents
cannot be deleted one at a time and the collection is dropped instead; a server will not drop the
collection it is writing to, so the recording is turned off first and put back exactly where it was
afterwards. A reset that quietly left a reader's profiling off would have cost them the next hour.

### What has to be answered for

| What | Class | Confirmed |
|---|---|---|
| Turning the recording on or off, or moving the threshold | write | on a [guarded environment](connections.md#what-gets-confirmed-before-it-runs) |
| Throwing away what was recorded | destructive | on a guarded environment |

That is `domain.ProfileHazard`, the seventh grammar for one rule beside the ones [a
statement](script.md), [an import](transfer.md), [a grant](accounts.md), a shell command, a Redis
command and [a kill](monitor.md#what-has-to-be-answered-for) are read by — and the frontend
**receives its answer** rather than deriving one.

Neither is ever *unfiltered*. That class is for a statement aimed at the wrong thing entirely, and
there is nothing here to aim: a profiler is one switch and one number on the server itself.

**A batch is answered for by its worst change**, which is the arrangement a batch of kills has.

## On a read-only connection

The pane reads, the dialog opens, and Save is refused with
[`READ_ONLY`](read-only.md) — nothing reaches the server. Turning a recording on is a change to the
server that outlives the connection making it, which is the promise a read-only connection is: what
is held back is the change, never the answer.

## What is not here

**Nothing is kept.** The pane holds one read at a time and caches nothing in a store, for the reason
[the monitor's panes](monitor.md#what-is-not-here) do: a count remembered from a minute ago is a
number about a server that has moved on. A change made here is not in the [History Log](log.md)
either, that log holding what **Execute** sent and nothing the app did of its own accord.

**There is no paging.** Every engine here keeps a bounded record of its own — a capped collection, a
ring, a plan cache — and the pane reads the top of it. A reader hunting a slow statement is looking
at the worst of them, not paging to the bottom of a list.

**A statement is never rewritten.** What the pane shows and what the query tab opens with is the text
the server recorded, whatever shape it is in — the same rule [a definition](definitions.md) travels
back by.
