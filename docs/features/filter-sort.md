# Filtering and sorting

[← Features and UI behaviour](../features.md)

**Filtering is the table tab's**, since it is the one surface whose rows come from a statement this
side writes and can therefore narrow. A query result and the history log hold their rows already and
are only ever *sorted* — by a header's `⋮` menu, in memory. What follows is the table tab.

There are **three ways in**, and they hold the same conditions:

- **The Filter & Sort button** in the table view's toolbar, beside `Cell Editor` and `Column`. It
  opens `FilterSortPanel`, which is where a
  condition is *built*: a tick box, a column, an operator and a value, each of the three parts
  opening the list of what it may be. `+` adds a condition beside the row and `()+` adds a **group**,
  which is parentheses joining its children by `and` or by `or` — the word between the rows is a
  button that flips the whole group, so `a AND (b OR c)` is sayable and not merely wishable.
  **Sort By** under it is a list of columns and directions, the first deciding and the rest breaking
  its ties. Nothing narrows anything until **Apply Filter & Sort**; the panel says **Criteria
  Edited** while the draft differs from what is in force. The whole of it is in
  [grid.md](../grid.md#the-filter--sort-panel).
- **A header's `⋮` menu**, where the reader already knows which field they mean: **Add Filter** opens
  the panel and puts a condition on *that* field in it, at the end of the root. A header knows the
  field and not the value, so what it adds is unspelled — it lands in the **draft**, narrowing
  nothing until the value is picked and **Apply Filter & Sort** is pressed. It is the shortest way
  from a column the reader is looking at to a condition on it.
- **Right-clicking a cell**, where a condition is *picked* rather than built. The menu is one object —
  [grid.md](../grid.md) lists every item — and **Filter ▸** offers every operator against the value under
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
actually holds — `LoadColumnValues`, a `SELECT DISTINCT … ORDER BY 1 LIMIT 200`, or on MongoDB a
`$group` / `$sort` / `$limit` pipeline, which is the only spelling whose cap is the server's: `distinct`
answers with every value there is and would hand back a megabyte to fill a list of two hundred — with
a box beside it for a value the list has not got. NULL is not offered there: `is NULL` is the way to it. There is
no OK on the panel: what it changes is the draft, and **Apply Filter & Sort** is the one commit.

**Ticking more than one value makes it `is in list`.** The rows are check boxes, and the operator
changes as the second one is ticked — `Name = Java developer` becoming
`Name is in list (Java developer, JavaScript)` — so what is on screen is always what will run;
a negative operator becomes `is not in list`, and unticking back to one gives the old operator back
while the panel is still open. The four comparisons that name a **bound** (`<`, `≤`, `>`, `≥`) are
single-pick instead: `> a` and `> b` are one comparison, not both.

**The filtering happens on the server**: the tree travels in `TablePageRequest`, becomes the read's
`WHERE`, and the **total is counted through it too**, so the pager walks the pages that are actually
there and the status line's statement shows the clause that ran. A draft row is never filtered away;
it is a row still being filled in.

The operator crosses the wire as a **token** (`gt`, `contains`, …) and is spelled by the driver, so
nothing a reader typed arrives as an operator; a conjunction crosses as `and` or `or` and nothing
else. On the SQL engines the *values* are written into the statement as string literals rather than
bound, for the same reason the `LIMIT` is: one string is then both what ran and what the status line
shows and copies — a statement carrying `?` is not one a reader could paste into an editor.
`sqlkit.Dialect.QuoteString` is what stands between the text and the statement, and both engines read
a string literal back into a column of any type. Two smaller consequences: the six text matches
compare the column **as text** (`CAST(… AS CHAR)` / `AS TEXT`), so they work on a number or a date as
well as a name, and a `%` the reader typed is their own wildcard. In memory those matches ignore case;
on the server the collation decides, which is MySQL ignoring it and PostgreSQL not.

**On MongoDB there is nothing to quote.** The tree becomes a `bson.D` and travels as a *document*, so
a field name and a value reach the server as values and can only ever be values. Three things follow.
The reader's text is read into whatever type the sampled field holds, which is what stands where a
SQL engine's cast stands — a value that will not read as that type stays the text it is, matching
nothing rather than quietly becoming something else. The six text matches are a `$regex` with the
value **escaped** and matched case-insensitively: a regular expression has a dozen metacharacters
where `LIKE` has two, so a `contains` of `a.b` finds `a.b` and not `axb`, and a reader who wants a
pattern writes one in the query tab. And `is NULL` is `{ field: null }`, which finds a document
*missing* the field as well as one holding null, because in a document store those are one question.
The statement shown under the grid is the command that ran, spelled from the same documents that were
sent: `db.users.find({ age: { $gt: 30 } }).sort({ _id: 1 }).skip(0).limit(100)`.
