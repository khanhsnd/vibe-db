# Tables: one grid, one behaviour

**Every surface that shows rows and columns of data is `features/results/ResultGrid`.** Not a
`<table>`, not a bespoke flex layout, not a second grid component. If a new view needs to show
rows — a query result, a table's data, a plan, a diff, a column list with values — it hands
`GridData` to `ResultGrid` and gets everything below for free. Extending the grid is the way to add
a table feature; a call site that styles its own rows is a bug, because the two drift apart the day
after they are written.

The only tabular things that are *not* this grid are the object tree and the column checkbox list,
which are single-column lists of names, not data. `TableDesigner` shows a table's fields, indexes,
keys and its pending changes, and every one of those is a grid — the Fields one editable, through
`onEditCell` like any other.

## What it looks like

| Part | Rule |
|---|---|
| Header | Two lines, 40 px: the column **name** in bold mono on top, its **type** below in muted 11 px with a type-kind icon beside it. The type shown is `fullType` (`varchar(255)`), falling back to `type`; the icon comes from `columnType.tsx`. Never uppercased — the server's own casing stands. On the right of every header sits the **`⋮` options button**, and beside the name of the sorted column an arrow saying which way. |
| Row gutter | 52 px, sticky at the left, 1-based row number in muted mono, right-aligned. |
| Rows | `ROW_HEIGHT` from `shared/theme/density.ts`. Mono 13 px. Numeric columns right-align, everything else left-aligns; alignment is decided by `typeKind`, never by the value. |
| NULL | The literal `(Null)` in `--syn-null` (`text-null`), from `NULL_TEXT` — one constant, so what the grid shows and what a copy writes cannot disagree. |
| Selection | Three depths: cells inside any rectangle get a fill (`--grid-cell-selected`), the **focus** cell — the pointer's last landing, which is the live rectangle's focus — is solid `--accent` with `--fg-on-accent` text, and a row the selection holds *end to end* is banded across its width (`--grid-row-selected`). A row the selection merely touches gets the band and the accent bar **on its gutter alone**, because a selected column touches every row and must not paint the whole grid. Headers of the selected columns take the cell fill too, so a range is readable from the edges. |
| Hover | A row highlight, applied *only* to rows outside the selection. Both are background colours, so a selected row must not also carry `hover:bg-hover` or the tint flickers away under the pointer. |
| Pending edit | A warning-coloured inset ring plus warning text, from `isDirty`. It is a predicate rather than a set of keys so a call site that hides columns can answer in its own coordinates. Every cell of a row that is not on the server yet is pending, so a new row is ringed whole. |
| Row for deletion | A row that `isDeleted` answers for is banded in `--grid-row-deleted` and struck through — set on the row, so every cell inherits it — and its gutter number turns `text-danger`. Deletion wins over the selection band, because it is the more consequential fact. It is a predicate for the same reason `isDirty` is one: a call site that pages answers in its own coordinates. |

## How it behaves

- **Dragging selects cells, never text.** The body owns one pointer handler that reads the row from
  `clientY / ROW_HEIGHT` and the column by walking the width list — no per-cell mouse handlers.
  `setPointerCapture` on press means the drag survives leaving the grid, and a pointer held *past*
  an edge keeps scrolling (`EDGE_STEP` per frame) so a range can run beyond one screenful. Past the
  edge, not near it — otherwise a click landing by the bottom row moves the grid under the pointer.
  `user-select: none` is already global in `index.css`; nothing in the grid may opt back in.
- **A selection is a list of rectangles, not one.** `Selection.ranges` holds them and the **last is
  live**: **Shift**-click reopens it from its own anchor, **`Ctrl`/`Cmd`**-click leaves every
  rectangle standing and starts a new one beside them, and a plain press starts over. Together with
  **dragging in the gutter, which selects whole rows** with the columns pinned across every column,
  that is how `Ctrl` picks rows one at a time and `Shift` picks a run of them. `Ctrl` only ever adds
  — clicking an already selected row does not toggle it off, because a rectangle cannot be split.
- **`Ctrl`/`Cmd`+`C`** copies every rectangle as tab-separated rows, top to bottom, so rows picked in
  any order still copy in the order they read.
- **`Ctrl`/`Cmd`+`V` pastes from the live rectangle's top-left** — a line per row, a tab per column —
  and writes nothing past the rows or columns on screen, so a paste can only overwrite what is
  already there. The origin is the top-left and not the focus, because a row taken from the gutter
  focuses its *last* column. `(Null)` comes back as NULL, `parseCell` being the inverse of the
  `NULL_TEXT` a copy wrote, so a row copied from the gutter pastes back whole and unchanged. One
  paste, one behaviour: the header menu's own item is the same call with the column's first row as
  its origin.
- Anything acting on rows asks `selectedRows`, which answers each touched row once and in order —
  that, not a range, is what `deleteRows` takes.
- **Double-click a cell, or press `Enter`, to edit it** — but only where the call site passed
  `onEditCell`; without it the grid is read-only. The cell under a double-click is taken from the
  press that preceded it, because pointer capture retargets the click that follows.
- **Right-clicking a header selects the whole column and opens the grid's own menu**, never the
  WebView's. **The `⋮` button on the header opens that same menu** from a left click, by dispatching
  the `contextmenu` event Radix already listens for — one menu with two ways in, rather than a second
  menu that drifts from the first. The menu carries **Sort Ascending**, **Sort Descending** and
  **Remove All Sorts** where `onSortChange` was passed, then **Copy column** (the column's cells as
  one line each, the same text a `Ctrl`+`C` over that range would write), **Copy field name**, and —
  only where `onEditCell` was passed — **Paste**, which is the paste below with the column's first
  row as its origin. A call site appends its own items through `columnMenuItems`; today only
  `TableDataView` does, with **Hide column** and **Show all columns**, because hiding is its state
  and not the grid's.
- **The grid never reorders its own rows.** It reports the column and direction through
  `onSortChange` and marks the header from `sort`; who sorts is the call site, because only it knows
  where the rows come from. One column at a time — hence *Remove All Sorts* clearing the one there
  is. A sort always sends the reader back to page one, and always drops the selection.
- **Columns resize** by dragging the right edge of the header, and **double-clicking that edge fits
  the column** to the widest of its name, its type line and its cells, capped at 520 px and measured
  on a canvas against the `--font-mono` / `--text-sm` tokens. Widths are keyed by column *name*, so
  hiding a column does not reset them.
- **The grid keeps its own selection when uncontrolled.** Pass `selection` +
  `onSelectionChange` only when something outside needs the focused cell.

## The bar under it

Every grid sits above one `GridFooter`, which is where a row is *acted on* rather than read:

```
+  −  │  ✓  ✗  ↻  ■                          … whatever the surface counts …
```

- **Add** appends an empty row at the end of the grid **and selects it**, so a paste that follows the
  add means the new row and not the one that was copied; **Delete** marks the selected rows, so both
  are pending like an edited cell. **Apply** writes every pending change — edits, then inserts, then
  deletions — and reloads; **Discard** drops them all. **Refresh** reloads alone. **Stop** appears
  only where there is something to cancel, which today is a running statement.
- **The four editing icons are hidden outright when the rows cannot be written back**, rather than
  shown disabled: a result that no `UPDATE` can name has no use for them. `Refresh` always shows.
- Delete is the only one that needs a selection; Apply and Discard need something pending.
- The right-hand side is the caller's, and both surfaces hand it the same `GridPager`: arrows, the
  offset window, the page count and the `Rows` size. Only where the rows come from differs — the
  table view walks the server, and a query result is paged inside `usePageWindow` out of the rows
  it already holds. A `+` on the pager's total says those rows are a *prefix* of the result, and
  `partial` is also what keeps `Next` alive on the last page: the caller can go and fetch the rest.
- The bar carries `--surface-footer` rather than `--surface-panel`, so it reads as controls and not
  as the status line that may sit directly below it.
- **Its buttons refuse focus** (`mousedown` is prevented), so pressing one leaves the keyboard on the
  grid. Add-then-paste is one gesture, and the keystroke would otherwise land on the button.

`useGridEdits` is the state behind it, and it is the same hook on both surfaces: pending cell
edits, appended draft rows, and rows marked for deletion, all keyed by row number and therefore all
dropped when a fresh read lands. An insert carries **only the cells that were filled in**, so a
column left alone takes the server's own default rather than an explicit NULL.

## When you add to it

1. New colour goes in `shared/theme/tokens.css` as a `--grid-*` token with a light-theme override
   and an `@theme inline` mapping — never a hex or a raw `var()` in the component.
2. Geometry constants stay at the top of `ResultGrid.tsx`; row height alone is shared, via
   `density.ts`, because the object tree virtualizes on the same number.
3. New behaviour is added once, in the grid, and this file is updated in the same change. Every call
   site — `ResultPane`, `TableDataView` and `TableDesigner` — then has it, which is the whole point.

## Where the two call sites legitimately differ

Not in what an edit means — `useGridEdits` and `GridFooter` make that identical — only in what
surrounds the grid:

- `ResultPane` pages in memory, so it maps the grid's *page-local* row index back to the whole
  result's on the way in — for edits, `isDirty`, `isDeleted` and a delete's rows alike. **It sorts in
  memory too**, and for the same reason keeps the sort as a list of row numbers (`sortedOrder`)
  rather than reordered rows: a pending change is keyed by the number a row had when it was read.
  Draft rows stay pinned after every fetched row, so **Add** still lands on the last page. A sort
  only reaches the rows already fetched, and it is dropped when the statement changes. Its footer
  can also **Stop** the run, and **Add** jumps to the last page, which is where a draft row lands.
  Asking for a page past the rows it holds replays the statement for as many as that page needs
  (`reach`), so `All` really does load all; the page number therefore survives a read of the same
  statement, and only a different one sends the reader back to page one.
- `TableDesigner` neither pages nor hides, so it maps nothing — its rows are the fields on screen.
  What differs is that its cells are a *declaration* rather than stored data, so an edit is parsed
  back into the field it describes (`Length` is part of the type) and `isDirty` answers by comparing
  the staged field with the stored one, rather than from a set of pending keys.
- `TableDataView` pages on the server, and can hide columns, so it maps the grid's *visible* column
  index back to the real one on the way in. **It sorts on the server too** — the sort travels in
  `TablePageRequest` and becomes the read's `ORDER BY`, because ordering the hundred rows in hand
  would only shuffle what the reader can already see. Its Cell Editor panel stages a value on the focused cell
  exactly as the grid's own inline editor does; the footer still commits. It is also the only site
  that adds to the header menu, since `ColumnPicker` and that menu toggle one set of hidden names.

Both mappings exist because the grid always reports the coordinates it drew, never the ones the
server knows. Neither one lets the *other* axis drift: a paged surface leaves columns alone and a
column-hiding surface leaves rows alone.
