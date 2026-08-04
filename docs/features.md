# Features and UI behaviour

Working: connection profiles (create / edit / delete) for **MySQL and PostgreSQL**, Test Connection
with server version and latency, connect / disconnect, **creating and dropping a database**,
**tables, views and routines listed per namespace**, **a table list of what the catalogue keeps about
each table**, **a table's fields, indexes and foreign keys in
the tree and in a designer tab**, **schema changes from the tree's context menu** — new / rename /
empty / truncate / drop a table, add / edit / rename / drop a field, add or drop an index or a
foreign key, drop a view or a routine — table data browsing with pagination, cell editing plus row
insert and delete on any grid whose rows can be traced to one table, **exporting a grid to CSV, JSON
or SQL**, query tabs with completion that **run a whole script**, **a history of every statement run,
kept on disk and put back in the editor by a double-click**, passwords in Windows Credential
Manager, and the full layout shell with dark and light themes and a **data font the reader picks**.

Not there yet: a **materialised view** is not listed, because it is dropped by a statement of its
own and a list that cannot be acted on is worse than one that does not claim it. A routine can be
dropped but not read or written — there is no body editor. A column's **comment** is neither read
nor written, so the designer has no column for one rather than a column that quietly drops what is
typed into it. An export writes what the grid holds rather than replaying the statement for
everything behind it.

## Picking an engine

**A new connection picks its type before it is filled in.** The dialog opens on a grid of engine
tiles — each engine's own mark at a size that reads by shape — and `Next` moves on to the form; a
double-click on a tile does both. `Back` in the form returns to the grid, which is also how an
existing connection changes engine: editing one opens straight on the form, since it already has a
type.

Everything the form defaults to comes from that choice: port 3306 and user `root` for MySQL, 5432
and `postgres` for PostgreSQL. Going back and choosing another engine rewrites only the fields that
still hold the old engine's default, so a typed port survives the switch. The editor loads that
engine's SQL dialect for highlighting and completion.

**Every connection wears its engine's chip** — `M` on teal for MySQL, `P` on blue for PostgreSQL —
in the tree, in the dialog's header and in the status bar. It replaces the engine badge the tree
used to carry on the right of a connection row, which is where the **connection status dot** now
sits: what the connection *is* on the left, how it is *doing* on the right.

One field differs rather than just defaulting: **PostgreSQL needs a database**. A session is bound to
one for life, so that is the database the connection bootstraps from — the one its database list is
read in. Reaching any other database dials it separately, which the adapter does on first use.

## Browsing a schema

The tree has as many levels as the engine does:

```
MySQL         connection → database →          Tables    → table → Fields       → field
PostgreSQL    connection → database → schema →  Views     → view          Indexes      → index
                                                Functions → function      Foreign Keys → foreign key
```

Each level loads on first expand and is cached until disconnect: a PostgreSQL database loads its
schemas, whichever node owns the object folders loads **its tables, views and routines in one
read**, and a table loads its shape — fields, indexes and foreign keys, also in one read. The reason
is the same both times: expanding any one folder needs every folder's count, so one read answers all
three. Clicking the node that owns tables — a MySQL database, a PostgreSQL schema — marks it the
active namespace for that connection, which is what a new query tab inherits.

A **view opens its rows in the same tab a table does**, and lands there read-only: no column of a
view is a primary key, so no single stored row can be named. A **function or a procedure** carries
its argument list as its badge, because on PostgreSQL two of them may share a name and be told apart
only by what they take — which is also why dropping one sends the whole routine and not its name.

A field carries its **bare** type as a badge and a key icon when it is part of the primary key; an
index carries its columns, and a foreign key the table it points at. The badge is the bare type and
not the declared one because six levels of indent leave no room for `character varying(255)` — the
whole of it is in the row's tooltip, and in the designer.

**One click selects, two clicks open** — see [frontend.md](frontend.md). Applied to the tree:

| Gesture | What happens |
|---|---|
| Click a row | Selects it, and names the active namespace if that row owns tables. Reads nothing — except on the **`Tables` folder**, which also opens [the table list](#the-table-list): what that folder *is* to a reader is the tables and their numbers, and the tree has room for the names alone. |
| Click the caret | Selects the node **and** opens or closes it, loading that level on first open. |
| Double-click a row | The same open — except on a **table**, which opens a data tab. A table has folders under it now, but double-clicking one has always meant "show me the rows", and the caret is what expands. |
| Open a closed connection | Dials it, whichever gesture asked. A disconnected connection node still carries an empty child list, so expanding it would otherwise spin the caret over nothing. The store expands the node when the session lands, and a failed connect leaves the caret shut. |

## The table list

Clicking the `Tables` folder opens a tab of **every table of that namespace with what the
catalogue keeps about it** — one row per table, in a `ResultGrid` like everything else that shows
rows and columns. Over it sit `Open Table`, `Design Table`, `New Table` and `Delete Table`; under
it the same `GridFooter` every grid has, with `Refresh` and the table count.

- **The columns are the engine's own**, and only the first is promised: the table's name, which is
  what the toolbar acts on. MySQL reports `Auto Increment`, `Modified Date`, `Data Length`,
  `Engine`, `Rows` and `Comment`; PostgreSQL reports `Owner`, `Access Method`, `Data Length`,
  `Rows` and `Comment` — and **no modified date, because the server keeps none**. Neither side of
  the wire names or formats a value: a size arrives spelled the way that engine spells it
  (`1,552 KB` from MySQL's own arithmetic, `1928 kB` from `pg_size_pretty`), because inventing a
  column an engine cannot fill would say nothing. A row count is the catalogue's **estimate** on
  both engines, and `(Null)` in PostgreSQL's says nothing has analysed the table yet.
- **The toolbar acts on the selected row**, and its four items are the same objects the tree's
  right-click builds from (`tableActions`), so a confirmation is worded once. `Delete Table` asks
  first, and the change re-reads both the tree's names and this list's numbers.
- **A double-click opens the row's table**, which is the tree's own gesture applied to a row: the
  grid is read-only here, so a click has nothing else to mean. `ResultGrid` reports it through
  `onActivateRow` — see [grid.md](grid.md).
- The list is **read per tab rather than cached** like the tree's names are: a size and a row count
  are what the reader opened it for, and a stale one is worse than a second query.

## Changing a schema

Every schema change is raised from a right-click in the tree, and every one of them is confirmed or
filled in before it runs. `useTreeMenu` is the single place the items are built, per node kind:

| Right-click | Items |
|---|---|
| A connection | `Connect` / `Disconnect`, `New Database…` — only once there is a session to make one on — `Edit Connection…`, `Delete Connection` |
| A database | `New Database…`, `Delete Database`, then what its level owns: `New Table` and `Refresh` on MySQL, `Refresh` alone on PostgreSQL, which holds schemas rather than tables |
| The node owning tables, or its `Tables` folder | `New Table`, `Refresh` |
| The `Views` or `Functions` folder | `Refresh` — one read fills all three folders, so it means the same wherever it is asked for |
| A table | `Open Table`, `Design Table`, `New Table`, `Rename Table…`, `Empty Table`, `Truncate Table`, `Delete Table`, `Refresh` |
| A view | `Open View`, `Delete View`, `Refresh` |
| A function or procedure | `Delete Function` / `Delete Procedure`, `Refresh` |
| The `Fields` folder | `Add Field`, `Refresh` |
| A field | `Edit Field`, `Rename Field…`, `Add Field`, `Delete Field`, `Refresh` |
| An index | `Delete Index`, `Refresh`. Disabled on the index behind a primary key: neither engine will drop the key as an index, so the item says so rather than failing at the server |
| A foreign key | `Delete Foreign Key`, `Refresh` |

- **Every item carries a glyph**, from `shared/ui/menuIcons.tsx` — one action, one icon, wherever it
  is offered: `Design Table` and `Edit Field` share theirs because both open the designer, and
  anything that destroys carries the bin in the danger colour.
- **`Empty Table` deletes every row; `Truncate Table` drops them all at once.** Truncate is the
  faster of the two and the one a table under a foreign key will refuse, which is why both are there.
- **A change re-reads what it invalidated, and nothing else**: the namespace's table list, then the
  table's own shape. Which shapes are still worth holding is read off the fresh table list, so a
  dropped or renamed table needs no telling.
- **A rejected statement stays where it was asked for** with the server's own wording, so a name
  already taken costs a retype and not the work.

### Where a schema change is made

**Anything with more than one answer in it happens in a tab, not a dialog** — `New Table`,
`Design Table`, `Add Field` and `Edit Field` all open (or reveal) the designer, which has the room
to work in and leaves the tree beside it readable. Three requests stay modal: `Rename …`, which is
one box; a confirmation, which has to be answered before anything else happens; and `New Database…`,
which has **no tab of its own to open** — a database is not a thing you sit and work in, and its
answers are one fixed screen rather than a list.

### New Database

The dialog is a tab strip over a form, and how much of the form there is belongs to the engine.
**PostgreSQL's database is an object in its own right**, so `General` asks for a Database Name, an
Owner, a Template, an Encoding, a Collation Order, a Character Classification, a Tablespace and a
Connection Limit, plus `Allow Connection` and `Is Template`; `Extensions` is a filterable list of
what the server has to offer; and `Comment` is free text. **MySQL's database is a namespace with a
character set**, so `General` asks for a name, a Character Set and a Collation — and the other two
tabs are not drawn, because MySQL has neither.

- **Every list is the server's own**, read in one call when the dialog opens: the roles, the
  templates, the encodings, the tablespaces, the available extensions. A blank entry reads
  `Server default` and leaves that clause out of the statement altogether, rather than this side
  guessing at what the default is.
- **MySQL's collations narrow to the chosen character set** without a second read, because a MySQL
  collation is named after the set it belongs to — `utf8mb4_general_ci`.
- **`SQL Preview` asks the driver what it would run.** It is the same list `Create` executes, so the
  two cannot drift; a database with no name yet previews nothing. An extension is shown as its own
  statement because it *is* one, run inside the database that has just been made.
- `Connection Limit` keeps `-1`, which is PostgreSQL's own "no limit", for anything typed into it
  that is not a number — a blank box meaning `0` would be a ban rather than an absence.

`Add Field` from the tree opens the designer with a blank field already appended, which is the one
thing a tab cannot say by being opened. It travels as a slot in the schema store rather than a field
of the tab, because the tab may already be open and a revealed tab has to hear the second ask too.

### The designer

`Design Table` opens a tab laid out the way a table designer is: a toolbar of `Save`, `Add …`,
`Delete …`, `Primary Key` and `Refresh`, over a strip of `Fields`, `Indexes`, `Foreign Keys` and
`Changes`. **`Save` takes the accent the moment there is anything to save** and drops back to a
plain toolbar button when there is not, and `Delete …` carries the danger tone, so the two
buttons that matter are readable before the labels are. `New Table` is the same tab with no table behind it yet and a name box in the toolbar;
its `Save` creates the table, and the tab then goes on designing it rather than offering to create
it twice — its indexes and foreign keys are made straight after, since `CREATE TABLE` carries the
fields and the primary key and nothing else.

- **The toolbar acts on the pane in front.** `Add` and `Delete` read `Field`, `Index` or
  `Foreign Key` depending on the tab, and `Primary Key` is live only over `Fields`. That is why the
  tab strip's selection is held by the designer rather than inside the strip. Changing tab clears
  the selection, because a cell reference is a position in the pane that reported it.
- **Every list is a `ResultGrid`**, because they are rows and columns — see [grid.md](grid.md), and
  **all three are edited in place**: one click on a cell opens it, `Escape` closes it again, and
  the arrows walk the rows a cell at a time.
- Its columns are the parts a field is *declared* from, not the string the server reports back:
  `Name`, `Type`, `Length`, `Decimal`, `Not null`, `Key`, `Default`. Length and decimals are parts
  of the type and travel as one string — `varchar(255)`, `numeric(10,2)` — and read as `0` when the
  type declares none. A `Default` is an **expression**, which is what a server-side default is:
  `now()`, `'IN_PROGRESS'`, `0`.
- **`Type` is a list, and the list is the engine's.** Clicking the cell offers MySQL's types or
  PostgreSQL's — `dataTypes.ts`, read off the connection's own profile — and typing filters them.
  It still takes **anything typed into it**, because a server also has domains, enums and types
  that list has never heard of; the options are what a column is usually given.
- **`Not null` is a checkbox and `Key` is the lock**, both flipped by one click. A key field shows
  a gold lock and **which column of the key it is** — the first is `1`, a second field clicked is
  `2` — counted down the fields, so a composite key reads in order. `Primary Key` in the toolbar is
  the same fact for a whole selection at once.
- `Primary Key` moves the key onto the selected fields, or off them if they all already hold it.
  Both engines are made to replace a key rather than add a second one, and dropping the key of a
  table that has none is not an error.
- **The Indexes grid** is `Name`, `Fields`, `Unique` and `Method`, and **the Foreign Keys grid** is
  `Name`, `Fields`, `Referenced Schema`, `Referenced Table`, `Referenced Fields`, `On Update` and
  `On Delete`. A list of columns is typed as one cell, comma separated, because that is how it
  reads. `Method` offers the engine's access methods and the two action cells offer the five
  referential actions — suggestions, like the type list, since the cell still takes what is typed.
  A blank `Referenced Schema` means the same namespace the table is in.
- **An index behind the primary key is not in the Indexes grid.** It is the key, and the Fields grid
  is what moves a key.
- **Nothing is sent until `Save`.** A pending cell carries the grid's warning ring; a field the
  server has not seen is ringed whole, and so is *any* pending index or foreign key — neither engine
  can alter which columns one covers, so an edited one is dropped and made again rather than changed
  a cell at a time. `Changes` lists what `Save` will do, in the order it will do it: **what is going
  goes first** — keys, then indexes — so nothing stands in the way of the field changes that follow;
  then the fields, a rename before the alter so the alter can still name the column; then **what is
  arriving**, so an index can name a column the same `Save` has just added. It reads as plain
  changes and not as SQL, because how a statement is spelled belongs to the driver and this side
  has never seen one.
- `Refresh` re-reads the table and drops whatever was staged. Nothing in the Indexes or Foreign Keys
  grids is ever `(Null)`, and neither is a missing length: a type with no length declared none, and
  `(Null)` would claim a column defaults *to* null.

The query toolbar mirrors the same shape: one `Database` picker on MySQL, and a `Schema` picker
beside it on PostgreSQL. Changing the database clears the schema, since a schema of one database
names nothing in another.

**Double-clicking** a table opens a data tab; a single click only selects it, and reads nothing from
the server. **Until the first page lands the pane says so** — `Loading <table>…` under a spinner,
from `shared/ui/Loading` — because a grid with no columns yet is a blank pane and a table of any
size can take seconds to answer. Only the *first* read gets it: once rows are on screen a re-read
leaves them there and the pager alone says it is working, which is what keeps a page turn from
blinking. A tab loads 100 rows per page by default, ordered by primary key so paging is stable —
until a header's `⋮` menu, or the Filter & Sort panel, gives it keys of its own; those travel into the
read's `ORDER BY` and therefore order the whole table rather than the page on screen.
**Paging lives on the right of the bar under the grid** — first, back, the page typed into a box,
forward, last — with a **gear** beside it that opens `Limit records` and the count that sizes a page,
and can lift the bound entirely; see [the row limit](#the-row-limit). **Under that bar is the status
line**: the read that produced the rows with a copy beside it, where the reader is in what it
returned, and how long ago it landed. The left of the control bar is where rows are added, deleted,
applied and discarded: it is the same `GridFooter` every grid has, and
[grid.md](grid.md) specifies both it and the line under it. Above the grid is one toolbar: **Export** against its right edge,
**Filter & Sort** on the left of it — see [filtering and sorting](#filtering-and-sorting) — and before
that two independent toggles, since they occupy different edges:

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft and
  Apply stages the value on the cell, exactly as opening it in the grid does — the footer's
  ✓ is what writes `UPDATE … WHERE <primary key>` and reloads. A table with no primary key is
  read-only, because no single row can be identified. MySQL adds a `LIMIT 1` belt to the braces;
  PostgreSQL has no such clause and does not need one, since the key already names one row.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes. **Right-clicking a column
  header hides or restores columns too**, alongside the copy and paste items every grid's header menu
  has — one hidden set, either way in. [grid.md](grid.md) specifies that menu.

## Tabs

**Every tab says in full what it is showing when the pointer rests on it**: what kind of thing and
its name, then `Database:`, `Connection:` and `Server:` as `user@host:port`. The strip truncates to a
name at 220 px, and two tabs called `users` on two connections are otherwise the same tab — the
tooltip is where that is settled. It is lines rather than a sentence because a reader is looking for
one of those facts, not reading prose, and it is the platform's own tooltip rather than a component,
which is what makes it appear over a tab the pointer merely passes.

## Query tabs

**New Query** sits in the command bar beside **New Connection**, and as the `+` at the end of the tab
strip; both open the same tab, which inherits the selected connection and namespace, falling back to
whichever connection is open.

A query tab carries its own toolbar, editor buffer, database and row limit, so several tabs can sit
on the same connection without treading on each other. The toolbar picks the database, then
**Execute** (`F5` or `Ctrl`+`Enter`) or **Explain** runs the statement; **Stop** cancels it.

- The namespace is applied on a connection pinned for that one statement — `USE` on MySQL,
  `SET search_path` on PostgreSQL — so the pool's other connections keep whatever they had. On
  PostgreSQL the database also decides *which pool* the connection comes from.
- A statement that returns rows fills the grid; one that does not reports its affected-row count.
  Which path a statement takes is decided by its leading keyword, read past any leading comments,
  plus a `RETURNING` clause on PostgreSQL, where that is what makes a write answer with rows.

### One Execute runs the whole script

**Execute runs whatever the editor holds**, whether that is one statement or twenty. The text is cut
into the statements it carries and they run in order on the same connection, stopping at the first
the server rejects.

- **Where a statement ends is the engine's**, so the splitter is the driver's: `sqlkit.Split` reads
  the semicolons that are *not* inside a string, a quoted identifier, a line or block comment, or —
  on PostgreSQL — a `$tag$ … $tag$` body, which is how a function carries semicolons of its own. A
  backslash escapes a quote on MySQL and does not on PostgreSQL, so `'C:\'` is one string there and
  a runaway one here. A chunk with no statement in it — trailing whitespace, the comment a script
  ends on — is dropped rather than sent.
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
- **A header's `⋮` menu, and the Filter & Sort panel, sort the result in memory**, since there is no
  statement to re-order: they reach the rows fetched so far and no further, so on a truncated read the
  prefix is sorted rather than the table. Fetching more re-sorts what arrives. A different statement
  drops the sort.
- **Errors surface in the output panel below the editor**, with the code, the server's own wording
  and the engine's error code, never as a dialog. A cancel is reported as `QUERY_CANCELLED`, not as
  a failure.
- Progress shows in two places: `Running…` beside the toolbar buttons, and a spinner on the pager
  under the grid. Previous rows stay on screen until the new ones land.
- **How many rows came back, and how long it took, are on the grid's own status line** — not on a
  strip of their own under the panel. There was one, and it said `325 rows · 5,077 ms` directly
  beneath a line already reading `Record 1 of 325`: two accounts of one read, a row apart. The
  elapsed time went with it, `ElapsedMs` still crossing the wire for whatever wants it next.

## The row limit

There are two row-count controls, and they are shaped by what they size rather than by each other.
The **query toolbar's** is `shared/ui/RowLimit`: a dropdown of presets plus `All` and `Custom…`,
where `Custom…` reveals a number input beside it for any count. Picking `Custom…` seeds that input
with the count already in force and applies it at once, so the number on screen is always the one
that will run; typing commits per keystroke, and clearing the box keeps the last valid count rather
than fighting the typist.

The **grid pager's** is the gear at the end of the pager, and it is a *checkbox* plus a count —
`Limit records`, then how many `records per page`. Lifting the bound is a different kind of read
rather than a bigger one, and a checkbox says that where one more entry in a list of sizes would
have hidden it. The count survives the bound being lifted, so turning it back on restores what it
was; it commits per keystroke for the same reason the dropdown's does.

| Control | Sizes | Default | What it sizes |
|---|---|---|---|
| Query toolbar `Limit` | 100, 1 000, 10 000, or typed | 1 000 | The rows one **Execute** may return — the first prefix, not a ceiling, since the pager can replay for more. Hitting it is reported as a `+` on the status line's total rather than passed off as the whole answer. It sizes the *next* Execute, so a pager that grew past it leaves it alone. |
| Grid pager gear | any typed count | 100 | One page of the grid. Clearing `Limit records` collapses the walk to a single page, so the pager sizes itself by the row total and the arrows go dead — and on a query result it first fetches every row, because otherwise it would only mean all of the prefix. On a **table** tab that read is streamed and watched, below. |

Lifting the bound is deliberately the *only* way to fetch unbounded, so the bounded defaults of
[decisions.md](decisions.md) cannot be lost by accident. It travels the wire as the sentinel
`app.AllRows` (`-1`) from both controls, and `driverRows` translates it into the driver's own
convention of a non-positive limit meaning "read everything". A missing, zero or otherwise
nonsensical count still falls back to `app.DefaultQueryRows` / `app.DefaultPageSize` — the Go zero
value can never mean unbounded.

Two consequences of that convention are worth knowing:

- `buildSelect` in either adapter **drops the `LIMIT … OFFSET …` clause** for an unbounded page,
  which makes its offset moot. `MaxRows` needs no equivalent, since `sqlkit.ScanRows` already reads
  to the end on a non-positive cap. It **binds nothing at all**: a limit and an offset are numbers
  this side counts rather than anything a reader typed, so they are written into the statement, and
  one string is then both what runs and what the status line shows — the only arrangement in which
  the two cannot drift apart.
- There is no ceiling on a typed count in either control. One would be theatre next to an unbounded
  read, and silently answering a different question than the one asked is worse than a slow grid.

### An unbounded read on a table tab is streamed, watched and stoppable

A table of a million and a half rows broke both halves of the first attempt at this: the read outran
the 15-second deadline every other read is given, and the answer would have had to cross the bridge
as one JavaScript string, a bound method's result being delivered to the WebView as exactly that. So
a table read that asked for every row and minted a **run id** behaves differently from every other
read:

- **It has no deadline.** How long every row of a table takes is not knowable ahead of time, and a
  clock that cuts it off is the very failure an unbounded read exists to avoid. `StopTableRows`
  bounds it instead.
- **Its rows arrive in batches.** `LoadTableRows` answers with the columns and the total alone and
  emits the rows as `app.EventRows` — `rows:batch`, `sqlkit.BatchRows` (5 000) rows at a time.
  `useTableData` stitches them back together and shows the grid once the call resolves. It is still
  **one statement and one scan**: the batches are how the answer is delivered, not a walk of
  `LIMIT … OFFSET …` pages, so nothing is counted twice and no row is skipped by a write landing
  mid-read.
- **Its count arrives separately, on a clock.** `app.EventRowsCounted` — `rows:counted`, every
  `sqlkit.CountEvery` (50 ms) — and that separation is the whole reason there are two events. A batch
  fills on a round number, so a count handed over with one would step in thousands however fast the
  read ran; on its own clock it is whatever the read has *truly* reached, so it climbs the way the
  number in any other client does. The last count sent is the exact one, whatever the clock had said
  a moment before.
- **A panel over the grid reports it while it runs** — `shared/ui/FetchProgress`: how long it has been
  going, how many rows have arrived, and `Stop`. It sits *over* the rows rather than replacing them,
  because whatever the grid already held is still true; `Loading` stays out of its way. The clock is
  the panel's own, so counting seconds costs the grid nothing.
- **Stopping keeps every row the read counted** — not only the batches that happened to fill.
  `ScanRows` hands the part-filled batch over on its way out of *any* ending, a stopped read
  included, because the alternative is a count of 60 031 against a grid of 60 000: the read visibly
  losing 31 rows it had already claimed. A stop is a choice and not a failure, so those rows become
  the grid's and the total becomes the number actually held. The columns are the ones already on
  screen, so a *first* read stopped before it answered shows nothing — it had named no columns yet.
  Leaving the tab or changing the limit stops the read too, rather than leaving a scan of a million
  rows emitting batches at a listener that has gone.
- **The count where a stop lands is ahead of the last one shown**, and honestly so: `Stop` has to
  reach the server and the scan only notices between rows, so a read pulling 300 000 rows a second
  gets tens of thousands further in that time. What the grid shows is what was truly read, not what
  the panel last had room to say.

Two limits are worth knowing before pointing this at a very large table. The rows are held **twice**
while they arrive — once as the batches queue for the WebView, once as JavaScript — so memory is the
real bound, which is the trade `All` is asking for. And a **query** result's `All` is unchanged: it
still replays the statement under the ordinary statement timeout, since a script's outcome is one
answer rather than a stream.

### Every statement run is kept

**Execute writes what it ran into a log on disk**, and the `History` pane is that log — see
[the history pane](#the-history-pane). It is kept whatever became of the statement, a rejected one
being among the likeliest to be wanted back, and it is written after the run rather than before, so
what is remembered is what actually happened. A log that cannot be written is dropped rather than
reported: it is not a failure the reader can act on, and certainly not one the query they just ran
should be reported as.

- **The log is one entry per statement, not per Execute.** A statement run again is *moved* to the
  head with its `Runs` counted up, rather than added beside itself — five hundred copies of one
  `SELECT` is not a list anybody reads, and what a reader wants back is the statement. Two entries
  are one when the text, the connection and the namespace all match: the same text against another
  database is another statement, that being where the tables it names live.
- **It holds 500 statements**, oldest dropped first, and lives in `history.json` beside
  `connections.json` in the reader's own configuration directory. The rule for what a log holds
  after one more statement is `domain.Remember`'s and is written down once.
- **The whole of what the reader executed is one entry**, script or single statement. What they
  pressed Execute on is what they want back, and cutting a twenty-statement script into twenty
  entries would bury the nineteen runs before it.
- The outcome kept is the **last** run's: the rows the grid ended up with, how long the whole
  Execute took, and the wording of the statement it stopped at.

## The output panel

The panel under the editor is a tab strip of **`Result`**, **`History`** and, for a script,
**`Messages`**; a further pane — a query plan — is one more entry in the list `QueryOutput` hands
`ResultTabs`. Nothing sits below that strip: what the run did is the pane's own to report, on the
status line under its grid, so `QueryOutput` is the tabs and nothing else.

**An Execute brings its own outcome forward** — `Result`, or `Messages` for a script that stopped —
wherever the reader had got to, and otherwise the strip stays where they left it. `Messages` is
there only for a script, so a single statement run while it was in front puts the reader back on
`Result` rather than on a pane that has gone.

### The history pane

`History` is every statement **this connection** has run, newest first, in a `ResultGrid` like
everything else that shows rows and columns: `When`, `Statement`, `Database`, `Runs`, `Rows`,
`Time (ms)` and `Message`, which reads `OK` or the server's own wording exactly as `Messages` does.

- **A double-click loads a row's statement into the editor**, which is what the log is for; the grid
  is read-only, so a click has nothing else to mean. It brings **the namespace it ran in** with it,
  because a statement names tables of one database and one run against another would fail on names
  that are not there — the toolbar's pickers move with it, visibly. The editor's own undo is what
  puts back whatever was being typed.
- **It is searched by the Filter & Sort panel every grid has**, in memory, there being no statement
  of its own to narrow: finding one statement among five hundred is a `contains` on the `Statement`
  column, and the footer counts what the conditions left of the log. `When` is spelled
  `2026-08-04 14:23:05` — local time, in the one spelling that also sorts as text.
- **The pane is read on mount rather than held in a store.** It is drawn only while the reader is
  looking at it, so every open is a fresh log and an Execute never has to tell it anything.
- **The log is every connection's and the pane shows one.** A statement of another server names
  tables that are not here, so it would be noise at best and unrunnable at worst.
- **`Clear History` forgets every statement**, behind the same confirmation a drop is behind, and it
  says that nothing on any server changes. It sits over the rows rather than in the footer, because
  it acts on the whole log where the footer acts on a row. There is no way to forget one entry: a log
  is what it is, and a reader who wants it gone wants it gone.

Under the pane sits the same `GridFooter` the table view has, with the source of the rows named on
its right — the qualified table, or why the result is read-only:

- **The editing icons show only for a writable result.** For one that no `UPDATE` can name they are
  not there at all; `Refresh` and `Stop` remain.
- **One click on a cell opens it** (or `Enter` on the focused one). `Esc` abandons the input and
  gives the keyboard back to the grid. Edits are held locally and marked in the grid, so a mistyped
  value costs nothing.
- ✗ drops every pending change, ✓ writes them — edits with the same `UPDATE … WHERE <primary key>`
  the table view uses, then inserts, then deletions — and reloads. ↻ reloads alone.
- **`Refresh` and `Apply` replay the statement that produced the rows**, not whatever the editor
  holds now, so editing the SQL after a run cannot make Apply execute something else.

## Filtering and sorting

There are **two ways in**, and they hold the same conditions:

- **The Filter & Sort button**, over every grid — its own head strip on a query result, the table
  view's toolbar beside `Cell Editor` and `Column`. It opens `FilterSortPanel`, which is where a
  condition is *built*: a tick box, a column, an operator and a value, each of the three parts
  opening the list of what it may be. `+` adds a condition beside the row and `()+` adds a **group**,
  which is parentheses joining its children by `and` or by `or` — the word between the rows is a
  button that flips the whole group, so `a AND (b OR c)` is sayable and not merely wishable.
  **Sort By** under it is a list of columns and directions, the first deciding and the rest breaking
  its ties. Nothing narrows anything until **Apply Filter & Sort**; the panel says **Criteria
  Edited** while the draft differs from what is in force. The whole of it is in
  [grid.md](grid.md#the-filter--sort-panel).
- **Right-clicking a cell**, where a condition is *picked* rather than built. The menu is one object —
  [grid.md](grid.md) lists every item — and **Filter ▸** offers every operator against the value under
  the pointer, so a condition read off a cell is never typed twice; it applies **at once**, having
  been picked whole. **Show/Hide Filter & Sort** opens the panel, and **Remove All Filters & Sorts**
  clears the lot, the sort included. Hiding the panel does not widen the grid — the conditions hold
  either way, which is what the count on the button is for.

**Twenty operators**: `=`, `≠`, `<`, `≤`, `>`, `≥`, `contains`, `does not contain`, `begins with`,
`does not begin with`, `ends with`, `does not end with`, `is NULL`, `is not NULL`, `is empty`,
`is not empty`, `is between`, `is not between`, `is in list`, `is not in list`. How many values one
takes is its own business — none, one, two, or any number — and a condition short of what its
operator needs narrows nothing until it is filled in. `is empty` asks about the empty string and
`is NULL` about the absence of a value; they are different questions.

**Each part opens a small panel under itself, never a dialog over the grid** — the table stays in
sight while it is being narrowed. The **column** panel and the **value** panel are both searched
rather than scrolled, since a table has as many columns as it likes and a column as many values as
its rows.

**A value is picked from the column's own values.** Clicking the value part lists what the column
actually holds — `LoadColumnValues`, a `SELECT DISTINCT … ORDER BY 1 LIMIT 200` — with a box beside
it for a value the list has not got. NULL is not offered there: `is NULL` is the way to it. There is
no OK on the panel: what it changes is the draft, and **Apply Filter & Sort** is the one commit.

**Ticking more than one value makes it `is in list`.** The rows are check boxes, and the operator
changes as the second one is ticked — `Name = Java developer` becoming
`Name is in list (Java developer, JavaScript)` — so what is on screen is always what will run;
a negative operator becomes `is not in list`, and unticking back to one gives the old operator back
while the panel is still open. The four comparisons that name a **bound** (`<`, `≤`, `>`, `≥`) are
single-pick instead: `> a` and `> b` are one comparison, not both.

**Where the filtering happens depends on where the rows come from**, exactly as with the sort. A
**table** tab filters *on the server*: the tree travels in `TablePageRequest`, becomes the read's
`WHERE`, and the **total is counted through it too**, so the pager walks the pages that are actually
there and the status line's statement shows the clause that ran. A **query result** filters the rows
it already holds, since it has no statement of its own to narrow, and so a filter there reaches only
what was fetched — the same bound its in-memory sort has. A draft row is never filtered away; it is
a row still being filled in.

The operator crosses the wire as a **token** (`gt`, `contains`, …) and is spelled into SQL by the
driver, so nothing a reader typed arrives as an operator; a conjunction crosses as `and` or `or` and
nothing else. The *values* are written into the statement as string literals rather than bound, for
the same reason the `LIMIT` is: one string is then both
what ran and what the status line shows and copies — a statement carrying `?` is not one a reader
could paste into an editor. `sqlkit.Dialect.QuoteString` is what stands between the text and the
statement, and both engines read a string literal back into a column of any type. Two smaller
consequences: the six text matches compare the column **as text** (`CAST(… AS CHAR)` / `AS TEXT`), so
they work on a number or a date as well as a name, and a `%` the reader typed is their own wildcard.
In memory those matches ignore case; on the server the collation decides, which is MySQL ignoring it
and PostgreSQL not.

## Exporting a grid

An **Export** button sits **in the strip over every grid, against its right edge** — its own head
strip on a query result, and the table view's toolbar beside `Cell Editor` and `Column`, one strip
over a table rather than two — and opens **CSV**, **JSON**
and **SQL**; each writes to a file picked in the system's own save dialog — so the file lands where the
reader put it, named after the table, and nothing is written if the dialog is dismissed. It is above
the rows rather than in the footer below them because it acts on the table as a whole, where the
footer acts on a row.

**What is written is what the grid holds, in the order it draws it.** A query result exports its
whole result rather than the page in front, since a reader who paged still means the result; a table
tab exports the page, because a table's rows are paged by the *server* and the page is all the grid
has. A hidden column is not exported: it is one nobody asked for.

**The cell menu's Save Data As ▸ is the same three formats**, so a reader who is already in the menu
need not travel to the button; **Copy As ▸** beside it puts the selection on the clipboard in the
clipboard's own shapes — tab-separated with field names, CSV, JSON — and treats a `NULL` the way the
file of that format does.

| Format | What it does with a `NULL` |
|---|---|
| CSV | An empty field, which is the only thing a CSV has to say "nothing" with |
| JSON | `null`, which is why a reader who needs to tell it from an empty string wants this one |
| SQL | The `NULL` keyword. Every other value is written as a literal, which both engines read back into a column of any type. The statements are asked of the driver, so the quoting is the engine's; a result no single table can be traced to is named `exported_rows`, which the reader has to correct — the honest outcome, rather than a file that looks right |

### When a result is writable

A result is writable only when the driver can trace its rows to one stored row each.
`sqlkit.Dialect.ParseSource` accepts a plain `SELECT … FROM <table>` with at most a `WHERE`,
`ORDER BY`, `LIMIT` or `OFFSET` after it, and rejects joins, unions, `GROUP BY`, `DISTINCT`, derived
tables and `EXPLAIN`. The named table is then looked up in the catalogue, and the result stays
read-only unless it carries every primary-key column exactly once — an aliased or repeated key
column names no row. The footer says which case applies. This is needed because `database/sql`
reports a column's type but not the table it came from, so the source cannot be read off the wire.

One `Dialect` field decides how a quoted name is read, so `` `shop`.`users` `` is a table on MySQL
and a string on PostgreSQL, where `"shop"."users"` is the table instead. The qualifier the statement
carries is read as whatever the engine's inner level is: a database on MySQL, a schema on
PostgreSQL, whose statements cannot name another database at all.

## Options

The gear at the right of the command bar opens **Options** (`features/settings/SettingsDialog`): a
rail of sections on the left, the one in force on the right, which is the shape a reader already
knows a settings dialog by. Sections are added to that rail — never a second dialog.

| Section | Holds |
|---|---|
| General | **Theme** — dark or light, the same switch the command bar's sun/moon is. |
| Editor | The **data font** — family, size, weight, with a preview line — and **word wrap** for the SQL editor, which reconfigures a live editor through a CodeMirror compartment rather than rebuilding it. |
| Records | What a **new tab** starts with: records per page (a table read and a query result alike) and the row limit a query tab caps its read at. Both are defaults, read once when a tab mounts, so changing one never moves a result the reader has already paged through. |

**A pick applies as it is made**, not behind an OK: the app behind the dialog is the preview, and a
dialog that could be dismissed half-set would need a draft of every preference to do it. The footer
therefore offers **Restore Defaults** and **Done**, and nothing that cancels. Everything is kept in
`localStorage` under one key, `vibe-db.settings`, by `stores/settings.ts` — the one place a
preference is saved and put in force.

### The font list is the machine's

The family list is **every font installed on this machine**, not a list the app ships: `SystemAPI.
ListFonts` reads the Windows font registry — the machine-wide hive *and* the reader's own, since a
font installed for one user is registered only there — strips the `(TrueType)` tag, folds a family's
`Regular`, `Italic`, `Bold` and `Oblique` variants back into it, and answers sorted. `Segoe UI Light`
and `Arial Black` survive whole, because those are families a stylesheet can name; `Arial Bold` is
Arial asked for in bold and is folded away. A few hundred families is normal, so the list is
**searched rather than scrolled**, and every row is drawn in its own family — the only honest preview
of a pick. Outside the desktop shell (a browser dev run) the host answers with nothing and the picker
offers the six families in `FALLBACK_FAMILIES`.

The default is **Cascadia Mono light 13 px**: a screenful of rows reads heavier than a paragraph of
the same weight, so data is set a weight below regular. A weight only draws as itself where the
family has it — Cascadia is variable and does, Consolas and Lucida Console ship 400 and 700 alone, and
Windows then rounds to the nearest they have. Sizes run 10–16 px, which is what a 28 px row can hold.

The pick is written onto the root element as `--font-data`, `--font-data-size` and
`--font-data-weight`, ahead of `--font-data-fallback`, which is why nothing rebuilds, why a family the
machine turns out not to have still lands on something monospaced, and why a column fit measures the
font the reader actually chose.

## Grid behaviour

Every table in the app is one component, `ResultGrid`, and it looks and behaves the same wherever it
appears: a two-line header, drag-selected cell ranges, in-place editing, resizable columns.
**[grid.md](grid.md) is the specification** — read it before changing any table, and add to it rather
than to a call site.
