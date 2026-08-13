# Running a routine

[← Features and UI behaviour](../features.md)

## Where it is

A function and a procedure are the two [definitions](definitions.md) that are *called*. A view is
selected from, a trigger is fired by a write to its table, and a sequence hands out a number — none
of those is a thing a reader fills arguments in for. So the definition tab of a routine has a third
pane, **Run**, beside `Definition` and `Changes`, and nothing else in the app has one.

It is where it is because that is where the reader is standing. Editing a procedure and then finding
out whether it works were two tabs and two namespaces apart; now the Save and the Run are one
toolbar and one pane apart.

**It is not offered on a routine that is not there yet.** A `New Procedure…` tab has nothing on the
server to call until its Save has run, and the arguments are read from the catalogue rather than out
of the buffer — so the pane appears once the object exists, the same moment the tab stops being a
`Create` and starts being a `Save`.

## What it takes

The parameters are a **catalogue read** — `information_schema.parameters`, `pg_proc`,
`all_arguments`, `sys` — and never a reading of the definition in the pane beside it. A body is
text, and a parameter list picked out of one would be a second parser on the wrong side of the wire.

Each parameter is a row of a grid with four columns. The first three are the server's and are
read-only, because what a routine takes is the routine's: a reader who wants it different edits the
definition next door.

| Column | What it is |
|---|---|
| **Name** | The parameter's own, or `arg1`, `arg2` … where the engine's declaration named none |
| **Mode** | `in`, `out` or `inout` — whether the value is written into the call or read back out of it |
| **Type** | The engine's own spelling: `int`, `varchar(50)`, `NUMBER`. It is never parsed here |
| **Value** | The one cell the reader types in |

**A function's return is not a parameter.** It is what the call answers with, and no catalogue here
calls it one — the row that stands for it is left out rather than drawn as a box nothing could be
typed into.

**An OUT parameter has no value to type**, its cell saying `(given back)`. It is what the routine
hands the reader, and the call is spelled so that it comes back — see below.

## What a value is

A value is written into the statement as a **string literal**, whatever the parameter's type is.
That is the bargain [a filter's value](filter-sort.md) already makes and for the same reason: every
one of these engines reads a literal back into a parameter of any type, so `'30'` reaches an `int`
as thirty and `'2027-01-01'` reaches a date as that day.

**An empty box is `NULL`** — the keyword, not the word. There is no separate tick for it, because
there is nothing else an empty box could honestly mean for a parameter that has to be given
something.

**Nothing is bound.** A call that bound its arguments could not be shown as the statement it runs,
and that preview is the whole point — see below.

## What runs

Under the arguments is **exactly what Run will send**, in order. It is `PreviewCall`, and
`CallRoutine` runs that same list from the same speller — [rule 13](../../CLAUDE.md#rules-that-must-not-be-broken)
again, and it earns its keep here more than anywhere: a procedure is the reader's *own program*, and
this is the last thing in front of them before it touches their rows.

Every statement of it is spelled by one speller in `sqlkit`, and what differs between the five SQL
engines is a **flag** (`sqlkit.CallStyle`) rather than a second spelling:

| Engine | A procedure | A function | An OUT parameter comes back |
|---|---|---|---|
| **MySQL** | `CALL db.p('30')` | `SELECT db.f('30')` | in a **user variable**: `SET @v = NULL`, the call, then `SELECT @v AS name` |
| **PostgreSQL** | `CALL s.p('30', NULL)` | `SELECT * FROM s.f('30')` | in the **row `CALL` answers with**, so nothing is set and nothing is read after |
| **SQL Server** | `EXEC db.dbo.p '30'` | `SELECT db.dbo.f('30')` | in a variable **declared in the same batch**, so the whole call is one statement holding three |
| **Oracle** | `BEGIN s.p('30'); END;` | `SELECT s.f('30') FROM dual` | it does not — see below |
| **SQLite** | — | — | — |

**They run on one connection.** That is not a convenience: on MySQL the value comes back through a
user variable and a variable belongs to the connection, so a second one would read a variable
nothing had set. It is the same pinned connection [a definition is saved on](definitions.md#saving-one)
— which means a call made while [a manual transaction](transaction.md) is open joins that
transaction, exactly as every other statement of the session does.

**A function is read and a procedure is run**, which is why the answer differs: a function's value
and a procedure's rows come back as rows, and a procedure that returned neither reports what it
changed instead.

## What is not there

**Oracle hands back no OUT parameter.** A PL/SQL block answers with nothing, so a value it would
give back reaches a client only through a *bound* variable — and a bound call is not the statement
the preview showed. So the call spells nothing at all and the run answers
[`UNSUPPORTED`](../errors.md) naming it, rather than sending a statement the server would refuse.
The routine's IN-only siblings run there like anywhere else.

**A routine of a package is not listed and so is not run.** It is addressed by two names and is not
an object of the schema on its own — the same reason the tree does not draw one.

**A table-valued function on SQL Server is not read as a table.** Its call is spelled as a scalar
one, that engine having two grammars and one catalogue answer this app does not tell apart; a
`SELECT * FROM f(…)` typed in a [query tab](query-tab.md) is what reads one.

**Nothing here is confirmed and nothing is guarded.** What a routine does is its own body's
business and no lexer on this side can read it, so the [environment
guard](connections.md#which-environment-a-connection-is-for) has nothing to weigh — the preview is what
stands between the reader and the run.

**The call is not kept in the History Log.** It is a schema surface rather than a statement the
reader typed, and nothing else on that surface is kept either — see [log.md](log.md#what-is-in-it).
