# Dumping, importing and exporting

[← Features and UI behaviour](../features.md)

## Dumping a table or a schema to a file

`Dump SQL File ▸` writes **the statements that would make these objects again**, to a file picked in
the system's own save dialog. It is the one item in the tree's menus that changes nothing on the
server, and it sits beside `Duplicate` for that reason: both copy the table, and the only difference
is whether the copy lands on this server or in a file.

- **It is the same two-way choice `Duplicate` is** — `Structure Only` and `Structure and Data` — and
  the same submenu, because it is one action asked in two ways.
- **Where it is offered says what it covers.** On a table, that table. On the node owning tables, or
  its `Tables` folder, **every table of that namespace** in the order the tree lists them. A view is
  never among them — it holds no rows of its own and its declaration is not a table's — and neither is
  a routine.
- **Nothing is confirmed first**, unlike every other item in these menus: nothing is being changed, so
  the save dialog *is* the question, and dismissing it writes nothing at all. The file is named after
  the table, or after the schema or database whose tables are going into it.
- **What kind of file it is belongs to the engine** (`engineWords.scriptFile`): a MongoDB connection
  reads `Dump Script File` and writes `.js`, because its statements are commands and not SQL — and
  calling that file `.sql` would be a lie about its contents.
- **The declaration is the engine's own answer wherever the engine has one.** MySQL's dump is
  `SHOW CREATE TABLE` verbatim — storage engine, character set, `AUTO_INCREMENT`, every index and
  every constraint — with the database written into the name so the file's declarations and its rows
  cannot land in two different places. PostgreSQL has no such statement, so its shape is described
  from the same catalogue read the tree draws: one `CREATE TABLE`, then a `CREATE INDEX` per index
  (never the primary key's, which arrives with the table), then an `ALTER TABLE … ADD CONSTRAINT` per
  foreign key. A `serial` column is declared as `serial` rather than as the `integer` and
  `nextval(…)` default the catalogue reports it as, or the file would name a sequence it never
  created; an index over an **expression** is the one thing a description cannot carry, and it is
  written as though the expression were a column name. MongoDB writes `db.createCollection(…)` and a
  `createIndex` per index, `_id_` excepted since the server builds that one itself — and **nothing
  about the fields**, because a collection declares none: they arrive with the documents.
- **The rows are streamed, not paged.** A dump reads each row once in one pass and writes each batch
  as it arrives, so a table of a million rows costs one batch of memory and not a table's worth. It is
  given two hours rather than a query's fifteen seconds, for the reason
  [Execute SQL File](console.md#execute-sql-file) is: a long job going fine is not a job to cut off.
- **A `NULL` and a value are written exactly as the SQL export writes them** — the same
  `RowStatements` the grid's `Export as SQL` calls, so a dump and an export of one table's rows
  differ in one thing only: **a dump names the namespace and an export names the table alone**,
  because a dump puts a database back where it came from and an export carries rows out of it. See
  [A SQL export names the table alone](results.md#a-sql-export-names-the-table-alone).

## The Import and Export wizards

`Import Wizard…` and `Export Wizard…` sit beside `Dump` on a table, on the `Tables` folder and on the
node that owns it — the same three places, covering the same subject: **one table where the reader
right-clicked one, and every table of the namespace where they right-clicked the folder.**

**They are not `Dump`, and the three do not overlap.** A dump writes the *statements* that would make
these objects again, for putting a table back on a server. The wizards carry the **rows**, in the
formats something other than a database reads. That is why SQL is not in the wizard's format list and
no wizard format is in the dump menu.

An export is also one of the four jobs a [clock](automation.md) can fire, on the wizard's own
defaults — and it writes the same file names every time, so a scheduled export is a *mirror* of the
namespace rather than a pile of dated copies. **An import is not scheduled**, being a write.

| | Reads | Writes |
|---|---|---|
| CSV (`.csv`), Text (`.txt`) | ✓ | ✓ |
| JSON (`.json`) | ✓ | ✓ |
| XML (`.xml`) | ✓ | ✓ |
| Excel (`.xlsx`) | ✓ | ✓ |
| HTML (`.html`), Markdown (`.md`) | | ✓ |

HTML and Markdown are written and never read: both are laid out to be *looked at*, and reading one
back would mean guessing which of a document's tables was the data — which is exactly the guess that
goes wrong on a document a person laid out. The formats live in
[`internal/transfer`](../architecture.md#carrying-a-table-to-a-file), which knows no databases at all.

**One question to a screen**, in the order the answers depend on each other: what the file *is*, then
where it is, then how its bytes are cut into fields, then which field goes into which column, and only
then what the rows are to **do** when they get there. `>>` runs to the last step from wherever the
reader is, so a second import of the same shape of file is Next, Next, `>>`.

- **A step that has nothing to ask is not shown.** A JSON file has no delimiter to be wrong about, so
  a JSON import is six steps and a CSV import is seven. A step of greyed-out boxes says only that the
  wizard was written for something else.
- **Nothing reaches a server until `Start`.** Every step before it reads the file and nothing else,
  which is why the preview needs no session — and why pointing the wizard at the wrong file costs
  nothing.
- **The preview is how the answers are checked.** The head of the file is drawn in a `ResultGrid`
  under the delimiter and rows steps, read exactly as the import will read it: a delimiter that is
  wrong shows as one column holding the whole line, and a header row that is not one shows as columns
  named after the first record.
- **The mapping is proposed, not typed.** A file whose fields are named after the columns — which is
  nearly every file exported from anything — arrives already mapped, and the key is the table's own
  primary key. A column the file has nothing for is **left out of the write entirely**, so the
  server's own default applies: that is not the same as writing `NULL` into it.
- **A workbook is asked which sheet.** Excel stores a date as a count of days and says so only in its
  style table, so that table is read and a date column imports as a date rather than as `45678`.

### What an import mode does

| Mode | |
|---|---|
| `Append` | Adds every record. A row already there is a failure the *server* reports — only it knows what a key collision is. |
| `Update` | Changes the records the key matches, and adds none. |
| `Append / Update` | Changes the records the key matches and adds the rest. |
| `Append without update` | Adds what the key does not match and leaves the rest alone. |
| `Delete` | Removes the records the key matches. |
| `Copy` | **Empties the destination** and then appends. |

The four that find a stored row first are **left disabled until a key column is ticked** on the
mapping step, because without one they would reach every row in the table. If one slips through
anyway — a key mapped to a field the file turns out not to carry — the statement matches *nothing*
rather than everything: `WHERE 1 = 0` on a SQL engine, and a filter that no document satisfies on
MongoDB.

**Whether the run is answered for first is a `domain` rule, not the wizard's.** `Copy` is
destructive, a keyed mode with no key is unfiltered, and everything else is an ordinary write —
`domain.ImportHazard`, read against the connection's environment by the same
`Environment.NeedsConfirmation` a typed statement goes through. The wizard *asks* the Go side and
draws the confirmation it is told to; it never decides. See
[What gets confirmed before it runs](connections.md#what-gets-confirmed-before-it-runs).

`Records per statement` is how many rows one statement carries — more is fewer round trips, and too
many is a statement the server refuses as oversized (`max_allowed_packet`). `Continue after a record
the server rejects` is the same switch [Execute SQL File](console.md#execute-sql-file) offers and
means the same thing. **The reader's own `Stop` always ends the run**, whichever way that switch is set.

### What an export writes

**One file per table**, named after the table, in the folder the reader picks — even for Excel, which
gets one workbook of one sheet each. A wizard that also asked for a file *name* would be asking a
question with no answer for the forty-table case. A character a file name may not hold is replaced
rather than dropped, so two tables differing only in punctuation do not write over each other.

- **The rows are streamed, not paged**, exactly as a dump's are: each batch is written to the file as
  it arrives, so a table of a million rows costs one batch of memory. Both wizards are given two
  hours for the reason a dump is.
- **A table that fails does not stop the rest.** An export changes nothing on a server, so there is
  nothing for a half-finished one to have left inconsistent — and a reader exporting forty tables
  would rather have thirty-nine and a message than a stop at the seventh.
- **A format that cannot say `NULL` asks what to write instead.** CSV, text, HTML and Markdown have
  nothing to say it with; JSON has `null`, XML says so in an attribute, and a spreadsheet's empty cell
  is not a cell holding no characters, so those three are not asked.
- **A UTF-8 file is written with a byte order mark**, which is what makes Excel open a UTF-8 CSV as
  UTF-8 instead of as the machine's own code page.

The last step of both wizards is a **transcript**, not a grid — for the reason
[Execute SQL File](console.md#execute-sql-file)'s is: a server's rejection is a paragraph it wrote
and a path is a hundred characters wide, and in cells those are the two columns clipped to nothing.
