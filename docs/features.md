# Features and UI behaviour

Working: connection profiles (create / edit / delete) for **MySQL and PostgreSQL**, Test Connection
with server version and latency, connect / disconnect, database listing, table listing per database,
**a table's fields, indexes and foreign keys in the tree and in a designer tab**, **schema changes
from the tree's context menu** — new / rename / empty / truncate / drop a table, add / edit / rename
/ drop a field, drop an index or a foreign key — table data browsing with pagination, cell editing
plus row insert and delete on any grid whose rows can be traced to one table, query tabs with
completion, passwords in Windows Credential Manager, and the full layout shell with dark and light
themes.

Not there yet: schema browsing stops at tables — views, procedures and functions are not listed, and
a *new* index or foreign key can only be made from a query (dropping one has a menu item; creating
one does not). A column's **comment** is neither read nor written, so the designer has no column for
one rather than a column that quietly drops what is typed into it. One Execute runs one statement; a
pasted multi-statement script is rejected by the server, since `multiStatements` stays off and the
client-side splitter is not written yet.

## Picking an engine

The connection dialog leads with `Engine`, and everything that follows takes its defaults from it:
port 3306 and user `root` for MySQL, 5432 and `postgres` for PostgreSQL. Switching engine rewrites
only the fields that still hold the old engine's default, so a typed port survives the switch. The
editor loads that engine's SQL dialect for highlighting and completion.

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
MySQL         connection → database → Tables → table → Fields    → field
PostgreSQL    connection → database → schema  → Tables → table → Indexes     → index
                                                                → Foreign Keys → foreign key
```

Each level loads on first expand and is cached until disconnect: a PostgreSQL database loads its
schemas, whichever node owns the `Tables` folder loads its table list, and a table loads its shape —
fields, indexes and foreign keys in one read, since expanding any of the three folders needs all
three counts. Clicking the node that owns tables — a MySQL database, a PostgreSQL schema — marks it
the active namespace for that connection, which is what a new query tab inherits.

A field carries its **bare** type as a badge and a key icon when it is part of the primary key; an
index carries its columns, and a foreign key the table it points at. The badge is the bare type and
not the declared one because six levels of indent leave no room for `character varying(255)` — the
whole of it is in the row's tooltip, and in the designer.

**One click selects, two clicks open** — see [frontend.md](frontend.md). Applied to the tree:

| Gesture | What happens |
|---|---|
| Click a row | Selects it, and names the active namespace if that row owns tables. Reads nothing. |
| Click the caret | Selects the node **and** opens or closes it, loading that level on first open. |
| Double-click a row | The same open — except on a **table**, which opens a data tab. A table has folders under it now, but double-clicking one has always meant "show me the rows", and the caret is what expands. |
| Open a closed connection | Dials it, whichever gesture asked. A disconnected connection node still carries an empty child list, so expanding it would otherwise spin the caret over nothing. The store expands the node when the session lands, and a failed connect leaves the caret shut. |

## Changing a schema

Every schema change is raised from a right-click in the tree, and every one of them is confirmed or
filled in before it runs. `useTreeMenu` is the single place the items are built, per node kind:

| Right-click | Items |
|---|---|
| A connection | `Connect` / `Disconnect`, `Edit Connection…`, `Delete Connection` |
| A PostgreSQL database | `Refresh` — it holds schemas, not tables |
| The node owning tables, or its `Tables` folder | `New Table`, `Refresh` |
| A table | `Open Table`, `Design Table`, `New Table`, `Rename Table…`, `Empty Table`, `Truncate Table`, `Delete Table`, `Refresh` |
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
to work in and leaves the tree beside it readable. Only the two requests that are a single answer
stay modal: `Rename …`, which is one box, and a confirmation, which has to be answered before
anything else happens.

`Add Field` from the tree opens the designer with a blank field already appended, which is the one
thing a tab cannot say by being opened. It travels as a slot in the schema store rather than a field
of the tab, because the tab may already be open and a revealed tab has to hear the second ask too.

### The designer

`Design Table` opens a tab laid out the way a table designer is: a toolbar of `Save`, `Add Field`,
`Delete Field`, `Primary Key` and `Refresh`, over a strip of `Fields`, `Indexes`, `Foreign Keys` and
`Changes`. **`Save` takes the accent the moment there is anything to save** and drops back to a
plain toolbar button when there is not, and `Delete Field` carries the danger tone, so the two
buttons that matter are readable before the labels are. `New Table` is the same tab with no table behind it yet and a name box in the toolbar;
its `Save` creates the table, and the tab then goes on designing it rather than offering to create
it twice.

- **Every list is a `ResultGrid`**, because they are rows and columns — see [grid.md](grid.md). The
  Fields grid is **edited in place**: double-click a cell, or press `Enter` on it.
- Its columns are the parts a field is *declared* from, not the string the server reports back:
  `Name`, `Type`, `Length`, `Decimal`, `Not null`, `Key`, `Default`. Length and decimals are parts
  of the type and travel as one string — `varchar(255)`, `numeric(10,2)` — and read as `0` when the
  type declares none. `Not null` and `Key` take `YES` (or `y`, `true`, `1`); anything else, a
  cleared cell included, is no. A `Default` is an **expression**, which is what a server-side
  default is: `now()`, `'IN_PROGRESS'`, `0`.
- `Primary Key` moves the key onto the selected fields, or off them if they all already hold it.
  Both engines are made to replace a key rather than add a second one, and dropping the key of a
  table that has none is not an error.
- **Nothing is sent until `Save`.** A pending cell carries the grid's warning ring, and a field the
  server has not seen is ringed whole. `Changes` lists what `Save` will do, in the order it will do
  it: drops, then adds, then a rename before the alter that follows it — so renaming and retyping a
  field in one sitting is one trip and the alter can still name the column. It reads as plain
  changes and not as SQL, because how a statement is spelled belongs to the driver and this side
  has never seen one.
- `Refresh` re-reads the table and drops whatever was staged. Nothing in the Indexes or Foreign Keys
  grids is ever `(Null)`, and neither is a missing length: a type with no length declared none, and
  `(Null)` would claim a column defaults *to* null.

The query toolbar mirrors the same shape: one `Database` picker on MySQL, and a `Schema` picker
beside it on PostgreSQL. Changing the database clears the schema, since a schema of one database
names nothing in another.

**Double-clicking** a table opens a data tab; a single click only selects it, and reads nothing from
the server. A tab loads 100 rows per page by default, ordered by primary key so paging is stable —
until a header's `⋮` menu sorts it by a column of its own, which travels into the read's `ORDER BY`
and therefore orders the whole table rather than the page on screen.
**Paging lives on the right of the bar under the grid**, beside the `Rows` control that sizes it and
can lift the bound entirely — see [the row limit](#the-row-limit). The left of that bar is where rows
are added, deleted, applied and discarded: it is the same `GridFooter` every grid has, and
[grid.md](grid.md) specifies it. Above the grid sit two independent toggles, since they occupy
different edges:

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft and
  Apply stages the value on the cell, exactly as double-clicking it in the grid does — the footer's
  ✓ is what writes `UPDATE … WHERE <primary key>` and reloads. A table with no primary key is
  read-only, because no single row can be identified. MySQL adds a `LIMIT 1` belt to the braces;
  PostgreSQL has no such clause and does not need one, since the key already names one row.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes. **Right-clicking a column
  header hides or restores columns too**, alongside the copy and paste items every grid's header menu
  has — one hidden set, either way in. [grid.md](grid.md) specifies that menu.

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
- **Rows are paged under the grid by the same pager the table view has.** What a read returned is a
  *prefix* of the result, and the pager cuts pages out of it in memory — turning to a page it
  already holds reads nothing. A total shown as `1,000+` says the prefix stopped at the toolbar's
  `Limit`, and that is also what keeps `Next` alive on the last page: asking for a page past the
  prefix, or picking `Rows` `All`, **replays the statement for as many rows as that needs**. So `All`
  loads all, and paging is never stuck at the limit. Two things follow from the replay: a statement
  with no `ORDER BY` may reshuffle what lands on a page, and the reader keeps their page number
  across a read of the same statement — only a different statement sends them back to page one.
- **A header's `⋮` menu sorts the result in memory**, since there is no statement to re-order: it
  reaches the rows fetched so far and no further, so on a truncated read it sorts the prefix rather
  than the table. Fetching more re-sorts what arrives. A different statement drops the sort.
- **Errors surface in the output panel below the editor**, with the code, the server's own wording
  and the engine's error code, never as a dialog. A cancel is reported as `QUERY_CANCELLED`, not as
  a failure.
- Progress shows in two places: a spinner beside the toolbar buttons and a `Running…` status strip
  under the output panel. Previous rows stay on screen until the new ones land.

## The row limit

Both row-count controls are the same component, `shared/ui/RowLimit`: a dropdown of presets plus
`All` and `Custom…`, where `Custom…` reveals a number input beside it for any count. Picking
`Custom…` seeds that input with the count already in force and applies it at once, so the number on
screen is always the one that will run; typing commits per keystroke, and clearing the box keeps the
last valid count rather than fighting the typist.

| Control | Presets | Default | What it sizes |
|---|---|---|---|
| Query toolbar `Limit` | 100, 1 000, 10 000 | 1 000 | The rows one **Execute** may return — the first prefix, not a ceiling, since the pager can replay for more. Hitting it is reported as a `+` on the pager's total rather than passed off as the whole answer. It sizes the *next* Execute, so a pager that grew past it leaves it alone. |
| Grid pager `Rows` | 100, 500, 1 000 | 100 | One page of the grid. `All` collapses the walk to a single page, so the pager sizes itself by the row total and the arrows go dead — and on a query result it first fetches every row, because otherwise `All` would only mean all of the prefix. |

`All` is deliberately the *only* way to fetch unbounded, so the bounded defaults of
[decisions.md](decisions.md) cannot be lost by accident. It travels the wire as the sentinel
`app.AllRows` (`-1`) for both controls, and `driverRows` translates it into the driver's own
convention of a non-positive limit meaning "read everything". A missing, zero or otherwise
nonsensical count still falls back to `app.DefaultQueryRows` / `app.DefaultPageSize` — the Go zero
value can never mean unbounded.

Two consequences of that convention are worth knowing:

- `buildSelect` in either adapter **drops the `LIMIT … OFFSET …` clause** for an unbounded page,
  and therefore returns its arguments alongside the statement — an offset with no limit binds
  nothing. `MaxRows` needs no equivalent, since `sqlkit.ScanRows` already reads to the end on a
  non-positive cap.
- There is no ceiling on a typed count in either control. One would be theatre next to an `All`
  option, and silently answering a different question than the one asked is worse than a slow grid.

## The output panel

The panel under the editor is a tab strip whose only pane so far is **`Result`**; a further pane —
messages, a query plan, history — is one more entry in the list `QueryOutput` hands `ResultTabs`.
The run status strip stays below the strip, because it describes the run rather than any one pane.

Under the pane sits the same `GridFooter` the table view has, with the source of the rows named on
its right — the qualified table, or why the result is read-only:

- **The editing icons show only for a writable result.** For one that no `UPDATE` can name they are
  not there at all; `Refresh` and `Stop` remain.
- **Double-click a cell to edit it** (or press `Enter` on the selected cell). `Esc` abandons the
  input. Edits are held locally and marked in the grid, so a mistyped value costs nothing.
- ✗ drops every pending change, ✓ writes them — edits with the same `UPDATE … WHERE <primary key>`
  the table view uses, then inserts, then deletions — and reloads. ↻ reloads alone.
- **`Refresh` and `Apply` replay the statement that produced the rows**, not whatever the editor
  holds now, so editing the SQL after a run cannot make Apply execute something else.

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

## Grid behaviour

Every table in the app is one component, `ResultGrid`, and it looks and behaves the same wherever it
appears: a two-line header, drag-selected cell ranges, in-place editing, resizable columns.
**[grid.md](grid.md) is the specification** — read it before changing any table, and add to it rather
than to a call site.
