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
| Head strip | 32 px above the header, on the header's own surface — the two read as one head — holding what acts on the grid *whole* rather than row by row, today the **Export** button alone against the right edge: a labelled `Button` with an accent-coloured glyph, not a bare icon, because it has the room to say what it is. It sits outside the scroller, so it stays put while the columns scroll under it, and the grid draws it **only where the call site passed `onExport`** — a surface with a toolbar of its own puts the same `ExportMenu` in that toolbar instead and passes nothing, because one table must never carry two strips. |
| Header | Two lines, 40 px on `--surface-bar`, so the shade alone says where the data starts. The column **name** on top in the **UI font**, bold 14 px — the one place the grid is not mono, a size above its cells and three weights clear of their regular, so a name reads as a label and not as the data beneath it — and its **type** below in muted mono 11 px with a type-kind icon beside it. The type shown is `fullType` (`varchar(255)`), falling back to `type`; the icon comes from `columnType.tsx`. Never uppercased — the server's own casing stands. On the right of every header sits the **`⋮` options button**, and beside the name of the sorted column an arrow saying which way. |
| Row gutter | 52 px, sticky at the left, 1-based row number in muted mono, right-aligned. |
| Rows | `ROW_HEIGHT` from `shared/theme/density.ts`. Mono 13 px. Numeric columns right-align, everything else left-aligns; alignment is decided by `typeKind`, never by the value. |
| NULL | The literal `(Null)` in `--syn-null` (`text-null`), from `NULL_TEXT` — one constant, so what the grid shows and what a copy writes cannot disagree. |
| Selection | Three depths: cells inside any rectangle get a fill (`--grid-cell-selected`), the **focus** cell — the pointer's last landing, which is the live rectangle's focus — is solid `--accent` with `--fg-on-accent` text, and a row the selection holds *end to end* is banded across its width (`--grid-row-selected`). A row the selection merely touches gets the band and the accent bar **on its gutter alone**, because a selected column touches every row and must not paint the whole grid. Headers of the selected columns take the cell fill too, so a range is readable from the edges. |
| Hover | A row highlight, applied *only* to rows outside the selection. Both are background colours, so a selected row must not also carry `hover:bg-hover` or the tint flickers away under the pointer. |
| Column editors | A column may say how its cells are drawn and changed, through `ResultColumn.editor`. A **`choice`** cell keeps its text and gains a muted chevron; a **`check`** cell is a centred box — filled `--accent` when on, an outline in `--fg-faint` when off — and never the word it holds; a **`key`** cell is a `--grid-key` lock followed by the cell's own value, which is the field's place in the key, and is empty when it holds nothing. A toggle cell shows **nothing** rather than `(Null)`, a no being a no. |
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
- **One click opens the cell**, and `Enter` does the same to the focused one — but only where the
  call site passed `onEditCell`; without it the grid is read-only. Nothing is sent by opening an
  editor, so a second click to reach a value the reader is already pointing at buys nothing. A
  click that was reaching for a *range* never opens anything: not one in the row gutter, not one
  held with `Shift`, `Ctrl` or `Cmd`, and not the end of a drag.
- **A double-click opens the row**, where the call site passed `onActivateRow` — the table list's
  rows stand for tables, and opening one is what two clicks mean everywhere outside a writable
  grid. It is read off the same press a click is, and a drag is not a double-click. Only a
  read-only grid is offered it: in a writable one the first click has already opened an editor and
  the second would land inside it.
- **`Escape` closes the editor and hands the keyboard back to the grid**, which is how a reader
  gets from editing to a plain selection — the cell stays focused and the arrows move again. It is
  the only way back, one click being the way in.
- **A `choice` column opens a searchable list instead of a plain box.** Typing filters the options
  and the arrows walk them; `Enter` takes the highlighted one, or **what was typed** where nothing
  is highlighted, because a column's options are what it is usually given and not all a server will
  take. The list is portalled and fixed — the grid scrolls, and would otherwise clip it — so it is
  told where its cell is again on every scroll, rather than dismissed under a wheel.
- **A temporal cell's box carries a `⋮`, and it opens the platform's own picker.** Which picker is
  read off the column's *type* rather than named by the call site, exactly as alignment is: a
  calendar for a `DATE`, a clock for a `TIME`, both for a `DATETIME` or a `TIMESTAMP`, and none at
  all for an `INTERVAL`, which is a length of time and not a moment. What is picked goes back into
  the box as text with SQL's own separator, so it can still be typed over and the footer still
  commits it, and a value the picker cannot read — an expression, a NULL — opens it empty and leaves
  the text alone until something is picked. A picker reaches seconds and no further, so picking over
  a value that carried a fraction of a second or a time zone writes it back without one — the box is
  there to type either back in. The panel carries **Now** — this moment in the reader's own zone, to
  the granularity the column keeps — and **Done**, though leaving the panel commits just as Done
  does, the value being in the box already. `Escape` closes the picker and leaves the box open,
  which is one step back rather than two. The panel is anchored like the `choice` list and by the
  same `useCellAnchor`.
- **A `check` or `key` column is flipped by that same single click, never opened.** A yes/no has
  nothing to type into it, so `Enter` toggles it too. Turning one on writes `CHECK_ON`, turning it
  off writes an empty cell; **what a `key` cell then shows is the call site's**, which is how a key
  column numbers its fields — the grid only says which cell was flipped.
- **The arrow keys walk the focus a cell at a time**, and never scroll the grid on their own: the
  focused cell moves, the selection becomes that one cell, and the view follows only far enough to
  keep it clear of the sticky header and the row gutter. **`Shift`+arrow** stretches the live
  rectangle to it instead, which is the keyboard's version of a drag. An open editor owns the
  keyboard, so its own arrows walk its text or its list.
- **Everything a click does is read off the press it belongs to** (`Hit`), never measured again
  when the click arrives: the press also focuses the grid, focusing can scroll it, and a second
  measurement would then name a different cell — which once cost the *first* click on a fresh grid
  entirely. A drag is not a click on the cell it began on, so the hit carries a `moved` flag that
  `extend` sets the moment the drag leaves that cell.
- **Right-clicking a header selects the whole column and opens the grid's own menu**, never the
  WebView's. **The `⋮` button on the header opens that same menu** from a left click, by dispatching
  the `contextmenu` event Radix already listens for — one menu with two ways in, rather than a second
  menu that drifts from the first. The menu carries **Sort Ascending**, **Sort Descending** and
  **Remove All Sorts** where `onSortChange` was passed, then **Copy column** (the column's cells as
  one line each, the same text a `Ctrl`+`C` over that range would write), **Copy field name**, and —
  only where `onEditCell` was passed — **Paste**, which is the paste below with the column's first
  row as its origin. A call site appends its own items through `columnMenuItems`; today only
  `TableDataView` does, with **Hide column** and **Show all columns**, because hiding is its state
  and not the grid's. Every item carries a glyph from `shared/ui/menuIcons.tsx`, which is where the
  icon for an action lives so the same action wears the same one in every menu.
- **The grid never reorders its own rows.** It reports the column and direction through
  `onSortChange` and marks the header from `sort`; who sorts is the call site, because only it knows
  where the rows come from. One column at a time — hence *Remove All Sorts* clearing the one there
  is. A sort always sends the reader back to page one, and always drops the selection.
- **Columns resize** by dragging the right edge of the header, and **double-clicking that edge fits
  the column** to the widest of its name, its type line and its cells, capped at 520 px and measured
  on a canvas against the tokens each is drawn with — `--font-ui` / `--text-base` for the name,
  `--font-mono` / `--text-sm` for the rest. Widths are keyed by column *name*, so hiding a column
  does not reset them.
- **The grid keeps its own selection when uncontrolled.** Pass `selection` +
  `onSelectionChange` only when something outside needs the focused cell.
- **Export belongs over the grid, never in the footer**, because it acts on the whole grid rather
  than on a row: `ExportMenu` opens CSV, JSON and SQL, each writing what the grid holds to a file
  picked in the system's own save dialog. It is one component in two places — the head strip the
  grid draws for itself, or the surface's own toolbar, where `TableDataView` sets it beside the
  **Cell Editor** and **Column** toggles and passes the grid no `onExport`, so a table carries one
  row of controls rather than two stacked. It is on the right because that is where a reader looks
  for what to *do with* the table, the left of the footer being where a row is changed. What each
  format does with a `NULL`, and which rows a surface counts as "what the grid holds", are in
  [features.md](features.md#exporting-a-grid).

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
  Export is not here at all — it belongs to the grid's head strip, above. Their four handlers are
  optional for the same reason the icons are: a grid that names itself read-only passes none rather
  than four callbacks that can never run.
- Delete is the only one that needs a selection; Apply and Discard need something pending.
- The right-hand side is the caller's, and both surfaces hand it the same `GridPager`: arrows, the
  offset window, the page count and the `Rows` size. Only where the rows come from differs — the
  table view walks the server, and a query result is paged inside `usePageWindow` out of the rows
  it already holds. A `+` on the pager's total says those rows are a *prefix* of the result, and
  `partial` is also what keeps `Next` alive on the last page: the caller can go and fetch the rest.
- The bar carries `--surface-bar` rather than `--surface-panel`, so it reads as controls and not
  as the status line that may sit directly below it. Every strip of controls uses that surface, the
  toolbars and the head strip above a grid included; a surface's own toolbar and that head strip are
  the one pair meant to meet edge to edge, being controls over the same table.
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
   site — `ResultPane`, `TableDataView`, `TableDesigner` and `ObjectListView` — then has it, which
   is the whole point.

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
- `TableDesigner` neither pages nor hides, so it maps nothing — its rows are whatever the pane in
  front holds. It is the one site that draws **three** editable grids and switches between them, so
  it holds the tab strip's selection itself and clears the selection on every change: a cell
  reference is a position in the pane that reported it.
  What differs is that its cells are a *declaration* rather than stored data, so an edit is parsed
  back into the thing it describes (`Length` is part of the type; `Fields` is a comma-separated list)
  and `isDirty` answers by comparing what is staged with what is stored, rather than from a set of
  pending keys. A field is ringed cell by cell; an index or a foreign key is ringed **whole**,
  because neither engine can alter one and it is dropped and remade rather than changed in parts. It
  is also the one site that names column editors: `Type` and `Method` are a `choice` over the
  engine's own lists, `On Update` and `On Delete` a `choice` over the referential actions,
  `Not null` and `Unique` a `check`, and `Key` a `key` whose numbers it counts down the draft.
- `ObjectListView` neither pages, hides nor edits: its rows are one namespace's tables and it maps
  nothing. It is the one site whose rows *stand for* something openable, so it is the one that
  passes `onActivateRow`, and the only read-only site with a toolbar of its own — over the grid,
  where a surface's controls go, acting on the table the selection names.
- `TableDataView` pages on the server, and can hide columns, so it maps the grid's *visible* column
  index back to the real one on the way in. **It sorts on the server too** — the sort travels in
  `TablePageRequest` and becomes the read's `ORDER BY`, because ordering the hundred rows in hand
  would only shuffle what the reader can already see. Its Cell Editor panel stages a value on the focused cell
  exactly as the grid's own inline editor does; the footer still commits. It is also the only site
  that adds to the header menu, since `ColumnPicker` and that menu toggle one set of hidden names.

Both mappings exist because the grid always reports the coordinates it drew, never the ones the
server knows. Neither one lets the *other* axis drift: a paged surface leaves columns alone and a
column-hiding surface leaves rows alone.
