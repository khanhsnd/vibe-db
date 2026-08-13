# Changing a schema

[← Features and UI behaviour](../features.md)

Every schema change is raised from a right-click in the tree, and every one of them is confirmed or
filled in before it runs. `useTreeMenu` is the single place the items are built, per node kind:

| Right-click | Items |
|---|---|
| A connection | `Connect` / `Disconnect`, `New Database…` — only once there is a session to make one on — `Edit Connection…`, `Delete Connection` |
| A database | `New Database…`, `Delete Database`, then what its level owns: `New Table`, `Find in Schema…`, `Dump SQL File ▸` and `Refresh` on MySQL and MongoDB, `Refresh` alone on PostgreSQL, which holds schemas rather than tables |
| The node owning tables, or its `Tables` folder | `New Table`, `Find in Schema…`, `Dump SQL File ▸`, `Refresh` |
| The `Views` or `Functions` folder | `Refresh` — one read fills every folder, so it means the same wherever it is asked for |
| A table | `Open Table`, `Design Table`, `Compare … ▸`, `New Table`, `Rename Table…`, `Duplicate Table ▸`, `Dump SQL File ▸`, `Import Wizard…`, `Export Wizard…`, `Empty Table`, `Truncate Table`, `Delete Table`, `Refresh` |
| A view | `Open View`, `Compare … ▸`, `Delete View`, `Refresh` |
| A function or procedure | `Delete Function` / `Delete Procedure`, `Refresh` |
| The `Fields` folder | `Add Field`, `Refresh` |
| A field | `Edit Field`, `Rename Field…`, `Add Field`, `Delete Field`, `Refresh` |
| An index | `Delete Index`, `Refresh`. Disabled on the index behind a primary key: no engine will drop the key as an index, so the item says so rather than failing at the server |
| A foreign key | `Delete Foreign Key`, `Refresh` |

**Every word above is the engine's** (`engineWords`): a MongoDB connection reads `Open Collection`,
`Delete Collection`, and `Delete every document of orders?` in the confirmation. And the *items*
follow the engine too — MongoDB is offered no `Truncate Collection`, no routine rows and no foreign
key rows, because it has none of the three.

- **Every item carries a glyph**, from `shared/ui/menuIcons.tsx` — one action, one icon, wherever it
  is offered: `Design Table` and `Edit Field` share theirs because both open the designer, and
  anything that destroys carries the bin in the danger colour.
- **`Empty Table` deletes every row; `Truncate Table` drops them all at once.** Truncate is the
  faster of the two and the one a table under a foreign key will refuse, which is why both are there.
  **MongoDB is offered `Empty` alone**: a collection has no storage to reset, so the only way to take
  every document out of one and leave its indexes standing is to delete them — and two menu items
  running one command under two names would be a lie about what the second does.
- **`Duplicate Table` is one action asked in two ways**, so it is the one item with a submenu:
  `Structure Only` and `Structure and Data` are the same bound method and differ by a flag. Both ask
  for the copy's name — nothing here makes an object without being told what to call it — and the
  copy is then the table whose shape is re-read. What travels is the table's own: its columns, their
  defaults and its indexes, and **never a foreign key**, which names two tables and is neither
  engine's to copy. **A name already taken is refused** — on the SQL engines by the server, and on
  MongoDB by the adapter, because that server accepts `create` for a collection it already has and
  `$out` writes straight over one.
- **A change re-reads what it invalidated, and nothing else**: the namespace's table list, then the
  table's own shape. Which shapes are still worth holding is read off the fresh table list, so a
  dropped or renamed table needs no telling.
- **A rejected statement stays where it was asked for** with the server's own wording, so a name
  already taken costs a retype and not the work.

## Where a schema change is made

**Anything with more than one answer in it happens in a tab, not a dialog** — `New Table`,
`Design Table`, `Add Field` and `Edit Field` all open (or reveal) the designer, which has the room
to work in and leaves the tree beside it readable. Three requests stay modal: `Rename …`, which is
one box; a confirmation, which has to be answered before anything else happens; and `New Database…`,
which has **no tab of its own to open** — a database is not a thing you sit and work in, and its
answers are one fixed screen rather than a list.

## New Database

The dialog is a tab strip over a form, and how much of the form there is belongs to the engine.
**PostgreSQL's database is an object in its own right**, so `General` asks for a Database Name, an
Owner, a Template, an Encoding, a Collation Order, a Character Classification, a Tablespace and a
Connection Limit, plus `Allow Connection` and `Is Template`; `Extensions` is a filterable list of
what the server has to offer; and `Comment` is free text. **MySQL's database is a namespace with a
character set**, so `General` asks for a name, a Character Set and a Collation — and the other two
tabs are not drawn, because MySQL has neither.

**MongoDB has no `CREATE DATABASE` at all.** A database exists exactly as long as it holds a
collection, and no command makes an empty one — so `General` asks for a Database Name and a **First
Collection**, and creating the database *is* creating that collection. A blank answer takes `data`
rather than failing: a reader who only wanted the database is better served by a collection they can
rename than by an error about a field they did not know was required. The preview pane is labelled
`Command Preview` there, because what it shows is not SQL.

- **Every list is the server's own**, read in one call when the dialog opens: the roles, the
  templates, the encodings, the tablespaces, the available extensions. A blank entry reads
  `Server default` and leaves that clause out of the statement altogether, rather than this side
  guessing at what the default is. MongoDB answers with no lists at all — a database there is a
  namespace and nothing more — so the dialog draws none of them.
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

## The designer

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
- **Every list is a `ResultGrid`**, because they are rows and columns — see [grid.md](../grid.md), and
  **all three are edited in place**: one click on a cell opens it, `Escape` closes it again, and
  the arrows walk the rows a cell at a time.
- Its columns are the parts a field is *declared* from, not the string the server reports back:
  `Name`, `Type`, `Length`, `Decimal`, `Not null`, `Key`, `Default` — and `Comment` where the engine
  keeps one. Length and decimals are parts
  of the type and travel as one string — `varchar(255)`, `numeric(10,2)` — and read as `0` when the
  type declares none. A `Default` is an **expression**, which is what a server-side default is:
  `now()`, `'IN_PROGRESS'`, `0`.
- **`Comment` is what somebody wrote about the field on the server**, and it is the one column here
  that no statement ever reads: it is documentation the catalogue keeps beside the column. It is
  **left out entirely** on an engine with nowhere to put one rather than drawn dead, because what a
  reader typed into it would be thrown away by the `Save` — `EngineInfo.hasColumnComments` is what
  says so. It exists on MySQL, where it is a *clause of the column*, and on PostgreSQL and Oracle,
  where it is a `COMMENT ON COLUMN` **statement of its own**; SQL Server's is an extended property
  written by a stored procedure rather than a comment, and SQLite has none.

  Clearing it clears the description, which is why an altered column on the two `COMMENT ON` engines
  spells that statement whether or not anything was written: leaving it out would leave whatever was
  there before. An **added** column with a blank comment spells nothing — there is no earlier
  description on a column that does not exist yet for an empty one to clear. Which of the two
  shapes an engine takes is a **flag** on `sqlkit.Dialect`, never a second speller.
- **`Type` is a list, and the list is the engine's.** Clicking the cell offers MySQL's types,
  PostgreSQL's, or MongoDB's BSON types — `dataTypes.ts`, read off the connection's own profile — and
  typing filters them. It still takes **anything typed into it**, because a server also has domains,
  enums and types that list has never heard of; the options are what a column is usually given.
- **On MongoDB a field is not a column, and the designer says so by what it does.** A collection holds
  no declaration to change, so a change to a field is a change to the *documents* that carry it:
  adding one writes it onto the documents that lack it and leaves the rest alone, renaming one is a
  `$rename`, dropping one is an `$unset`, and altering one converts its values in place — a value the
  server cannot convert is removed rather than left wrong, which is the only outcome in which the
  collection afterwards holds what was declared. A **new** collection is the one case with nothing to
  change: `New Collection` creates it and the fields declared alongside have no documents to be
  written onto, so the Fields grid comes back holding `_id` alone. That is the truth about a new
  collection rather than a shortcoming — its fields arrive with its first document. See
  [architecture.md](../architecture.md#the-document-store) and
  [decisions.md](../decisions.md) for why the alternative, a validator built from the declared fields,
  is worse.
- **`Not null` is a checkbox and `Key` is the lock**, both flipped by one click. A key field shows
  a gold lock and **which column of the key it is** — the first is `1`, a second field clicked is
  `2` — counted down the fields, so a composite key reads in order. `Primary Key` in the toolbar is
  the same fact for a whole selection at once.
- `Primary Key` moves the key onto the selected fields, or off them if they all already hold it.
  Both SQL engines are made to replace a key rather than add a second one, and dropping the key of a
  table that has none is not an error. **MongoDB answers `UNSUPPORTED`** for anything but `_id`: the
  server mints it, indexes it and will not let it be changed, so a request naming it is already true
  and one naming anything else cannot be granted. A reader who wants some other field to be unique
  wants a unique index, which the Indexes grid makes.
- **The Indexes grid** is `Name`, `Fields`, `Unique` and `Method`, and **the Foreign Keys grid** is
  `Name`, `Fields`, `Referenced Schema`, `Referenced Table`, `Referenced Fields`, `On Update` and
  `On Delete`. A list of columns is typed as one cell, comma separated, because that is how it
  reads. `Method` offers the engine's access methods and the two action cells offer the five
  referential actions — suggestions, like the type list, since the cell still takes what is typed.
  A blank `Referenced Schema` means the same namespace the table is in. **There is no Foreign Keys
  pane on MongoDB**, which has no referential constraints of any kind: a pane whose `Save` could only
  fail is a pane worth leaving out. Its `Method` list is the values a MongoDB index *key* takes —
  blank for the ordinary ascending index, then `hashed`, `text`, `2dsphere`, `2d` — because that is
  where MongoDB keeps what SQL puts in a `USING` clause.
- **An index behind the primary key is not in the Indexes grid.** It is the key, and the Fields grid
  is what moves a key.
- **There is no Partitions pane.** How a table's rows are *stored* is not part of what the three
  grids declare, and a piece is added, emptied and dropped from the tree — see
  [partitions.md](partitions.md).
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
