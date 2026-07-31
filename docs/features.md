# Features and UI behaviour

Working: connection profiles (create / edit / delete) for **MySQL and PostgreSQL**, Test Connection
with server version and latency, connect / disconnect, database listing, table listing per database,
table data browsing with pagination and single-cell editing, query tabs with completion, passwords
in Windows Credential Manager, and the full layout shell with dark and light themes.

Not there yet: schema browsing stops at tables — views, procedures and functions are not listed.
One Execute runs one statement; a pasted multi-statement script is rejected by the server, since
`multiStatements` stays off and the client-side splitter is not written yet.

## Picking an engine

The connection dialog leads with `Engine`, and everything that follows takes its defaults from it:
port 3306 and user `root` for MySQL, 5432 and `postgres` for PostgreSQL. Switching engine rewrites
only the fields that still hold the old engine's default, so a typed port survives the switch. The
tree tags every connection with its engine, and the editor loads that engine's SQL dialect for
highlighting and completion.

One field differs rather than just defaulting: **PostgreSQL needs a database**. A session is bound to
one for life, so that is the database the connection bootstraps from — the one its database list is
read in. Reaching any other database dials it separately, which the adapter does on first use.

## Browsing a schema

The tree has as many levels as the engine does:

```
MySQL         connection → database → Tables → table
PostgreSQL    connection → database → schema  → Tables → table
```

Each level loads on first expand and is cached until disconnect: a PostgreSQL database loads its
schemas, and whichever node owns the `Tables` folder loads its table list. Clicking the node that
owns tables — a MySQL database, a PostgreSQL schema — marks it the active namespace for that
connection, which is what a new query tab inherits.

The query toolbar mirrors the same shape: one `Database` picker on MySQL, and a `Schema` picker
beside it on PostgreSQL. Changing the database clears the schema, since a schema of one database
names nothing in another.

Clicking a table opens a data tab: 100 rows per page by default, ordered by primary key so paging
is stable. The pager's `Rows` control sizes the page and can lift the bound entirely — see
[the row limit](#the-row-limit). Above the grid sit two independent toggles, since they occupy
different edges:

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft,
  Apply issues `UPDATE … WHERE <primary key>` and reloads, Refresh reloads. A table with no primary
  key is read-only, because no single row can be identified. MySQL adds a `LIMIT 1` belt to the
  braces; PostgreSQL has no such clause and does not need one, since the key already names one row.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes.

## Query tabs

A query tab carries its own toolbar, editor buffer, database and row limit, so several tabs can sit
on the same connection without treading on each other. The toolbar picks the database, then
**Execute** (`F5` or `Ctrl`+`Enter`) or **Explain** runs the statement; **Stop** cancels it.

- The namespace is applied on a connection pinned for that one statement — `USE` on MySQL,
  `SET search_path` on PostgreSQL — so the pool's other connections keep whatever they had. On
  PostgreSQL the database also decides *which pool* the connection comes from.
- A statement that returns rows fills the grid; one that does not reports its affected-row count.
  Which path a statement takes is decided by its leading keyword, read past any leading comments,
  plus a `RETURNING` clause on PostgreSQL, where that is what makes a write answer with rows.
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
| Query toolbar `Limit` | 100, 1 000, 10 000 | 1 000 | The rows one Execute may return. Hitting the limit is reported in the status strip rather than passed off as the whole answer. |
| Table pager `Rows` | 100, 500, 1 000 | 100 | One page of the offset/limit walk. `All` collapses the walk to a single page, so the pager sizes itself by the row total and the arrows go dead. |

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

Inside the pane sits a toolbar naming what is on screen, and the buttons that write it back:

- **Selecting a row reveals `Discard`, `Apply` and `Refresh`.** They are hidden until then, since
  none of them has a subject without one.
- **Double-click a cell to edit it** (or press `Enter` on the selected cell). `Esc` abandons the
  input. Edits are held locally and marked in the grid, so a mistyped value costs nothing.
- `Discard` drops the pending edits, `Apply` writes each one with the same
  `UPDATE … WHERE <primary key>` the table view uses and reloads, `Refresh` reloads alone.
- **`Refresh` and `Apply` replay the statement that produced the rows**, not whatever the editor
  holds now, so editing the SQL after a run cannot make Apply execute something else.

### When a result is writable

A result is writable only when the driver can trace its rows to one stored row each.
`sqlkit.Dialect.ParseSource` accepts a plain `SELECT … FROM <table>` with at most a `WHERE`,
`ORDER BY`, `LIMIT` or `OFFSET` after it, and rejects joins, unions, `GROUP BY`, `DISTINCT`, derived
tables and `EXPLAIN`. The named table is then looked up in the catalogue, and the result stays
read-only unless it carries every primary-key column exactly once — an aliased or repeated key
column names no row. The toolbar says which case applies. This is needed because `database/sql`
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
