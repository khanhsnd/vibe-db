# Finding text in a schema

[← Features and UI behaviour](../features.md)

**Right-click a database, the node that owns tables, or its `Tables` folder: `Find in Schema…`.** It
opens a tab that reads **every table of that namespace** looking for one piece of text, and answers
with one row per object that holds it and how many times.

```
┌ Connection: ────────────┬ Find results for “khánh”: ──────────────────────┐
│ [localhost-postgres  ⌄] │ Name                    │ Number of Matched Re… │
│ Database:               │ ▦ CV_VerificationCandi… │                     2 │
│ [hirec-test          ⌄] │                                                │
│ Schema:                 │                                                │
│ [public              ⌄] │                                                │
│                         │                                                │
│ Find what:              │                                                │
│ [khánh              ]   │                                                │
│ Look In:   '\' as escape│                                                │
│ [Data          ⌄]       │                                                │
│ Search Mode:            │                                                │
│ [Contains      ⌄]       │                                                │
│ ☐ Case Insensitive      │                                                │
│                [ Find ] │                                                │
├─────────────────────────┴────────────────────────────────────────────────┤
│ Found 1 objects with matched records          40 objects searched         │
```

- **`Look In` is two different questions, not two settings of one.** `Data` counts the **rows** whose
  columns hold the text, which is a statement and therefore the driver's
  (`driver.Session.MatchingRows`) — every column is read as text, so a search for `36` finds the
  number as readily as the name. `Structure` counts the **names** that hold it — the table's own, then
  every field, index and constraint — and asks no server anything at all: those names are what the
  catalogue reads the tree already makes answer with. One rule matches both (`domain.TextMatch`), so
  `Contains` cannot mean two things.
- **`Search Mode` is four modes and the reader's text is literal in three of them.** `Contains` and
  `Prefix` are *patterns*: `%` stands for any run of characters, `_` for one, and `\` makes either of
  them itself — which is what the note beside `Look In` says, and it is shown only while one of those
  two is picked. `Whole Word` is the text between two word edges, taken literally, so `a.b` finds
  `a.b` and not `axb`. `Regular Expression` is the reader's own, in their engine's own flavour of one,
  and an expression the server refuses is reported as the server's own rejection.
- **`Case Insensitive` decides, rather than the collation deciding for it.** A caseless match folds
  both sides; a case-*sensitive* one on MySQL compares the pattern as bytes and asks `REGEXP_LIKE` for
  `c`, because its usual collations would otherwise ignore case whatever the box said.
- **The connection is one of the pickers**, so a search can be pointed at another server or another
  database without going back to the tree — and the tab moves with it, its strip naming where it is
  now looking rather than where it was opened.
- **A count per object, and not the matches.** A table with four thousand matching rows is not a list
  anybody scrolls, so what the grid holds is where to look next.
- **A double-click on a data hit opens a query tab holding the read behind the count** —
  `SELECT * FROM "public"."CV_VerificationCandidates" WHERE (CAST("Name" AS TEXT) LIKE '%khánh%' OR …)`,
  or `db.users.find({ $or: [ … ] })` on a document store. It is **the engine's own statement, arriving
  with the hit**, spelled from the *same* condition the count was made through — so the tab cannot
  show a different set of rows from the number that sent the reader to it, and nothing in the frontend
  has spelled a statement. It is **not run for them**: a statement in an editor is the reader's to
  send, which is the rule everywhere else in the app. A **structure** hit has no such read — matching
  names is not one — so it opens the designer, where a field, an index and a constraint are read.
- **An object the search could not read is named, and the search carries on.** A schema nearly always
  holds a table this account may not read, and stopping at the first one would make the feature
  unusable there; a warning under the results says which and what the server said. A **Stop** replaces
  `Find` while it runs, which is the honest bound on a read as long as the schema is.
- Views and routines are **not** searched, and neither is a value's *type* or a column's default: what
  is read is the tables of the namespace, their rows and their names. On MongoDB the fields looked in
  are the **sampled** ones — the only field list a collection has — and a `$regex` matches a field
  holding a string, so a number in a document is not text to search. See [grid.md](../grid.md).
