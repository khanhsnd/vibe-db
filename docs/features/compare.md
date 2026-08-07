# Comparing two tables, and making them agree

[← Features and UI behaviour](../features.md)

**Pick one table, `Ctrl`-click a second, right-click either: `Compare A ⇄ B ▸ Structure Only…` or
`Data Only…`.** The two need not be in one database, on one server, or even on one engine — a
staging table against the production one is the case the feature exists for, and each side carries
its own session.

The item is on a table's and a view's menu, and it is **there even when only one thing is picked**,
drawn dead and reading `Compare — Ctrl-click a second table`. That is the exception to the rule that
an item with nothing to do is left out: the gesture is the whole feature, and a gesture nobody can
see is a gesture nobody finds.

## The tab

```
[Structure|Data] ⌄⌃ ⌃⌄  Apply to [users_b|users_a]  ☐ Differences only  Refresh   Take all  Take none  [Apply to users_b]
├─ Origin: shop.users_a ─────────────────────╥─────┬─ Target: shop.users_b ───────────────────────┤
│ Name              │ Declaration            ║Take │ Name              │ Declaration  │Difference │
│ ▾ ▦ shop.users_a  │                        ║ ☐   │ ▾ ▦ shop.users_b  │              │3 of 11 …  │
│   ▾ 📁 Fields     │                        ║ ☐   │   ▾ 📁 Fields     │              │3 of 7 …   │
│     🔑 id         │ int NOT NULL PK        ║ ☐   │     🔑 id         │ int NOT N..  │           │
│     ▯ name        │ varchar(50) NULL       ║ ☑   │     ▯ name        │ varchar(100) │type, null │
│     ▯ created_at  │ timestamp NOT NULL     ║ ☑   │                   │              │only in u… │
│   ▾ 📁 Indexes    │                        ║ ☐   │   ▾ 📁 Indexes    │              │1 of 3 …   │
│     ⇅ by_email    │ (email)                ║ ☑   │     ⇅ by_email    │ UNIQUE (em..)│unique     │
│   ▸ 📁 Foreign K… │                        ║ ☐   │   ▸ 📁 Foreign K… │              │all 1 ide… │
├─ dragged ────────────────────────────────────────────────────────────────────────────────┤
│  Script (8) │ DDL Diff                                                                    │
```

`║` is the splitter, and the ticks sit on the far side of it — a strip of their own between the two
trees. The data half is the same three panes with rows in them, and it alone carries `[⇄|⇅]` and
`Rows`.

Three rules hold the whole surface up, and everything else follows from them.

**1. The same tree, drawn twice.** The shape half is **two panes**, each holding one side's tree:
the table is the root, `Fields`, `Indexes` and `Foreign Keys` fold under it, and each row under one
names a thing and carries what *that side* declares it as. Both are built from one list, so a caret
belongs to the **row** and not to a pane — folding either folds the pair, and the two can never
drift out of step. Every row wears the sidebar tree's own glyph and colour, so a field looks the
same in both places. Three tab strips would make one question into three; a reader comparing two
tables is asking what is different *about them*, and this is that question with both answers laid
against each other. **Expand all** / **Collapse all** are on the toolbar and only in this half —
they are the same one press applied to every branch, and a tree with nothing to open has no use for
them.

The data half is the same panes with rows in them instead of a tree. **Only it can be stacked**
(`[⇄|⇅]`): two trees one above the other cannot be read across, and the ticks between them would
have nowhere to be.

**2. Row *n* of one pane is row *n* of the other.** The pairing is worked out on the Go side
(`internal/compare`), the two panes are built from that one list of pairs, a **blank** stands where
a side has not got the row, and the two **scroll as one**. The divider between them starts at half
and **moves the boundary**, so neither side is ever left holding a strip too narrow to read.

**3. A tick is what is taken, and it is a strip of its own between the two sides.** Not a column of
either pane: a tick is a fact about the *comparison* and not about either table, and a column inside
a pane slides out of sight the moment that pane is scrolled sideways — which is the one control a
reader must never have to chase. The strip moves only up and down, whatever either side is doing.
A **section's own tick takes everything under it** and the **table's takes the lot** — the same
control as a row's and not a fourth kind of button. `Take all` / `Take none` set the lot from the
toolbar. What is ticked is the whole of what `Apply` touches, and the direction control says which
of the two tables changes — the *other* one is never written to. Changing the direction, the half
being read or the row count clears the ticks, because a pick is a position in *this* comparison and
the answer under it has moved.

Stacked, there is no between, so each pane carries the tick as its own first column — and it is
still the same tick: a row is taken or it is not, and which pane the reader reached for is not a
second question.

| Colour | Means |
|---|---|
| Amber row, with the differing **cells** deeper | On both sides and not alike |
| Green row | On the **right** and not on the left |
| Red row | On the **left** and not on the right |
| Flat grey row | The blank opposite one of the two above — a fourth thing, and neither a value nor an empty one |
| No wash | The two sides agree |

The colours are `--grid-diff-*` tokens, a family of their own rather than the success and danger
colours reused: nothing here has gone wrong or gone right, and `removed` only ever means "the other
side has not got it".

**Every grid here is dense** — 22 px rows, a one-line header, a narrow gutter — because the pane
holds two grids and a script, and every pixel a row does not take is another row on the screen.
Nothing is dropped to get it: the header keeps the name and the type-kind glyph, and the whole type
is in its tooltip. See [grid.md](../grid.md).

## Script

Under the panes, **exactly what Apply will run**, numbered, copyable, and in the engine's own
spelling. It is not a description and it is not built by the frontend: the ticked rows become a list
of changes, and *that same list* is sent to be spelled and to be run
(`PreviewTableChanges` / `ApplyTableChanges`, `PreviewRowWrite` / `ApplyRowWrite`) — so the script
the reader copied is the script that ran. The data half's statements come from `RowStatements`,
which is the one way a row is written anywhere in the app, so a migration's rows and an import's are
the same statements.

The order is the designer's own: **what is going goes first**, so a constraint never stands in the
way of the field change under it; the fields move next; and what is arriving is made **last**, so an
index or a key can name a column this same run has only just added. An index and a foreign key that
merely differ are dropped and remade rather than altered, because neither engine can change which
columns one covers.

`Apply` **stops at the first statement the server rejects**, and says which. That is the honest
outcome — the order is what makes the run work, so every step after a failure is meaningless — and
there is no transaction around it, MySQL committing implicitly at every DDL statement. The preview
is what protects the reader, which is why it is there and why it is exact.

## DDL Diff

The other pane under the panes: both tables' **own declarations**, side by side and line for line.
It is the server's answer rather than a rendering of the comparison above it — MySQL is asked
`SHOW CREATE TABLE` — so it carries the storage engine, the character set and the auto-increment,
none of which a catalogue comparison can see. The comparison says what *this app* can tell apart;
this says what the two tables **are**. Lines are matched by their own content and never by position,
which is the same rule the field comparison follows.

## Structure

Fields, indexes and foreign keys, as three foldable sections under the table itself. The table and
each section say how many of the things below them differ (`3 of 7 differ`, `all 3 identical`,
`neither side has any`) so a shut one still answers the question, and the section for a constraint
neither engine has is not drawn at all. Folding the table shuts the whole comparison down to one
line, which is what makes two of them comparable at a glance.

**Everything is matched by name, never by position.** A column inserted in the middle would
otherwise report every column below it as changed — instead it is drawn *in the middle*, in the
place the right has it, with a blank opposite. Two engines spell one type two ways, so the
comparison folds case (`VARCHAR(50)` is `varchar(50)`) but never length (`varchar(50)` is not
`varchar(100)`), and **no default is not a default of the empty string**.

## Data

Rows matched on a key, with the cells that differ deepened. What the comparison was *able* to do is
under the panes, and none of it is optional:

- **Which columns it matched on.** The primary key both sides declare, or — where they share none —
  every column they share, which the line says. On that fallback a row is matched on the whole of
  itself, so nothing can come back changed, only present on one side; a warning says so rather than
  letting "no changed rows" read as a finding.
- **Which columns it left out.** Only the columns *both* sides hold are compared; the rest are
  named. A value with nothing opposite it is a difference between two shapes, and Structure is where
  that is read.
- **How far it got.** A comparison is **bounded on purpose** — both sides are held in memory at once
  and cross the bridge as one answer, so there is no `All` here the way there is on a table tab:
  that read is
  [streamed](row-limit.md#an-unbounded-read-on-a-table-tab-is-streamed-watched-and-stoppable),
  and a diff cannot be, needing both sides together. Both reads are ordered by the key before they are
  cut, so the two windows line up, and a side the read did not see the whole of says so.

Applying the data half is an **upsert** of everything ticked that the source holds — changing the
rows the key names and adding the ones it does not — and a **delete** of everything ticked that only
the table being written holds. Two writes and not one, because they are two intentions and
`RowStatements` takes one mode.

A count of each outcome sits under the panes in the colours it drew them in — it is the legend and
the tally at once. Nothing on it is clickable: **Differences only** is what narrows the panes, and a
count that also filtered would be two controls wearing one label.
