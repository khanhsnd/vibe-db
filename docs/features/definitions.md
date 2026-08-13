# Definitions

[← Features and UI behaviour](../features.md)

## What a definition is

A table is a *shape*, and [the designer](schema-changes.md) draws it as three grids of fields,
indexes and keys. A view, a routine and a trigger are not: a view is a query and the other two are
programs, and none of them is a list of anything. So they are edited as what they are — the
**statement that declares them** — in a tab of their own. A **materialized view**, a **sequence**
and an **event** join them, for exactly the same reason and through exactly the same machinery.

**The buffer is the whole `CREATE`**, exactly as the server answered with it. Four of the five SQL
engines will say what they hold:

| Engine | A view's definition comes from | A routine's comes from | A trigger's comes from |
|---|---|---|---|
| MySQL | `SHOW CREATE VIEW` | `SHOW CREATE PROCEDURE` / `FUNCTION` | `SHOW CREATE TRIGGER` |
| PostgreSQL | `pg_get_viewdef`, under a `CREATE OR REPLACE` spelled by the driver | `pg_get_functiondef`, whole | `pg_get_triggerdef`, whole |
| SQLite | `sqlite_master.sql` — the statement the file stores | — | `sqlite_master.sql`, the same read |
| SQL Server | `sys.sql_modules.definition` | the same | the same |
| Oracle | `all_views.text`, under a `CREATE OR REPLACE` spelled by the driver | `all_source`, under one | `all_source`, under one |

The three kinds only some engines have are read the same way where the engine answers, and
*described* where it keeps numbers rather than a statement:

| Kind | Where it exists | Its definition comes from |
|---|---|---|
| **Materialized view** | PostgreSQL, Oracle | `pg_get_viewdef` and `all_mviews.query` — the query alone, under a `CREATE MATERIALIZED VIEW` spelled by the driver |
| **Sequence** | PostgreSQL, Oracle, SQL Server | `pg_sequences`, `all_sequences`, `sys.sequences` — **numbers, not a statement**, so the whole `CREATE SEQUENCE` is put together out of them |
| **Event** | MySQL | `SHOW CREATE EVENT`, whole — schedule, definer and body |

A sequence's `START WITH` is **where it has got to** rather than where it began, because a save
drops it and makes it again: a sequence restarted at its original number would hand out values rows
already hold. That is the one place a description here is deliberately not the declaration that was
originally run, and it is the honest one.

Neither a materialized view nor a sequence is spelled `OR REPLACE` — no engine has that form for
either — so a save of one is the drop and the create the Changes pane shows, and **a drop takes what
was stored with it**: a materialized view's rows, a sequence's current value.

It is a **read** and not a render wherever it can be, for the reason a dump's `CREATE TABLE` is on
the engines that answer for themselves: a declaration put back together out of a catalogue is only
ever as complete as the catalogue's columns. Where an engine keeps the body alone — PostgreSQL and
Oracle both answer for a view with its query and nothing else — the `CREATE` in front of it is the
**driver's** to spell, like every other statement.

**MySQL's answer names no database**, so the driver qualifies it: a routine put back as it came
would land in whichever database the pool happened to be on rather than the one it was read from.
Oracle's is qualified for the same reason. Nothing else about the text is touched — a `DEFINER`
clause, an `ALGORITHM`, a `sql_mode`-dependent body all reach the reader as the server spelled them.

**A definition is run *inside* its namespace rather than named into one.** Every other statement this
app writes addresses its object by a qualified name; a definition cannot, because its body is the
reader's own text and an unqualified table inside a view, a routine or a trigger resolves against
whichever namespace the connection is in. So a Save and a Delete pin a connection first — MySQL's and
SQL Server's `USE`, PostgreSQL's `search_path`, Oracle's `CURRENT_SCHEMA` — which is `sqlkit.DDL.Pin`,
and the only place in a SQL adapter a statement is not self-addressing. Two engines refuse the
statement outright without it: MySQL will not have a trigger and its table in different schemas, and
SQL Server's `CREATE TRIGGER` has to be the first statement of a batch in the database it belongs to.

**There is no `DELIMITER` anywhere, and none is needed.** A definition is *one* statement however
many semicolons a routine or trigger body holds, and it is run as one — nothing on either side of the
wire re-lexes what the reader typed. That is the same rule the rest of the app follows from the other
end: [reading a script is the driver's](script.md), and this never asks.

**Two engines have no definition to read.** A MongoDB view is a collection declared by a *document*
— the collection it reads and the pipeline over it — rather than by a statement, a change stream is
something a *client* watches rather than a trigger the server keeps, and a Redis key space is not
declared at all. Both answer [`UNSUPPORTED`](../errors.md) naming which, and the tab is not offered
on either: `EngineInfo.hasSource` is what says so, and no component compares against an engine's
name. See [engines.md](engines.md#what-each-engine-can-be-asked-to-do).

## What names one

A name is not always enough, and the two fields beside it are each one engine's way of saying which:

- **A routine carries its arguments.** On PostgreSQL two functions may share a name and be told apart
  only by what they take, so a drop, a definition read and a tab's identity all carry them.
- **A trigger carries its table.** On PostgreSQL a trigger name is unique per *table* rather than per
  schema, so `DROP TRIGGER n ON t` is the only drop that finds one — a **flag** on `sqlkit.Dialect`
  (`TriggerOnTable`). SQL Server differs the other way: its trigger statements take two parts where
  every other statement that adapter writes takes three.

Both travel as one `driver.SourceRef`, which is what a drop, a read and a save are all addressed by.

## Opening one

Every one of these is on the tree's right-click, and a double-click on the row opens the definition
of anything that *is* one — a routine or a trigger, which have no rows and no folders.

| Where | What it offers |
|---|---|
| A **view** | `Open View` — its rows, read-only, in a table tab — then `Design View`, `New View…`, Compare, and `Delete View` |
| A **materialized view** | `Open Materialized View` — its *stored* rows — then `Design`, `New…`, **`Refresh Data`**, and `Delete` |
| A **sequence** | `Design Sequence`, `New Sequence…`, and `Delete Sequence` |
| An **event** | `Design Event`, `New Event…`, and `Delete Event` |
| A **function** or a **procedure** | `Design Function` / `Design Procedure`, `New Function…`, `New Procedure…`, and `Delete …` |
| A **trigger** | `Design Trigger`, `New Trigger…` on the same table, and `Delete Trigger` |
| A **table** | `New Trigger…`, with the rest of what is declared there — the only row that names the table one would fire on |
| The **`Views`** folder | `New View…` |
| The **`Materialized Views`**, **`Sequences`** and **`Events`** folders | `New …` for the one kind each of them holds. None is drawn at all on an engine that has not got that object — see [engines.md](engines.md#what-each-engine-can-be-asked-to-do) |
| The **`Routines`** folder | `New Function…` and `New Procedure…` — no engine has a folder for one of them alone |
| The **`Triggers`** folder | `New Trigger — right-click a table`, drawn dead. A trigger is declared *about* a table and this folder names none, so the item says where it is rather than vanishing — the rule [Compare's second table](../grid.md) follows |

**A new one is named before its tab opens.** The prompt asks for the name, and the tab opens on the
engine's own skeleton addressed to it — `CREATE VIEW "public"."orders_recent" AS`, and for a routine
the body shape that engine wants: MySQL wraps one in `BEGIN … END`, PostgreSQL puts it in a string
tagged with its language, SQL Server opens with `AS` and Oracle says `IS`. A trigger's skeleton
addresses **two** things, itself and the table it fires on, and diverges further still: PostgreSQL's
calls a function it cannot declare inline, and SQL Server's has no `FOR EACH ROW` at all. That
skeleton is a **flag** on `sqlkit.Dialect` (`Starters`) and not a template in a component, because it
is a statement and [nothing outside a driver spells one](../CLAUDE.md#rules-that-must-not-be-broken).

There is no second name box on the tab. The name is in the statement, which is where a declaration
puts it, and a box repeating it would be a second place for it to disagree with itself.

**One tab per definition**, like every other tab — and what identifies one is the whole reference
above, not the name alone. Two `New …` tabs are always two tabs: neither object is on the server yet
for a second one to be pointed at.

## Saving one

Two panes, and the second is the point of the first.

- **Definition** is the editor — the same one a query tab has, with the same completion over the
  namespace's tables, because a view's body *is* a query. It is not reviewed: what the reviewer
  warns about is a script of statements, and this is one statement that is meant to declare
  something.
- **Changes** is exactly what Save will run, in order. It is `PreviewSource`, and `SaveSource` runs
  that same list from the same speller — the arrangement [a migration's preview
  has](schema-changes.md), and for a stronger reason here.

**Saving an existing definition drops it first.** There are two statements in the Changes pane and
the first of them is `DROP VIEW …`, because nothing here rewrites the reader's text to say `OR
REPLACE` and a `CREATE` on its own would be refused for a name that is taken. There is **no
transaction** around the two — DDL is not transactional on three of these five engines — so a
create that fails after the drop succeeded leaves the object gone. That is what the preview is
for: it says `DROP` in as many words, before anything happens, every time.

A **new** object has one statement in that pane and no drop. Its Save turns the tab into an
ordinary one, so pressing Save again replaces the object rather than declaring it a second time and
being refused.

An empty editor spells **nothing at all** rather than the drop on its own, and Save is dead until
there is something in it. That is checked in `domain` terms on the Go side as well as here: a Save
that came to mean "drop it" would be the one thing this tab must never do.

`Refresh` re-reads the server's own words, throwing away whatever is in the buffer — `Reset` on a
new one, which puts the skeleton back. Both are what they say and neither asks first, the buffer
having never left this machine.

## What is not there

**A routine *is* run from here, in a pane of its own.** A function and a procedure are the two of
these that are called, so their tab has a third pane beside Definition and Changes — see
[routine-run.md](routine-run.md), which owns everything about what a routine takes and how the call
is spelled. A trigger is not run from anywhere: what fires it is a write to its table.

**`Refresh Data` is the one item a materialized view has that an ordinary one does not.** Its rows
are *stored*, so they go stale rather than being re-read: `REFRESH MATERIALIZED VIEW` on PostgreSQL
and `DBMS_MVIEW.REFRESH` on Oracle, which is a **flag** on `sqlkit.Dialect` and not a second
speller. It is not confirmed — it loses nothing and changes no shape — and it takes as long as the
query behind it takes.

**A sequence's value is not edited from here.** `ALTER SEQUENCE … RESTART` and `setval` are not the
same statement on any two engines and one of them is a function call; changing where a generator has
got to is a query tab's business, where the server answers for itself. The definition tab **shows**
where it has got to, which is the fact a reader came for.

**An event exists on MySQL alone.** Every other scheduler here is a *procedure called with
arguments* — Oracle's `DBMS_SCHEDULER.CREATE_JOB`, SQL Server Agent's `msdb.dbo.sp_add_job` — which
is not a declaration anything could read back and put again, so those engines answer
[`UNSUPPORTED`](../errors.md) naming it rather than listing a folder they could not fill.

**A trigger the server made for itself is not listed**, and neither is one the reader could not act
on: PostgreSQL's internal triggers are how a foreign key is enforced — the *constraint* is the thing
to edit — and SQL Server's DDL triggers are on the database or the server rather than on a table, so
no namespace's folder is the right place for one.

**No definition goes into a dump.** A dump is of tables — their shape and their rows — so a view, a
routine and a trigger are all outside it, exactly as they were before triggers existed. See
[transfer.md](transfer.md).
