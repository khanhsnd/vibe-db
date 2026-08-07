# Features and UI behaviour

Working: connection profiles (create / edit / delete) for **MySQL, PostgreSQL, SQLite, SQL Server,
Oracle, MongoDB and Redis**,
**over an SSH tunnel, with TLS held to a certificate this machine trusts, and signing in by AWS or
Google cloud identity as well as by password**, Test Connection
with server version and latency, connect / disconnect, **an environment label that colours a
connection everywhere and stands between the reader and a statement they cannot take back**,
**laying a script out and warning about it before it runs**, **a value viewer that reads a cell as
the JSON, GeoJSON, array or blob it holds**, **creating and dropping a database**,
**tables, views and routines listed per namespace**, **a table list of what the catalogue keeps about
each table**, **a table's fields, indexes and foreign keys in
the tree and in a designer tab**, **schema changes from the tree's context menu** — new / rename /
empty / truncate / drop a table, add / edit / rename / drop a field, add or drop an index or a
foreign key, drop a view or a routine — **reading and writing the definition of a view, a function
or a procedure**, table data browsing with pagination, cell editing plus row
insert and delete on any grid whose rows can be traced to one table, **exporting a grid to CSV, JSON
or SQL**, **backing a SQL namespace up into a folder the app keeps, and restoring
one**, query tabs with completion that **run a whole script**, **a console that runs one statement
at a time and keeps the transcript**, **running a file of statements against a connection**,
**marking a connection with a colour that washes down the sidebar**, **a history of every statement
run, kept on disk and put back in the editor by a double-click**, **a log of everything the window
has run, across every connection, on one clock**, passwords in Windows Credential
Manager, and the full layout shell with dark and light themes and a **data font the reader picks**.

Not there yet: a **materialised view** is not listed, because it is dropped by a statement of its
own and a list that cannot be acted on is worse than one that does not claim it. A routine is
declared and edited but not **run** with its arguments filled in — that is a query tab's business.
A MongoDB view has no definition to open, being declared by a document rather than by a statement.
A column's **comment** is neither read
nor written, so the designer has no column for one rather than a column that quietly drops what is
typed into it. An export writes what the grid holds rather than replaying the statement for
everything behind it.

## One feature to a file

This page is the index and holds no behaviour of its own. Each feature is [its own
file](features/), so a change lands in one place and a reader is never scrolling past four features
to reach the one they came for. How the docs are laid out is written down in
[CLAUDE.md](../CLAUDE.md#how-the-docs-are-written).

| Feature | Read |
|---|---|
| Picking an engine, filling a profile in, TLS, SSH, the environment guard, a connection's colour | [features/connections.md](features/connections.md) |
| What each of the seven engines has, and what it has not | [features/engines.md](features/engines.md) |
| The tree, what a level loads, filtering it, and the table list | [features/schema-tree.md](features/schema-tree.md) |
| A Redis key space: the grid of keys, the prefix tree, the search bar, the key panel | [features/key-space.md](features/key-space.md) |
| Finding text across a schema | [features/search.md](features/search.md) |
| Comparing two tables, and making them agree | [features/compare.md](features/compare.md) |
| The tree's schema changes, `New Database`, and the designer | [features/schema-changes.md](features/schema-changes.md) |
| A view's or a routine's definition: reading it, declaring one, and saving it back | [features/definitions.md](features/definitions.md) |
| Dumping a table or a schema, and the Import and Export wizards | [features/transfer.md](features/transfer.md) |
| Backing a namespace up, the list of backups, and restoring one | [features/backup.md](features/backup.md) |
| The table tab: opening it, paging it, and its two panels | [features/table-tab.md](features/table-tab.md) |
| Query tabs, Explain, a MongoDB tab, a Redis tab, running a whole script | [features/query-tab.md](features/query-tab.md) |
| `Format`, what the editor says before anything runs, and the log every statement is kept in | [features/script.md](features/script.md) |
| The console, and Execute SQL File | [features/console.md](features/console.md) |
| The History Log: every statement the window has run, on one clock | [features/log.md](features/log.md) |
| The two row-limit controls, and the streamed unbounded read | [features/row-limit.md](features/row-limit.md) |
| The output panel, the history pane, editing a result, exporting a grid | [features/results.md](features/results.md) |
| Filtering and sorting a table tab | [features/filter-sort.md](features/filter-sort.md) |
| Reading a value whole, and why bytes arrive as `\x…` | [features/value-viewer.md](features/value-viewer.md) |
| The menu bar, the dividers, the tab strip, Options and the font list | [features/shell.md](features/shell.md) |

## Grid behaviour

Every table in the app is one component, `ResultGrid`, and it looks and behaves the same wherever it
appears: a two-line header, drag-selected cell ranges, in-place editing, resizable columns.
**[grid.md](grid.md) is the specification** — read it before changing any table, and add to it rather
than to a call site.
