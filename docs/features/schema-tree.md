# The schema tree

[← Features and UI behaviour](../features.md)

## Browsing a schema

The tree has as many levels, and as many folders, as the engine does:

```
MySQL         connection → database →          Tables       → table      → Fields       → field
PostgreSQL    connection → database → schema →  Views        → view              Indexes      → index
                                                Functions    → function          Foreign Keys → foreign key

MongoDB       connection → database →          Collections  → collection → Fields  → field
                                                Views        → view              Indexes → index
```

**A folder for something the engine has not got is not drawn.** MongoDB has no stored routines —
server-side JavaScript was removed from the server — and no referential constraints of any kind, so
neither folder appears rather than appearing permanently empty. `EngineInfo.hasRoutines` and
`hasForeignKeys` are what say so, and a component never compares against an engine's name.

**What a thing is called is the engine's too.** A collection is not a table and a document is not a
row, so `engineWords` supplies the nouns for the tree's folder, the right-click menu, the object list
and every confirmation. Only the wording changes: a `NodeKind` is still `tables` and the bound method
is still `DropTable`, so one name means one thing on the wire whatever it says on the screen. The
warning on **Truncate** is why it matters — on a table store it warns about a foreign key, and a
document store cannot have one, which is also why MongoDB is offered `Empty` alone: there is no
storage to reset, so truncating and emptying would be one command under two names.

Each level loads on first expand and is cached until disconnect: a PostgreSQL database loads its
schemas, whichever node owns the object folders loads **its tables, views and routines in one
read**, and a table loads its shape — fields, indexes and foreign keys, also in one read. The reason
is the same both times: expanding any one folder needs every folder's count, so one read answers all
three — which is why that read happens on **the namespace's own expand** and not on its folders':
the three folders arrive with their counts already on them, and there is no second click that only
fills in numbers. Clicking the node that owns tables — a MySQL database, a PostgreSQL schema, a
MongoDB database — marks it the active namespace for that connection, which is what a new query tab
inherits, and the row is drawn in the success colour to say so.

**A namespace's badge is how much is in it, and a Redis database's is how many keys.** On every other
engine that number is what the read already listed — tables, views and routines added up — but a key
space lists one fixed table, so counting the list would say `1` of every database on the server. The
count is the server's own `DBSIZE`, arriving with that same read, so the reader can tell which of the
sixteen databases has anything in it. See [architecture.md](../architecture.md#the-key-value-store).

**A MongoDB field is inferred, not declared.** A collection says nothing about what it holds, so the
Fields folder is the fields the *documents* were found to carry — a sample of the collection's first
few hundred, `_id` always among them. What that costs, and where it shows, is in
[grid.md](../grid.md#where-the-columns-come-from).

A **view opens its rows in the same tab a table does**, and lands there read-only: no column of a
view is a primary key, so no single stored row can be named. A **function or a procedure** carries
its argument list as its badge, because on PostgreSQL two of them may share a name and be told apart
only by what they take — which is also why dropping one sends the whole routine and not its name.

A field carries its **bare** type as a badge and a key icon when it is part of the primary key; an
index carries its columns, and a foreign key the table it points at. The badge is the bare type and
not the declared one because six levels of indent leave no room for `character varying(255)` — the
whole of it is in the row's tooltip, and in the designer.

**One click selects, two clicks open** — see [frontend.md](../frontend.md). Applied to the tree:

| Gesture | What happens |
|---|---|
| Click a row | Picks it **alone**, and names the active namespace if that row owns tables. Reads nothing — except on the **`Tables` folder**, which also opens [the table list](#the-table-list): what that folder *is* to a reader is the tables and their numbers, and the tree has room for the names alone. |
| **`Ctrl`/`Cmd`-click a row** | Picks it **beside** whatever is picked already, or takes it back out. It reads nothing and travels nowhere: picking a second table is not opening it. Two tables picked this way is the whole precondition for [Compare](compare.md#comparing-two-tables-and-making-them-agree). |
| Click the caret | Picks the node **and** opens or closes it, loading that level on first open. |
| Double-click a row | The same open — except on a **table**, which opens a data tab. A table has folders under it now, but double-clicking one has always meant "show me the rows", and the caret is what expands. |
| Right-click a row | The menu, on a pick it left alone if it landed **inside** one of several — so a pair can be acted on whole — and on that row alone otherwise. It is the rule the grid's own cell menu already sets. |
| Open a closed connection | Dials it, whichever gesture asked. A disconnected connection node still carries an empty child list, so expanding it would otherwise spin the caret over nothing. The store expands the node when the session lands, and a failed connect leaves the caret shut. |

**Every picked row is washed, and the focused one is bold** — the two depths the grid's own selection
has, and for the same reason: everything that acts on one row acts on the last press, and Compare
acts on two. `Ctrl` **toggles** here where the grid's only ever adds, because a rectangle cannot be
split and a list of rows can — taking the wrong table back out of a pair is the commonest correction
there is.

### Filtering the tree

**Filter objects** narrows the tree to the rows whose name holds what was typed, case-insensitively,
and **opens every folder above a hit** so the hit is on the screen rather than behind a caret.
`searchTree` is the whole of it, and three things follow from where it sits:

- **A folder never matches.** `Tables` is under every namespace, so a filter that matched it would
  answer every query with the whole tree. Only objects match — a connection, a database, a schema, a
  table, a view, a routine, a field, an index, a foreign key.
- **A hit keeps its own children shut.** The reader asked for that row, not for everything under it.
  Their own expansion is untouched by the search and is what is left when the box is emptied —
  `Escape`, or the cross in the box, empties it.
- **Only what has been read is searched**, the tree being filled a level at a time. A namespace
  nobody has opened holds no names yet, so nothing in it can be found; the empty state says exactly
  that rather than claiming the table does not exist.

## The table list

Clicking the `Tables` folder opens a tab of **every table of that namespace with what the
catalogue keeps about it** — one row per table, in a `ResultGrid` like everything else that shows
rows and columns. Over it sit `Open Table`, `Design Table`, `New Table` and `Delete Table`; under
it the same `GridFooter` every grid has, with `Refresh` and the table count.

- **The columns are the engine's own**, and only the first is promised: the table's name, which is
  what the toolbar acts on. MySQL reports `Auto Increment`, `Modified Date`, `Data Length`,
  `Engine`, `Rows` and `Comment`; PostgreSQL reports `Owner`, `Access Method`, `Data Length`,
  `Rows` and `Comment` — and **no modified date, because the server keeps none**; MongoDB reports
  `Kind`, `Documents`, `Data Size`, `Storage Size`, `Avg Document Size`, `Indexes` and `Index Size`.
  No side of the wire names or formats a value: a size arrives spelled the way that engine spells it
  (`1,552 KB` from MySQL's own arithmetic, `1928 kB` from `pg_size_pretty`, `1.5 MB` from MongoDB's
  own, whose `$collStats` reports bytes), because inventing a column an engine cannot fill would say
  nothing. A row count is the catalogue's **estimate** on the SQL engines, and `(Null)` in
  PostgreSQL's says nothing has analysed the table yet; MongoDB's is exact, and a **view** reports no
  numbers at all — it has no storage of its own, so its row carries its name and its kind and nulls
  where the numbers would be, rather than a row of zeroes claiming an empty collection.
- **The toolbar acts on the selected row**, and its four items are the same objects the tree's
  right-click builds from (`tableActions`), so a confirmation is worded once. `Delete Table` asks
  first, and the change re-reads both the tree's names and this list's numbers.
- **A double-click opens the row's table**, which is the tree's own gesture applied to a row: the
  grid is read-only here, so a click has nothing else to mean. `ResultGrid` reports it through
  `onActivateRow` — see [grid.md](../grid.md).
- The list is **read per tab rather than cached** like the tree's names are: a size and a row count
  are what the reader opened it for, and a stale one is worse than a second query.
