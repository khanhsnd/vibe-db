# Results — the output panel, editing and export

[← Features and UI behaviour](../features.md)

## The output panel

The panel under the editor is a tab strip of **`Result`**, **`History`**, **`Snippets`** and, for a
script, **`Messages`**; a further pane — a query plan — is one more entry in the list `QueryOutput`
hands `ResultTabs`. Nothing sits below that strip: what the run did is the pane's own to report, on
the status line under its grid, so `QueryOutput` is the tabs and nothing else.

The strip runs from what this run produced to what outlives it: a plan belongs to the result beside
it, `History` to the connection, [`Snippets`](snippets.md) to the reader themselves, and `Messages` to
the script that needed it.

**An Execute brings its own outcome forward** — `Result`, or `Messages` for a script that stopped —
wherever the reader had got to, and otherwise the strip stays where they left it. `Messages` is
there only for a script, so a single statement run while it was in front puts the reader back on
`Result` rather than on a pane that has gone.

**A failure can be taken away with the reader.** The app selects nothing by dragging — it is a
desktop client, not a document — but *every surface that reports a failure is an exception*: the
error panel, the one-line strip under a grid that could not save, an empty pane whose hint is the
server's wording, the console's transcript, the note under a box a dialog refused, and the status
bar. The error panel also carries a **copy button**, which takes the code, the wording and the
engine's own detail together, since dragging across three lines in a pane forty pixels tall is not
how anybody wants to quote an error.

### The history pane

`History` is every statement **this connection** has run, newest first, in a `ResultGrid` like
everything else that shows rows and columns. It has two siblings, each answering a different question:
the [History Log](log.md) holds every *run* in the order it happened, across every connection, and the
[snippet library](snippets.md) holds the statements the reader **named** — kept by them rather than by
the app, and put in at the caret rather than over the buffer.

The columns are `When`, `Statement`, `Database`, `Runs`, `Rows`, `Time (ms)` and `Message`, which
reads `OK` or the server's own wording exactly as `Messages` does.

- **A double-click loads a row's statement into the editor**, which is what the log is for; the grid
  is read-only, so a click has nothing else to mean. It brings **the namespace it ran in** with it,
  because a statement names tables of one database and one run against another would fail on names
  that are not there — the toolbar's pickers move with it, visibly. The editor's own undo is what
  puts back whatever was being typed.
- **It is ordered by a header's own `⋮` menu**, in memory, there being no statement of its own to
  re-order; the footer says how many statements the log holds. There is no filtering here. `When` is
  spelled `2026-08-04 14:23:05` — local time, in the one spelling that also sorts as text.
- **The pane is read on mount rather than held in a store.** It is drawn only while the reader is
  looking at it, so every open is a fresh log and an Execute never has to tell it anything.
- **The log is every connection's and the pane shows one.** A statement of another server names
  tables that are not here, so it would be noise at best and unrunnable at worst.
- **`Clear History` forgets every statement**, behind the same confirmation a drop is behind, and it
  says that nothing on any server changes. It sits over the rows rather than in the footer, because
  it acts on the whole log where the footer acts on a row. There is no way to forget one entry: a log
  is what it is, and a reader who wants it gone wants it gone.

### Editing a result

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

## Exporting a grid

An **Export** button sits **in the strip over every grid, against its right edge**, with the
`Cell Editor` toggle beside it — its own head strip on a query result, and the table view's toolbar
where `Cell Editor` and `Column` already are, one strip
over a table rather than two — and opens **CSV**, **JSON**
and **SQL**; each writes to a file picked in the system's own save dialog — so the file lands where the
reader put it, named after the table, and nothing is written if the dialog is dismissed. It is above
the rows rather than in the footer below them because it acts on the table as a whole, where the
footer acts on a row.

**What is written is what the grid holds, in the order it draws it.** A query result exports its
whole result rather than the page in front, since a reader who paged still means the result; a table
tab exports the page, because a table's rows are paged by the *server* and the page is all the grid
has. A hidden column is not exported: it is one nobody asked for. **A reader who means the whole
table rather than what is on screen wants [`Dump SQL File`](transfer.md#dumping-a-table-or-a-schema-to-a-file)**
in the tree instead, which reads every row out of the server rather than off the grid.

**The cell menu's Save Data As ▸ is the same three formats**, so a reader who is already in the menu
need not travel to the button; **Copy As ▸** beside it puts the selection on the clipboard in the
clipboard's own shapes — tab-separated with field names, CSV, JSON — and treats a `NULL` the way the
file of that format does.

| Format | What it does with a `NULL` |
|---|---|
| CSV | An empty field, which is the only thing a CSV has to say "nothing" with |
| JSON | `null`, which is why a reader who needs to tell it from an empty string wants this one |
| SQL | The `NULL` keyword. Every other value is written as a literal, which both SQL engines read back into a column of any type. The statements are asked of the driver, so the quoting is the engine's; a result no single table can be traced to is named `exported_rows`, which the reader has to correct — the honest outcome, rather than a file that looks right. **On MongoDB the format writes commands, not SQL**: one `db.<collection>.insertOne({…})` per document, with an absent value *left out* rather than written as null — a field nobody wrote is not a field holding nothing, and that is the difference a document store keeps. Each value is read back out of the notation the grid drew it in, so a number exports as a number and an `ObjectId` as one |

Beside **Export**, the `</>` action opens a right-hand **Generate cURL** drawer for the table in
front. The first format is cURL; it generates one POST command, naming the table in the URL and
every field in a JSON body with an empty-string value. The reader can copy it with one click, and
the format picker is intentionally ready for additional request formats later.

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
