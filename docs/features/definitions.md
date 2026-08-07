# Views and routines

[← Features and UI behaviour](../features.md)

## What a definition is

A table is a *shape*, and [the designer](schema-changes.md) draws it as three grids of fields,
indexes and keys. A view and a routine are not: a view is a query and a routine is a program, and
neither is a list of anything. So they are edited as what they are — the **statement that declares
them** — in a tab of their own.

**The buffer is the whole `CREATE`**, exactly as the server answered with it. Four of the five SQL
engines will say what they hold:

| Engine | A view's definition comes from | A routine's comes from |
|---|---|---|
| MySQL | `SHOW CREATE VIEW` | `SHOW CREATE PROCEDURE` / `FUNCTION` |
| PostgreSQL | `pg_get_viewdef`, under a `CREATE OR REPLACE` spelled by the driver | `pg_get_functiondef`, whole |
| SQLite | `sqlite_master.sql` — the statement the file stores | — |
| SQL Server | `sys.sql_modules.definition` | the same |
| Oracle | `all_views.text`, under a `CREATE OR REPLACE` spelled by the driver | `all_source`, under one |

It is a **read** and not a render wherever it can be, for the reason a dump's `CREATE TABLE` is on
the engines that answer for themselves: a declaration put back together out of a catalogue is only
ever as complete as the catalogue's columns. Where an engine keeps the body alone — PostgreSQL and
Oracle both answer for a view with its query and nothing else — the `CREATE` in front of it is the
**driver's** to spell, like every other statement.

**MySQL's answer names no database**, so the driver qualifies it: a routine put back as it came
would land in whichever database the pool happened to be on rather than the one it was read from.
Oracle's is qualified for the same reason. Nothing else about the text is touched — a `DEFINER`
clause, an `ALGORITHM`, a `sql_mode`-dependent body all reach the reader as the server spelled them.

**There is no `DELIMITER` anywhere, and none is needed.** A definition is *one* statement however
many semicolons a routine body holds, and it is run as one — nothing on either side of the wire
re-lexes what the reader typed. That is the same rule the rest of the app follows from the other
end: [reading a script is the driver's](script.md), and this never asks.

**Two engines have no definition to read.** A MongoDB view is a collection declared by a *document*
— the collection it reads and the pipeline over it — rather than by a statement, and a Redis key
space is not declared at all. Both answer [`UNSUPPORTED`](../errors.md) naming which, and the tab is
not offered on either: `EngineInfo.hasSource` is what says so, and no component compares against an
engine's name. See [engines.md](engines.md#what-each-engine-can-be-asked-to-do).

## Opening one

Every one of these is on the tree's right-click, and nothing else is needed to reach them.

| Where | What it offers |
|---|---|
| A **view** | `Open View` — its rows, read-only, in a table tab — then `Design View`, `New View…`, Compare, and `Delete View` |
| A **function** or a **procedure** | `Design Function` / `Design Procedure`, `New Function…`, `New Procedure…`, and `Delete …` |
| The **`Views`** folder | `New View…` |
| The **`Routines`** folder | `New Function…` and `New Procedure…` — no engine has a folder for one of them alone |

**A new one is named before its tab opens.** The prompt asks for the name, and the tab opens on the
engine's own skeleton addressed to it — `CREATE VIEW "public"."orders_recent" AS`, and for a routine
the body shape that engine wants: MySQL wraps one in `BEGIN … END`, PostgreSQL puts it in a string
tagged with its language, SQL Server opens with `AS` and Oracle says `IS`. That skeleton is a
**flag** on `sqlkit.Dialect` (`Starters`) and not a template in a component, because it is a
statement and [nothing outside a driver spells one](../CLAUDE.md#rules-that-must-not-be-broken).

There is no second name box on the tab. The name is in the statement, which is where a declaration
puts it, and a box repeating it would be a second place for it to disagree with itself.

**One tab per definition**, like every other tab — and a routine is named by its arguments as well
as its name, because on PostgreSQL two functions may share one and be told apart only by what they
take. Two `New …` tabs are always two tabs: neither object is on the server yet for a second one to
be pointed at.

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

**A routine is not run from here.** Executing one with its arguments filled in is a query tab's
business — `CALL …` or `SELECT f(…)` — and this tab declares rather than calls.

**A materialised view is not listed at all**, so it has no definition to open either: it is dropped
and refreshed by statements of its own, and a list that cannot be acted on is worse than one that
does not claim it. See [schema-tree.md](schema-tree.md#browsing-a-schema).
