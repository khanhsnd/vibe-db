# Query tabs

[← Features and UI behaviour](../features.md)

**New Query** sits in the command bar beside **New Connection**, and as the `+` at the end of the tab
strip; both open the same tab, which inherits the selected connection and namespace, falling back to
whichever connection is open.

A query tab carries its own toolbar, editor buffer, database and row limit, so several tabs can sit
on the same connection without treading on each other. The toolbar picks the database, then
**Execute** (`F5` or `Ctrl`+`Enter`) or **Explain** runs the statement; **Format**
(`Ctrl`+`Shift`+`F`) lays it out; **Stop** cancels it.

- The namespace is applied on a connection pinned for that one statement — `USE` on MySQL,
  `SET search_path` on PostgreSQL — so the pool's other connections keep whatever they had. On
  PostgreSQL the database also decides *which pool* the connection comes from. On MongoDB it is
  simply the database the command is addressed to; one connection reads them all.
- A statement that returns rows fills the grid; one that does not reports its affected-row count.
  Which path a statement takes is decided by its leading keyword, read past any leading comments,
  plus a `RETURNING` clause on PostgreSQL, where that is what makes a write answer with rows. On
  MongoDB it is decided by the *method*: `find` and `aggregate` answer with documents, `insertOne`
  and `deleteMany` with a count.

## What Explain shows

**Explain runs the statement and asks about it as well.** `Result` therefore holds the statement's own
rows — the same rows Execute would have put there, writable on the same terms — and two more panes
appear beside it, `Summary` and `Explain`, holding what the server said it was going to do. It is
guarded like Execute for exactly that reason; see
[What gets confirmed before it runs](connections.md#what-gets-confirmed-before-it-runs).

- **The plan is read before the statement runs**, and that order is the whole of what makes it worth
  reading: asking afterwards would ask about a server the statement has already changed, and what
  `DELETE FROM t` was going to do is not what the planner says once the rows are gone. On MongoDB it
  also means a command with no plan answers `UNSUPPORTED` while it has still changed nothing.
- **The plan is read by the driver, never by the frontend.** Each engine asks in two forms — the
  `EXPLAIN` a reader would have typed, and the JSON form that carries a number per step — and both
  land in `driver.Plan`. **Neither is ever the `ANALYZE` form**: the statement runs once, on the path
  every statement takes, and the plan alongside it is an estimate.
- **Either form may fail on its own and the other is still shown.** A plan that cannot be had at all
  is no plan rather than a failure: the statement has run, its rows are in hand, and the two extra
  panes simply do not appear.
- `Summary` is what the plan says about itself, in the engine's own wording: a total cost and a
  startup cost on PostgreSQL, a query cost and a table count on MySQL, the winning stage and how many
  plans lost on MongoDB. It is a list of pairs rather than a fixed set of fields, because a field for
  a fact an engine has not got would only be filled with a zero that read as an answer.
- **`Explain` holds three tabs**, and they are three because they answer different questions:
  - **`Grid`** — the engine's own EXPLAIN, exactly as it answered: MySQL's grid of access types,
    PostgreSQL's plan text down one column, MongoDB's one document. Nothing is normalised, reordered
    or renamed — a reader who reaches for Explain knows these words, and rewriting them would put this
    client between them and their server.
  - **`JSON`** — the plan document as the server wrote it, laid out, with numbered lines and folds.
    It is where a step's cost and the shape of the tree actually live, and it is the answer to "but
    what did the server *say*", which no reading of it can be.
  - **`Statistics`** — one row per kind of step: how many there are, what they cost between them, and
    what share of the plan that is, dearest first. It counts a step's **own** cost, meaning what it
    adds beyond what it reads, so the shares add up to the plan rather than to several times it.
- **A step the engine did not price is left blank, not zeroed.** MySQL prices reading a table and
  evaluating its rows; it does not price the sort or the grouping above them, and a `0.00` there would
  read as free rather than as unpriced. A document store prices nothing at all, so its statistics carry
  no cost columns — `Costed` says so, and the grid is not left to infer it from a column of zeroes.

## What a MongoDB tab runs

The editor takes the shell's own commands, and the driver reads them whole — the grammar is small
enough to. `db.<collection>.<method>(…)` for a collection, `db.<method>(…)` for the database, plus the
cursor methods a read is bounded by:

```javascript
db.users.find({ tier: "gold" }).sort({ joined: -1 }).limit(20)
db.users.findOne({ _id: ObjectId("507f1f77bcf86cd799439011") })
db.orders.aggregate([
    { $match: { total: { $gt: 100 } } },
    { $group: { _id: "$customer", spend: { $sum: "$total" } } }
])
db.users.updateMany({ tier: "gold" }, { $set: { note: "reviewed" } })
db.getCollection("audit-log").countDocuments({})
db.runCommand({ collStats: "users" })
```

- **The arguments are documents, not text.** A field name and a value reach the server as *values*, so
  there is nothing to quote and nothing a reader typed can be read as anything else.
- **The shell's constructors are read as themselves**: `ObjectId`, `ISODate` / `Date`, `NumberLong`,
  `NumberInt`, `NumberDecimal`, `UUID`, `BinData`, `Timestamp`, `RegExp`, `MinKey`, `MaxKey`, a
  `/pattern/flags` literal, and `new` in front of any of them. A filter that spelled an ObjectId as a
  string would match nothing and say nothing about why, which is the whole reason this is a parser and
  not `JSON.parse`.
- **It is not JavaScript.** There are no variables, no arithmetic and no control flow, because a client
  that half-implemented them would be a client whose answers could not be trusted. **Anything beyond
  the methods this side knows is `db.runCommand({…})`**, which reaches the server verbatim — and the
  error for an unknown method says so.
- **A `find` on one collection is writable**, its documents mapping one to one onto stored ones with
  `_id` addressing them. A projection that leaves `_id` out is read-only, for the same reason an
  aliased key column makes a SQL result read-only: the row it names can no longer be found.
- **Explain** works on `find` and `aggregate`, those being the two with a plan; anything else answers
  `UNSUPPORTED` rather than pretending — and it answers **before the command runs**, so explaining an
  `insertOne` inserts nothing. `Explain` → `Grid` holds the plan as one row whose cells are the plan's
  own nested documents, **View Value** being where one is read, and `Statistics` holds the same answer
  as its stages: `FETCH`, `IXSCAN`, or the `$cursor` that feeds a pipeline and the plan under *it*.
  There is no cost and no row estimate, a document store pricing nothing, so those columns are not
  drawn; see [What Explain shows](#what-explain-shows). A server on the slot-based engine reports its
  plan twice — as stages and as the slots it compiled them into — and it is the stages that are read.
- **`Ctrl`+`Shift`+`F` lays a command out** the way it lays a statement out: a brace holding more than
  a line comfortably carries goes down the page, and one that does not stays across it. Nothing a
  server would notice changes.

## What a Redis tab runs

**A newline ends a command.** There is no terminator to leave out — a Redis command is a line of
arguments — so a script is its lines, and a semicolon at the end of one is read as punctuation and
dropped, because every file this app writes ends its statements with one. An argument is a run of
characters with no space in it, or a quoted run: `SET greeting "hello world"` is three arguments, and
`redis-cli`'s own escapes (`
`, `A`) work inside the quotes. A quote **anywhere** in an argument
opens a quoted run, exactly as `redis-cli` reads one, so JSON typed bare loses its quotes and JSON
typed inside `'…'` does not.

**What comes back is drawn in whichever of four shapes it has**: one value in one cell, a list as
rows, a list of lists as a column per position (`XRANGE`), and the pairs of a map as Field and Value
(`CONFIG GET`, `XINFO STREAM`). A key that is not there is a NULL cell rather than an error, that
being what the server actually said.

**A handful of commands are refused before they are sent**, each because the reader could not see
what it would do. `SELECT` and `SWAPDB` change which database the *connection* is on, and that
connection is one of a pool shared by every read of that database — a tab that ran one would leave
the grid reading from somewhere else at a moment nothing could predict; the database a tab reads is
the one it was opened on. `SUBSCRIBE` and `MONITOR` never answer, and would hold a pooled connection
open for as long as the app was. `MULTI`, `WATCH` and `EXEC` cannot span two commands here, each
being sent on its own connection. `EXPLAIN` has no meaning: Redis has no plan to ask for.

**`FLUSHDB` and `FLUSHALL` are confirmed on a laptop as well as on production.** They are classified
*unfiltered* rather than destructive — they name nothing and therefore take everything, which is
exactly what a `DELETE` with no `WHERE` is — and that is the one class this app stops for wherever it
is run. See [what gets confirmed before it runs](connections.md#what-gets-confirmed-before-it-runs).

## One Execute runs the whole script

**Execute runs whatever the editor holds**, whether that is one statement or twenty. The text is cut
into the statements it carries and they run in order on the same connection, stopping at the first
the server rejects.

- **Where a statement ends is the engine's**, so the splitter is the driver's: `sqlkit.Split` reads
  the semicolons that are *not* inside a string, a quoted identifier, a line or block comment, or —
  on PostgreSQL — a `$tag$ … $tag$` body, which is how a function carries semicolons of its own. A
  backslash escapes a quote on MySQL and does not on PostgreSQL, so `'C:\'` is one string there and
  a runaway one here. A chunk with no statement in it — trailing whitespace, the comment a script
  ends on — is dropped rather than sent. MongoDB reads the same semicolons through the same lexer,
  with `//` opening a comment and both quotes opening a string.
- **The grid shows the last result set the script produced**, so a script ending in a `SELECT` shows
  it and one ending in an `UPDATE` keeps the last set it did return.
- **A `Messages` pane appears beside `Result` for a script**, listing every statement with its row
  count, its affected count, its time and either `OK` or the server's own wording. It is a
  `ResultGrid`, because it is rows and columns.
- **A rejection is a message, not a failure — for a script.** One statement on its own still fails
  as a failure, so the output panel shows the server's wording where the reader is looking; but the
  statements above a rejected one *did run*, and throwing would hide how far the script got. A
  script that stops opens the `Messages` pane by itself, and its `Messages (n)` tab says how many
  statements there were to account for.
- **Rows are paged under the grid by the same pager the table view has.** What a read returned is a
  *prefix* of the result, and the pager cuts pages out of it in memory — turning to a page it
  already holds reads nothing. A total shown as `1,000+` on the status line says the prefix stopped
  at the toolbar's `Limit`, and that is also what keeps `Next` alive on the last page: asking for a
  page past the prefix, or clearing the gear's `Limit records`, **replays the statement for as many
  rows as that needs**. So lifting the bound really does load all, and paging is never stuck at the
  limit. Two things follow from the replay: a statement
  with no `ORDER BY` may reshuffle what lands on a page, and the reader keeps their page number
  across a read of the same statement — only a different statement sends them back to page one.
- **A header's `⋮` menu sorts the result in memory**, since there is no statement to re-order: it
  reaches the rows fetched so far and no further, so on a truncated read the prefix is sorted rather
  than the table. Fetching more re-sorts what arrives. A different statement drops the sort. **A
  query result is not filtered** — narrowing belongs to a tab whose rows come from a statement this
  side writes, which is the table tab.
- **Errors surface in the output panel below the editor**, with the code, the server's own wording
  and the engine's error code, never as a dialog. A cancel is reported as `QUERY_CANCELLED`, not as
  a failure.
- Progress shows in two places: `Running…` beside the toolbar buttons, and a spinner on the pager
  under the grid. Previous rows stay on screen until the new ones land.
- **How many rows came back, and how long it took, are on the grid's own status line** — not on a
  strip of their own under the panel. There was one, and it said `325 rows · 5,077 ms` directly
  beneath a line already reading `Record 1 of 325`: two accounts of one read, a row apart. The
  elapsed time went with it, `ElapsedMs` still crossing the wire for whatever wants it next.
