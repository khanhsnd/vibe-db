# The table tab

[← Features and UI behaviour](../features.md)

**Double-clicking** a table opens a data tab; a single click only selects it, and reads nothing from
the server. **Until the first page lands the pane says so** — `Loading <table>…` under a spinner,
from `shared/ui/Loading` — because a grid with no columns yet is a blank pane and a table of any
size can take seconds to answer. Only the *first* read gets it: once rows are on screen a re-read
leaves them there and the pager alone says it is working, which is what keeps a page turn from
blinking.

**A page in hand is read once.** Looking at another tab and coming back reads nothing: the rows are
the rows the reader left, and so is the row they had added and not yet applied — see
[what a tab keeps](shell.md#tabs). What sends for more is a **changed question** — another page,
another limit, another order, another filter, or the connection dialled again — or the **`↻`** under
the grid, which is the same Refresh every grid's footer has ([grid.md](../grid.md#the-bar-under-it)).
The status line's `Last Refresh` is therefore the age of what is on screen and means it.

A tab loads 100 rows per page by default, ordered by primary key so paging is stable —
until a header's `⋮` menu, or the Filter & Sort panel, gives it keys of its own; those travel into the
read's `ORDER BY` and therefore order the whole table rather than the page on screen.
**Paging lives on the right of the bar under the grid** — first, back, the page typed into a box,
forward, last — with a **gear** beside it that opens `Limit records` and the count that sizes a page,
and can lift the bound entirely; see [the row limit](row-limit.md). **Under that bar is the status
line**: the read that produced the rows with a copy beside it, where the reader is in what it
returned, and how long ago it landed. The left of the control bar is where rows are added, deleted,
applied and discarded: it is the same `GridFooter` every grid has, and [grid.md](../grid.md)
specifies both it and the line under it. Above the grid is one toolbar: **Export** against its right
edge, **Filter & Sort** on the left of it — see [filtering and sorting](filter-sort.md) — and before
that two independent toggles, since they occupy different edges:

**Both panels are dragged, and the size is remembered.** The handle between a panel and the grid is
the divider itself and lights up `--accent` under the pointer; the bounds stop a drag leaving either
panel too small to read or the grid with nothing left. A panel's width or height is kept per
*combination of open panels*, so the list the reader widened is that width the next time they open
it — see [frontend.md](../frontend.md) on `panelIds`.

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft and
  Apply stages the value on the cell, exactly as opening it in the grid does — the footer's
  ✓ is what writes `UPDATE … WHERE <primary key>` and reloads. A table with no primary key is
  read-only, because no single row can be identified. MySQL adds a `LIMIT 1` belt to the braces;
  PostgreSQL has no such clause and does not need one, since the key already names one row.
  **A query result has the same panel**, from the button on its own head strip beside Export: it is
  one `CellEditor` on both surfaces, and the line under it says why a value cannot be written back
  where it cannot — a table with no primary key here, a result no `UPDATE` can name there.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes. **Right-clicking a column
  header hides or restores columns too**, alongside the copy and paste items every grid's header menu
  has — one hidden set, either way in. [grid.md](../grid.md) specifies that menu.
- **A column is dragged into place by its header**, and where a table's columns were put is
  **remembered against that table** — which connection, which namespace, which table — so opening it
  again next week, or after the app has been closed, opens it arranged. A query result is arranged
  the same way but remembers nothing: there is no stored table under it to remember one against, and
  its order goes with its statement, exactly as its sort does. The order is kept by column *name*,
  so a column added to the table since, or one dropped from it, moves nothing else.
  [grid.md](../grid.md) specifies the drag.
