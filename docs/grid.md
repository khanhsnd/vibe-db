# Tables: one grid, one behaviour

**Every surface that shows rows and columns of data is `features/results/ResultGrid`.** Not a
`<table>`, not a bespoke flex layout, not a second grid component. If a new view needs to show
rows — a query result, a table's data, a plan, a diff, a column list with values — it hands
`GridData` to `ResultGrid` and gets everything below for free. Extending the grid is the way to add
a table feature; a call site that styles its own rows is a bug, because the two drift apart the day
after they are written.

The only tabular things that are *not* this grid are the object tree and the column checkbox list,
which are single-column lists of names, not data.

## What it looks like

| Part | Rule |
|---|---|
| Header | Two lines, 40 px: the column **name** in bold mono on top, its **type** below in muted 11 px with a type-kind icon beside it. The type shown is `fullType` (`varchar(255)`), falling back to `type`; the icon comes from `columnType.tsx`. Never uppercased — the server's own casing stands. |
| Row gutter | 52 px, sticky at the left, 1-based row number in muted mono, right-aligned. |
| Rows | `ROW_HEIGHT` from `shared/theme/density.ts`. Mono 13 px. Numeric columns right-align, everything else left-aligns; alignment is decided by `typeKind`, never by the value. |
| NULL | The literal `(Null)` in `--syn-null` (`text-null`), from `NULL_TEXT` — one constant, so what the grid shows and what a copy writes cannot disagree. |
| Selection | Three depths: every row the selection touches gets a soft band (`--grid-row-selected`) and an accent bar down its gutter, cells inside the rectangle get a stronger fill (`--grid-cell-selected`), and the **focus** cell — the one the pointer last landed on — is solid `--accent` with `--fg-on-accent` text. Headers of the selected columns take the cell fill too, so a range is readable from the edges. |
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
- **Shift-click extends** from the existing anchor. **Dragging in the gutter selects whole rows**,
  with the column range pinned across every column.
- **`Ctrl`/`Cmd`+`C`** copies the rectangle as tab-separated rows.
- **Double-click a cell, or press `Enter`, to edit it** — but only where the call site passed
  `onEditCell`; without it the grid is read-only. The cell under a double-click is taken from the
  press that preceded it, because pointer capture retargets the click that follows.
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

- **Add** appends an empty row at the end of the grid; **Delete** marks the selected rows, so both
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

`useGridEdits` is the state behind it, and it is the same hook on both surfaces: pending cell
edits, appended draft rows, and rows marked for deletion, all keyed by row number and therefore all
dropped when a fresh read lands. An insert carries **only the cells that were filled in**, so a
column left alone takes the server's own default rather than an explicit NULL.

## When you add to it

1. New colour goes in `shared/theme/tokens.css` as a `--grid-*` token with a light-theme override
   and an `@theme inline` mapping — never a hex or a raw `var()` in the component.
2. Geometry constants stay at the top of `ResultGrid.tsx`; row height alone is shared, via
   `density.ts`, because the object tree virtualizes on the same number.
3. New behaviour is added once, in the grid, and this file is updated in the same change. Both call
   sites — `ResultPane` and `TableDataView` — then have it, which is the whole point.

## Where the two call sites legitimately differ

Not in what an edit means — `useGridEdits` and `GridFooter` make that identical — only in what
surrounds the grid:

- `ResultPane` pages in memory, so it maps the grid's *page-local* row index back to the whole
  result's on the way in — for edits, `isDirty`, `isDeleted` and a delete's range alike. Its footer
  can also **Stop** the run, and **Add** jumps to the last page, which is where a draft row lands.
  Asking for a page past the rows it holds replays the statement for as many as that page needs
  (`reach`), so `All` really does load all; the page number therefore survives a read of the same
  statement, and only a different one sends the reader back to page one.
- `TableDataView` pages on the server, and can hide columns, so it maps the grid's *visible* column
  index back to the real one on the way in. Its Cell Editor panel stages a value on the focused cell
  exactly as the grid's own inline editor does; the footer still commits.

Both mappings exist because the grid always reports the coordinates it drew, never the ones the
server knows. Neither one lets the *other* axis drift: a paged surface leaves columns alone and a
column-hiding surface leaves rows alone.
