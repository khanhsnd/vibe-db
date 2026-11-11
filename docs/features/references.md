# Following a foreign key

[← Features and UI behaviour](../features.md)

A column of a [table tab](table-tab.md) that a **foreign key** is declared on says where its values
point, and takes the reader there two ways.

- **The header names the referenced table, and that name opens it.** It sits on the header's type
  line after the type, in `--accent` — `↗ customers` — so which table a column points at is read
  rather than hovered for. A name too long for the column is clipped with an ellipsis, and the
  **tooltip** spells both ends of the key whole —
  `customer_id → customers.id — click to open customers`. It opens a table tab like any other: the one a double-click in the tree
  would have opened, and **reused** if it is already in the strip, with whatever page and filter the
  reader left on it untouched.
- **`Go to <table> · <column> = <value>` on the cell menu opens it narrowed to that row.** The
  referenced tab is given the conditions that name the row — one `=` per pair of the key — and its
  Filter & Sort panel is opened, so the reader can see why the table is one row long and can take
  the condition off. A tab already open on that table is reused and its filter **replaced**: the
  reader asked for one row, and a row hidden behind last week's filter would look like a broken
  link. A NULL points at nothing, so a NULL cell has no such item.

## What a reference is

- **It is the catalogue's, never a guess.** The keys are read from the same
  [catalogue cache](schema-tree.md) the tree draws `Foreign Keys` from — one read per table, and none
  where the tree has already opened it — so a column that merely *looks* like a key (`user_id` with
  no constraint behind it) gets no link. An engine with no referential constraints
  (`EngineInfo.hasForeignKeys`) is never asked.
- **A composite key is followed whole.** Go to writes a condition for every column of the key, read
  off the row the menu was raised on; one of two columns alone would name every row sharing it. A
  member the row has no value for — it is NULL, or its column is hidden and so not on the grid —
  writes no condition rather than a wrong one.
- **A key into another namespace opens that namespace's table**, and the label qualifies it
  (`catalog.products`) only then. Which part of a namespace the key's qualifier replaces follows
  the namespace itself: the schema where it has one, the database where it has not.
- **A column in two keys carries the first**, a header having room for one link.
- **Only the direction the key is declared in.** A table does not list who points *at* it: that is
  a fact held in every other table's catalogue entry, and reading them all to draw one header would
  be a namespace-wide read behind a tab that asked for a page.

The grid knows none of this. A column carries a `ColumnReference` — a label and an `onOpen` — and
[the grid](../grid.md) draws the arrow and the menu item; `features/table/references.ts` is what
turns a constraint into one, and `TableDataView` is what makes "open" mean a tab.
