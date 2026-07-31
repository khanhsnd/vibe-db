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
| Pending edit | A warning-coloured inset ring plus warning text, from the `dirty` set. |

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

## When you add to it

1. New colour goes in `shared/theme/tokens.css` as a `--grid-*` token with a light-theme override
   and an `@theme inline` mapping — never a hex or a raw `var()` in the component.
2. Geometry constants stay at the top of `ResultGrid.tsx`; row height alone is shared, via
   `density.ts`, because the object tree virtualizes on the same number.
3. New behaviour is added once, in the grid, and this file is updated in the same change. Both call
   sites — `ResultPane` and `TableDataView` — then have it, which is the whole point.

## Where the two call sites legitimately differ

Only in what an edit *means*, never in how the grid looks or reads:

- `ResultPane` holds edits locally in `useResultEdits` and marks them `dirty` until **Apply** writes
  them, because a query result is a snapshot the user may be poking at.
- `TableDataView` writes each edit at once through `useTableData.updateCell` and reloads, matching
  its Cell Editor panel. It also maps the grid's *visible* column index back to the real one, since
  the column picker can hide columns.
