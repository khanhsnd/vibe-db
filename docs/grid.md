# Tables: one grid, one behaviour

**Every surface that shows rows and columns of data is `features/results/ResultGrid`.** Not a
`<table>`, not a bespoke flex layout, not a second grid component. If a new view needs to show
rows — a query result, a table's data, a plan, a diff, a search's hits, a column list with values — it hands
`GridData` to `ResultGrid` and gets everything below for free. Extending the grid is the way to add
a table feature; a call site that styles its own rows is a bug, because the two drift apart the day
after they are written.

The only tabular things that are *not* this grid are the object tree and the column checkbox list,
which are single-column lists of names, not data. `TableDesigner` shows a table's fields, indexes,
keys and its pending changes, and every one of those is a grid — the Fields one editable, through
`onEditCell` like any other.

**A transcript is not a table, and the rule does not reach it.** The console's log and
[Execute SQL File](features.md#execute-sql-file)'s both hold one statement and what it came to, and a
statement is laid out over as many lines as its author wrote it over while an error is a paragraph the
server wrote. Neither fits a cell: put in one they become the widest column of the set, clipped, and
the thing the reader opened the surface to read is the thing they cannot see. The test is what the
*unit* is — a row of fields, or a passage of text — and not whether it could be tabulated.

## What it looks like

| Part | Rule |
|---|---|
| Head strip | 32 px above the header, on the header's own surface — the two read as one head — holding what acts on the grid *whole* rather than row by row: **Filter & Sort** on the left, where a reader looks for what narrows the table, and against the right **Cell Editor** then **Export**, where they look for what to *do with* it. All three are labelled `Button`s with an accent-coloured glyph, not bare icons, because the strip has the room to say what they are. It sits outside the scroller, so it stays put while the columns scroll under it, and the grid draws it **where the call site passed `onExport`, `filtering` or `cellPanel` and did not pass `headStrip={false}`** — a surface with a toolbar of its own puts the same buttons in that toolbar and turns the strip off, because one table must never carry two strips. It still passes the handlers, which is what the cell menu's own items run. Today the one surface that filters is the table view, and its own toolbar carries all three, so the strip the grid draws for itself is a query result's: **Cell Editor** and **Export**. |
| Filter & Sort panel | Under the head strip and over the header, drawn only where the call site passed `filtering` and the reader asked for it — the **Filter & Sort** button, the cell menu's **Show/Hide Filter & Sort**, or a header menu's **Add Filter**. It is `FilterSortPanel`, and it is the whole of what the section below describes. It is **dragged rather than fixed**: a grid that can be narrowed puts the panel and the rows in a `Split` of its own, so a reader who opened the panel still says how much of the pane the rows keep, and the handle between them is the only divider — the panel carries no bottom border beside it. The conditions take the slack, which is what makes dragging the handle worth doing; the keys scroll rather than push **Apply** off a panel dragged short. The height is remembered per combination like every other panel, and a grid the call site passed no `filtering` has no splitter at all. The conditions hold whether it is showing or not: hiding the panel is not widening the grid, which is why the button carries a **count** of the conditions and sort keys in force. |
| Header | Two lines, 40 px on `--surface-bar`, so the shade alone says where the data starts. The column **name** on top in the **UI font**, bold 14 px — the one place the grid is neither mono nor the reader's own font, a size above its cells and weights clear of their light, so a name reads as a label and not as the data beneath it — and its **type** below in muted mono 11 px with a type-kind icon beside it. The type shown is `fullType` (`varchar(255)`), falling back to `type`; the icon comes from `columnType.tsx` and is drawn in **that class's own colour** (`--type-*`), so a wide header is read as shapes and colours rather than by spelling out seven type names. Never uppercased — the server's own casing stands. A column that is part of the **primary key** carries the tree's own `--grid-key` lock ahead of its name, from `ResultColumn.primaryKey`, so a result the driver could not trace back to a table simply has none. **Nullability is not drawn here**, and must not be: a driver that cannot report it says `false`, which the grid cannot tell from a column the server really will not take NULL for — the designer reads the catalogue and can. On the right of every header sits the **`⋮` options button**, and beside the name of a sorted column an arrow saying which way — followed by **which key it is** where more than one is in force, so the header under the second key does not claim the order for itself. |
| Row gutter | 52 px, sticky at the left, 1-based row number in muted mono, right-aligned. |
| Density | Two sets of numbers and no more. The **roomy** one every grid takes, and the **dense** one a grid drawn to be *scanned* asks for with `dense`: 22 px rows, a 24 px one-line header, a 34 px gutter (`density.ts`). Nothing is dropped to get it — the header keeps the name and the type-kind glyph, and its whole type moves into the header's tooltip. The comparison is the surface that asks: it holds two grids and a script in one pane, so every pixel a row does not take is another row on the screen. |
| Rows | `ROW_HEIGHT` from `shared/theme/density.ts`, or the dense one where the call site asked. The **data font** — the `.data-font` class, which is the family, size and weight the reader picked in Settings, mono 13 px light by default. Numeric columns right-align, everything else left-aligns; alignment is decided by `typeKind`, never by the value. |
| Long values | A cell draws at most `PREVIEW_LIMIT` characters and then an ellipsis — `cellPreview`. A cell is a couple of hundred pixels wide, so the rest was never readable, and the browser still shapes every character of it to find where the ellipsis goes: **two columns of JSON documents made a 200-row table lag on text layout alone.** Nothing that *acts* on a value is clipped — a copy, an export, a filter's own condition and **View Value** all carry the whole of it — and a fit measures the preview because `MAX_FIT_WIDTH` caps a column far below that many characters anyway. |
| NULL | The literal `(Null)` in `--syn-null` (`text-null`), from `NULL_TEXT` — one constant, so what the grid shows and what a copy writes cannot disagree. |
| Bytes | A value that is not text arrives as `\x…` hex and is drawn as that text, like any other. The grid has no special case for it: **View Value** is where a blob becomes a picture or a dump, and a row of them reads as a row of lengths rather than as replacement characters. Why it arrives that way is in [features.md](features.md#why-bytes-arrive-as-x). |
| Documents | A nested document or an array arrives as the shell's own notation — `{ city: "Hanoi" }`, `[1, "two"]` — and is drawn as that text. The grid has no special case for it either: **View Value** is where one is read whole, in the **Document** view, and the **Cell Editor** panel is where one is *changed* in that same view — the notation is the parser's own, so a cell typed into it goes back as the document it showed, and nobody is sent to `Raw` to edit what `Raw` does not lay out. Laying it out is the **engine's**, through the `formatValue` a call site with a session passes; a grid with none offers `Raw` alone rather than a layout it cannot vouch for. See [features.md](features.md#reading-a-value-whole). |
| Selection | Three depths: cells inside any rectangle get a fill (`--grid-cell-selected`), the **focus** cell — the pointer's last landing, which is the live rectangle's focus — is solid `--accent` with `--fg-on-accent` text, and a row the selection holds *end to end* is banded across its width (`--grid-row-selected`). A row the selection merely touches gets the band and the accent bar **on its gutter alone**, because a selected column touches every row and must not paint the whole grid. Headers of the selected columns take the cell fill too, so a range is readable from the edges. |
| Hover | A row highlight, applied *only* to rows outside the selection. Both are background colours, so a selected row must not also carry `hover:bg-hover` or the tint flickers away under the pointer. |
| Outlined column | A column may draw its cells as **the rows of a tree**, through `ResultColumn.outline`: indented by how deep the row sits, with a caret on a row that holds others and a glyph saying what it is. It is a *column* and not a mode of the grid, because that is what it is — one column carries the hierarchy and every other column beside it stays a plain fact about the row. The rows are still a flat list in the order they read, and **the call site is what flattens the tree**: the grid neither holds a shape nor decides what is open. The caret stops its own press, for the reason the resize handle does — a drag is read off the body, and a caret that let one through would start selecting cells every time a branch was opened. A leaf keeps the caret's width, so every name on one level starts on one line. |
| Column editors | A column may say how its cells are drawn and changed, through `ResultColumn.editor`. A **`choice`** cell keeps its text and gains a muted chevron; a **`check`** cell is a centred box — filled `--accent` when on, an outline in `--fg-faint` when off — and never the word it holds; a **`key`** cell is a `--grid-key` lock followed by the cell's own value, which is the field's place in the key, and is empty when it holds nothing. A toggle cell shows **nothing** rather than `(Null)`, a no being a no. |
| Pending edit | A warning-coloured inset ring plus warning text, from `isDirty`. It is a predicate rather than a set of keys so a call site that hides columns can answer in its own coordinates. Every cell of a row that is not on the server yet is pending, so a new row is ringed whole. |
| Row for deletion | A row that `isDeleted` answers for is banded in `--grid-row-deleted` and struck through — set on the row, so every cell inherits it — and its gutter number turns `text-danger`. Deletion wins over the selection band, because it is the more consequential fact. It is a predicate for the same reason `isDirty` is one: a call site that pages answers in its own coordinates. |
| Row of a comparison | A row `markRow` answers for is washed in its own `--grid-diff-*` colour and its gutter carries a bar of that colour: **added** (on the right and not the left), **removed** (the other way about), **changed**, and **absent** — the blank standing opposite a row this side has not got, which is a fourth thing and neither a value nor an empty one. A row the two sides agree about is not marked at all. On a `changed` row, every cell `markCell` answers for is drawn deeper still (`--grid-diff-cell`), so a row of forty columns says *which* of them moved. They are their own colour family and not the success and danger ones reused: nothing about a comparison has gone wrong or gone right. Deletion wins over a mark, and no surface is ever both — a comparison is read-only. |

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
  **Remove All Sorts** where `onSortChange` was passed, **Add Filter** where `filtering` was — it
  opens the Filter & Sort panel with a condition on that field in its draft, a header knowing the
  field and not the value — then **Copy column** (the column's cells as
  one line each, the same text a `Ctrl`+`C` over that range would write), **Copy field name**, and —
  only where `onEditCell` was passed — **Paste**, which is the paste below with the column's first
  row as its origin. A call site appends its own items through `columnMenuItems`; today only
  `TableDataView` does, with **Hide column** and **Show all columns**, because hiding is its state
  and not the grid's. Every item carries a glyph from `shared/ui/menuIcons.tsx`, which is where the
  icon for an action lives so the same action wears the same one in every menu.
- **Right-clicking a *cell* opens the cell menu**, which is the longest menu in the app and lives in
  `cellMenu.ts` rather than in the grid — the grid is large enough, and every item there acts through
  a prop the grid was handed, so nothing in it knows a call site. A press **outside** the selection
  replaces it with the cell under the pointer; one **inside** leaves every rectangle standing, so a
  range the reader picked can be acted on whole. A press in the row gutter takes the whole row, as a
  left one does. The cell is remembered (`menuCell`) rather than read off the focus, because a
  right-click inside a selection must not move it.
  | Group | Items |
  |---|---|
  | **View Value**, always | Opens `ValueDialog` on the cell under the pointer — the same `ValueViewer` a Cell Editor panel draws, read-only. It is first in the menu and is the one item **every** grid offers: it reads the cell and changes nothing, and the designer, the table list and the history log have no cell-editor panel to put it in. What a value is read *as* — JSON, a document, GeoJSON, an array, bytes — is decided by the value and its **type**, never by an engine's name; see [features.md](features.md#reading-a-value-whole) |
  | Writing, where `onEditCell` was passed | **Set to Empty String**, **Set to NULL**, **Generate UUID** ▸ (lowercase, uppercase, without hyphens) — each acting on **every cell of the selection**, and the UUID minting a fresh one per cell so a range of rows gets a range of ids |
  | **Edit in Cell Editor**, where `onEditInPanel` was passed | Opens the surface's own editor panel on that cell, which both writable surfaces do by selecting the cell and showing the panel |
  | **Delete Record**, where `onDeleteRows` was passed | The same call the footer's Delete makes, on the rows the selection touches; it says *Delete 3 Records* when it means three, and it is the one item drawn `danger` |
  | Clipboard | **Copy** (the `Ctrl`+`C` text), **Copy Field Name**, **Copy As** ▸ (tab-separated with field names, CSV, JSON — `copyAs.ts`, treating a NULL exactly as the file export does), **Paste** where `onEditCell` was passed, and **Save Data As** ▸ (the `ExportMenu` formats) where `onExport` was |
  | Narrowing | **Show/Hide Filter & Sort**, **Filter** ▸ and **Remove All Filters & Sorts** where `filtering` was passed, and **Sort** ▸ (ascending, descending, remove) where `onSortChange` was. **Filter** ▸ offers every operator against the value under the pointer — `= 30`, `contains shop` — and the condition it adds **applies at once**, having been picked whole rather than spelled; it lands at the end of the root. An operator taking two values or a list cannot be read off one cell, so it arrives with its value unset and the panel opens on it. The **label** is clipped where the value is long — a JSON document spelled out in a menu item pushes the menu off the screen — but the condition itself carries the whole value |
  | **Refresh**, where `onRefresh` was passed | Reads the rows again — the footer's own Refresh, reachable without travelling to the bar |
  An absent handler leaves its item **off** rather than showing it dead, which is the rule
  `onEditCell` already set for the whole grid; a grid with **no rows** offers `Refresh` alone,
  because every other item would name a row that is not there.
- **A surface may raise a different cell menu, through `cellMenu`.** It is handed exactly the context
  the built-in one is, so an override is a *shorter menu* and never a second set of handlers —
  everything it can act through is still a prop the grid was given. It exists for the one surface
  where the built-in menu is wrong nearly throughout rather than in one item: a **key space**, which
  has no NULL to set on a key that would then not exist, no UUID worth minting into one, no condition
  to add — [its own bar](features.md#a-key-space-is-searched-not-filtered) is where it is narrowed —
  and no sort worth offering over a page cut from a sorted scan. A flag per absent item would have
  been eight flags. `features/table/keyMenu.ts` is the one override there is.
- **A filter is a tree, not a list** (`filter.ts`). Its leaves are **conditions** — a column, an
  operator and as many values as that operator takes — and its branches are **groups**, each joining
  its children by `and` or by `or`. The root is a group, and a tree rather than a list because a
  reader who means `a AND (b OR c)` cannot say it in a list; those parentheses are the whole point
  of a group, and the driver puts them in for a group nested inside another and nowhere else. The
  operator travels as a **token**, never as the operator's own spelling: what turns `gt` into `>` is
  the driver, so nothing a reader typed reaches a statement as SQL.
  **Who applies the tree is the call site**, and today that is `TableDataView` alone: it puts the
  tree in the read's `WHERE` and counts its total through it. A surface holding its rows already —
  a query result, the history log — passes no `filtering` and is therefore never narrowed, only
  sorted.
- **The grid never reorders its own rows.** It reports the keys through `onSortChange` and marks the
  headers from `sort`; who sorts is the call site, because only it knows where the rows come from.
  A **list** of keys, the first deciding and the rest breaking its ties — the panel is where a second
  is added, while a header's own menu means one key and replaces whatever was there. A sort always
  sends the reader back to page one, and always drops the selection.
- **Columns are dragged into place by their headers**, where the call site passed `onMoveColumn`.
  Pressing anywhere on a header that is not the resize edge or its `⋮` picks the column up — the
  header dims, **the column it belongs to is outlined down the whole grid**, and an accent line runs
  the same height to show the boundary it would land on, the boundary being read like a caret: past
  a column's half-way line is the place *after* it. Both marks are the grid's rather than the header
  row's, and both run past the header on purpose: what is being carried is a *column*, and a mark
  that stopped at the header would leave the reader matching it against the rows by eye. Dropping it back where it came
  from, either side of itself, is not a move. The grid reports the drop in the positions it **drew**
  — `from`, and the place it landed *before* — and **who keeps the order is the call site**, because
  only it knows whether the order outlives the grid: `TableDataView` stores one against the table
  and `ResultPane` holds one for as long as its statement stands. Neither reorders a row: the order
  is a list of column *names* (`columnOrder.ts`), every column keeps the place it holds in a row,
  and both surfaces map the grid's column back through it exactly as one maps a hidden column. A
  drag does not scroll the grid, so a column is carried among the headers on screen — twice for
  further.
- **Columns resize** by dragging the right edge of the header, and **double-clicking that edge fits
  the column** to the widest of its name — plus the lock, on a key column — its type line and its
  cells, capped at 520 px and measured on a canvas against the tokens each is drawn with — `--font-ui` / `--text-base` for the name, and
  `--font-data` / `--font-data-size` / `--font-data-weight` for the rest, so a fit measures the font
  the reader chose rather than the one shipped. Widths are keyed by column *name*, so hiding a column
  does not reset them. It measures the **first `FIT_SAMPLE` (2 000) rows and no more**: a grid may
  hold every row of a table, and measuring a million cells is seconds of frozen window for a width
  nobody can see anyway.
- **A column may say what width it opens at** (`ResultColumn.width`), for the one where the grid's
  own 160 px is plainly wrong: a column of **ticks** is not as wide as a column of values. It is
  only the width it *opens* at — a drag or a fit replaces it, and the reader's width always wins,
  which is the order the three are read in.
- **A grid may be drawn as a strip that follows another** (`strip`): no row gutter, no scrollbars,
  no header menu and no resize handle — the grids either side carry all of them, and a second set
  would only repeat what is already there. It exists for the column of **ticks between a
  comparison's two panes**, which is a grid of its own precisely so that scrolling either side
  sideways cannot slide it away. Its width is the frame's and not the reader's: a column dragged
  wider than the strip around it could never be scrolled back, there being no bar to scroll it with.
- **A column may refuse an edit** (`ResultColumn.readOnly`), which is the one arrangement a single
  `onEditCell` cannot express: a grid whose rows are *read* and which carries one column that is
  acted on. A comparison is that grid — a **Take** tick beside the columns of facts — and without
  it, passing `onEditCell` for the tick would open a text editor on every fact beside it.
- **The grid keeps its own selection when uncontrolled.** Pass `selection` +
  `onSelectionChange` only when something outside needs the focused cell.
- **The grid hands its scroller over through `viewportRef`**, for the one surface that has to move
  two grids as one. Two panes of a comparison hold the same rows in the same order, and a pane
  scrolled on its own would put row *n* of one beside row *m* of the other — which is the single
  thing that makes a diff unreadable. It is the **element** and not a scroll position, because who
  mirrors whom is the call site's; `ComparePanes` is the only one, and what stops the echo there is
  that a scroller already at the position it is being sent to does nothing, never a flag on a timer.
  Pass a stable ref: it is attached with the element and would otherwise detach on every render.
- **Export belongs over the grid, never in the footer**, because it acts on the whole grid rather
  than on a row: `ExportMenu` opens CSV, JSON and SQL, each writing what the grid holds to a file
  picked in the system's own save dialog. It is one component in two places — the head strip the
  grid draws for itself, or the surface's own toolbar, where `TableDataView` sets it beside the
  **Cell Editor** and **Column** toggles and passes `headStrip={false}`, so a table carries one
  row of controls rather than two stacked. It is on the right because that is where a reader looks
  for what to *do with* the table, the left of the footer being where a row is changed.
  `FilterSortButton` is the same arrangement on the other side of the strip. What each
  format does with a `NULL`, and which rows a surface counts as "what the grid holds", are in
  [features.md](features.md#exporting-a-grid).
- **`CellEditorButton` is that same arrangement for the cell-editor panel**: one button, in the head
  strip through `cellPanel` or in the surface's own toolbar, and it opens the one `CellEditor` both
  writable surfaces draw. The panel itself is not the grid's — it is the surface's, beside it — but
  the *toggle* is, so a grid that has one never has to be asked twice where the button goes.

## The Filter & Sort panel

`FilterSortPanel` is where a grid is narrowed and ordered, and it is one panel for both: a reader
narrowing a table is nearly always about to order it too, and two panels would be two Applys.

- **Nothing is applied until Apply.** A condition is three parts and a value picked out of a list,
  and a read per keystroke would be a read of rows nobody has asked for yet. The panel therefore
  holds a **draft** and `Apply Filter & Sort` is what hands it over; **Criteria Edited** beside the
  button is how the draft says it differs from what is in force, and the button is dead until it
  does. Only what changed is sent — an Apply on an untouched panel is not a re-read. The **cell
  menu** is the exception and applies at once: its condition was picked whole.
- **The draft is the grid's, not the panel's** (`useFilterDraft`). The panel is mounted only while it
  is showing, so a draft held inside it would be thrown away by the very button that hides it: a
  reader who removed two conditions and then closed the panel would find both back on opening it
  again, the removal having been undone rather than left waiting for Apply. Hiding the panel neither
  widens the grid nor withdraws the edit in hand. It does not outlive the *tab*, though — a table tab
  is taken down when it is looked away from, and what it keeps is what was asked for, never what was
  half-asked.
- **The draft follows what is in force.** A condition added from the cell menu, or a sort taken off a
  header's menu, appears in the panel rather than being overwritten by the draft beside it.
- **A header's Add Filter adds to the draft, not to what is in force.** It is the one addition that
  is *not* picked whole — a header knows the field and not the value — so it opens the panel and
  lands at the end of the root with its value unset, and Apply is still what narrows the rows. That
  is also what keeps it cheap: no read is made for a condition nobody has spelled yet, and a draft
  the reader was half-way through is added to rather than replaced. It reaches the draft **directly**
  (`draft.addCondition`), the draft being the grid's own, so asking twice for one field is simply two
  rows and there is no ask to carry into the panel and clear again.
- **A condition is three parts, and each opens what it may be** — the column, the operator, and the
  value. The column and the value open a **small panel hung under the part itself** (`PickerPanel`,
  a popover) rather than a dialog over the grid: what is being changed is three words on a row, and
  a modal that hides the table to change one of them costs the reader the rows they are narrowing.
  The operator opens a plain `MenuButton`, its twenty being a fixed list in a deliberate order that
  a search box would only get in the way of. Both panels wear the menus' own surface
  (`MENU_SURFACE`), so a part that opens a list and a part that opens a menu read as the same kind of
  control. The column and the value are accent-coloured and the operator is not, because the two ends
  are the reader's own and the middle is the comparison between them. A value the operator has not
  been given yet reads `<?>`.
- **Both panels are searched, not scrolled** (`SearchList`): a table has as many columns as it likes
  and a column as many values as its rows, so neither is a list a reader should have to walk down to
  reach a name they already know. The column panel puts the keyboard in its search box, because
  finding the name *is* the panel; the value panel puts it in the value box, because typing a value
  the column has never held is as ordinary as picking one it has.
- **Twenty operators** (`FILTER_OPS`), and what each takes is its own business: `is NULL`,
  `is not NULL`, `is empty` and `is not empty` take **none**; `is between` and `is not between` take
  **two**; `is in list` and `is not in list` take **any number above none**; the rest take one. An
  unset value is **absent** from `values` rather than empty, which is what leaves `= ''` sayable —
  a reader who means the empty string reaches for `is empty`. `is empty` and `is NULL` are different
  questions, and the panel does not pretend otherwise.
- **A condition short of a value narrows nothing, and neither does an unticked one.** The tick box
  beside a row is how a condition is put aside without being lost. `activeFilter` is what prunes
  both, plus any group left holding nothing, and it prunes **once per read** — what is left is
  exactly the tree that becomes a `WHERE`. Nothing else crosses the wire: the tick is the builder's
  business, not the server's.
- **`+` adds a condition beside the row, `()+` adds a group.** On a *group's* own row both add
  *inside* it, because a group is opened before it is filled; on the **Filter** header both append to
  the root. A group's conjunction is the word on every child but its first, and it is a button: one
  group joins one way, so flipping it anywhere flips it for the group.
- **Sort By is a list**, added to with its own `+`: a column and a direction per row, the first
  deciding and the rest breaking its ties.
- **The value panel is `ValuePick`**: the values the column actually holds, ticked rather than typed,
  with a box beside them for one it does not hold. The values come from the server —
  `LoadColumnValues`, a `SELECT DISTINCT … ORDER BY 1 LIMIT` capped by `app.ColumnValueLimit` — so
  they are the column's own and not the page's, and they are read **on every open**, the panel being
  mounted with the popover: a value added since the last look is there. **NULL is not among them**:
  `is NULL` is how a reader reaches it, and a list cannot say the difference. Nor is the **empty
  string**, which an empty box already means. A surface that cannot ask leaves `suggest` out and the
  panel is then the box alone.
- **Ticking a second value turns the condition into `is in list`.** The rows of the value panel are
  **check boxes** wherever the operator has a list form to grow into — the grid's own `SquareCheck` /
  `Square` pair, so one glyph means one thing everywhere — and the operator part above the panel
  changes to `is in list` the moment a second value is ticked, so the condition on screen is always
  the condition that will run. `PROMOTIONS` in `filter.ts` is the pairing: `=` and the four text
  matches become `is in list`, their negations `is not in list`. An operator naming a **bound** is
  left out and its rows are a bare tick that *replaces*: `> a` and `> b` are one comparison, not
  both. Unticking back to one value gives back the operator it grew out of, for as long as the panel
  stays open.
- **The panel's shape follows its values**, not the other way about: one value is a box, more than
  one is a row of chips with an *add* box under them, and a range is **From** and **To**.
- **There is no OK on the value panel.** What it changes is the draft, and the draft already has one
  commit — `Apply Filter & Sort`; a second one inside it would be a button that looks like it applied
  something and did not. A panel of check boxes never closes itself either, there being another tick
  to make; only the single-pick kind closes on the pick, having nothing left to ask.
- **Builder and Text.** `Text` is drawn and **disabled**: a typed condition is where the panel is
  going, and leaving the choice out would make it look as though there were only ever one way in.

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
- The right-hand side is the caller's, and both surfaces hand it the same `GridPager`: **first,
  back, the page in force, forward, last**, and one gear. Nothing on it counts rows — that belongs
  to the status line below — so the walk stays a walk. The page in force is an **input** rather
  than a label: a page is typed into it and committed with `Enter` or by leaving the box, and
  `Escape` puts the box back to the page the grid is showing. Only where the rows come from differs
  — the table view walks the server, and a query result is paged inside `usePageWindow` out of the
  rows it already holds — and `paging.ts` is the arithmetic all four of them share, so the arrows
  and the words under them cannot disagree on where a page begins. `partial` is what keeps `Next`
  alive on the last page: the total is a floor rather than a count, so the caller can go and fetch
  the rest, and a *typed* page past that floor is allowed for the same reason.
- **The gear is the whole page-size control**, a popover holding `Limit records` and a count.
  Unchecking it asks for **every** row, which is a different kind of read rather than a bigger one —
  streamed and watched on a table tab, a replay of the statement on a query result — and that is
  why it is a checkbox and not one more entry in a list of sizes. The typed count survives the
  bound being lifted, so turning it back on restores what it was.
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

## The line under that

Under the footer sits one `GridStatus`, which is where a read is *read about* rather than acted on:

```
SELECT * FROM `shop`.`users` ORDER BY `id` LIMIT 100 OFFSET 0  ⧉ │ Record 19 of 1,000 in page 1 of 10 │ Last Refresh: 3m
```

- **The statement is the one that ran**, never one this side spelled again. A table read is built by
  the driver with its bounds written into it rather than bound (`buildSelect`), so one string is
  both what ran and what is shown; it travels back as `TableRows.statement`. A query result shows
  the statement the reader typed. Either way the **copy** beside it writes exactly that to the
  clipboard, and confirms on the icon itself — a tick for a moment — there being no room here for a
  message.
- **The tail of the statement is pinned, and the ellipsis eats the column list instead**
  (`PINNED_WORDS`). A statement is nearly always wider than the bar, and everything that *moves* —
  `LIMIT 100 OFFSET 300`, or its absence — is the last four words of it, so truncating from the
  right hid the only part worth watching: turning a page or clearing `Limit records` looked like
  nothing had happened. The column list is what gets clipped, being the same list on every page.
- **A read whose statement this side never learned says `No statement`.** Only the answer carries
  the statement, so a **stopped** read has none: what was on the bar belongs to the *previous* page,
  and leaving it there would put `LIMIT 100 OFFSET 0` beside thirty thousand rows. The copy is
  disabled for the same reason — there is nothing true to copy.
- **`Record` is where the reader is**, which is the focused row: the page's own offset plus that
  row's place in the page. With nothing focused it is the page's first row. A total shown as
  `1,000+` is a floor, from the same `partial` the pager reads.
- **`Last Refresh` runs on the status line's own clock**, so counting the seconds costs the grid
  nothing: a grid holding a million rows is not re-rendered once a second to move one word.
- It carries `--surface-panel` rather than the footer's `--surface-bar`, because it is a status line
  and not a strip of controls — the shade alone says which is which — and it holds exactly one
  control, the copy.

## When you add to it

1. New colour goes in `shared/theme/tokens.css` as a `--grid-*` token with a light-theme override
   and an `@theme inline` mapping — never a hex or a raw `var()` in the component.
2. Geometry constants stay at the top of `ResultGrid.tsx`; row height alone is shared, via
   `density.ts`, because the object tree virtualizes on the same number.
3. New behaviour is added once, in the grid, and this file is updated in the same change. Every call
   site — `ResultPane`, `TableDataView`, `TableDesigner`, `ObjectListView`, `QueryHistory`,
   `ScriptMessages`, the two panes of `ComparePanes` and the three plan surfaces (`Summary`, and
   `Grid` and `Statistics` inside `ExplainPane`) — then has it, which is the whole point.

## Where the columns come from

On a table store the columns are a fact: the catalogue says what a table has, and every row has all
of them. **On a document store there is no such fact** — a collection declares nothing, and two
documents in one collection may share no field at all. The grid is unchanged by that, and does not
need to be; what changes is only where the header comes from, and that is worth knowing because it is
the one place the app shows a *sample* rather than an answer.

- **The field list is inferred** from the first `sampleDocs` documents of the collection, `_id` always
  among them because every document has one. That list is what the tree draws under **Fields**, what a
  filter's field names are checked against, and what a value the reader typed is read into.
- **A bounded read shows everything it read.** The page's own documents are merged into that list, so
  a field the sample missed still gets a column the moment a page returns it.
- **A streamed read cannot**, and this is the one visible limit. `All` pushes its rows as batches and
  the header goes over before the first of them, so its columns are the sampled ones — a field outside
  the sample is not drawn for that read. A bounded page of the same rows shows it.
- **A nested value is one column**, drawn in the shell's notation. A column per path would give two
  documents of one collection different columns, which is the one thing a grid cannot draw.

**On a key-value store there is no fact and no sample either** — the columns are a *constant*. A
Redis key space is drawn as `Key`, `Type`, `Value`, `TTL` and `Size` on every server there has ever
been, `Key` being the primary key because it is the only thing that identifies a row and it
identifies exactly one. Because they are a constant, they are also the one set of column names the UI
may *name*: `features/table/keySpace.ts` does, and everything that reads one is reached only where
`EngineInfo.keys` says the namespace is a key space at all. What is never done is asking an engine's
name anything. What is sampled there is not the header but the **value**: a container too
large for a row is drawn as its first fifty members and ends with `… 50 of 4321`, and that mark is
what makes writing the cell back fail — an edit of a sample would drop whatever it did not show. See
[architecture.md](architecture.md#the-key-value-store).

None of this is a special case in `ResultGrid`. It receives `GridData` exactly as every other surface
does; the difference is entirely `internal/driver/mongo/documents.go` and
`internal/driver/redis/keys.go`, and it is written down here because a reader who does not know the
list is a sample would read a missing column as a missing field.

## Where the two call sites legitimately differ

Not in what an edit means — `useGridEdits` and `GridFooter` make that identical — only in what
surrounds the grid:

- `ResultPane` pages in memory, so it maps the grid's *page-local* row index back to the whole
  result's on the way in — for edits, `isDirty`, `isDeleted` and a delete's rows alike — and it maps
  the column too, its reader being free to arrange the headers. **It sorts in
  memory**, there being no statement of its own to re-order, and it keeps the outcome as a list of
  row numbers (`useGridOrder`, over `sortedOrder`) rather than as reordered rows: a pending change is
  keyed by the number a row had when it was read. Only the rows **fetched** are ordered — a sort
  reaches no further, and a draft row therefore stays pinned after them, so **Add** still lands on
  the last page. A sort is dropped when the statement changes, and so is the column order: there is
  no stored table under a result to remember one against. **It does not filter**: it passes no
  `filtering`, so it carries neither the button nor the panel, and its cell menu offers no condition
  — a header's own menu, which sorts, is the whole of what acts on a result in hand. It draws the
  **Cell Editor** panel under its grid, `CellEditor` and its two bounds being the same on both
  writable surfaces; the panel is closed with the grid, since the button that opens it lives on that
  grid's head strip and a run that answered with an error would otherwise strand it open.
  `QueryHistory` is the same arrangement without the drafts, the paging, the panel or the arranging,
  which is the whole of what
  `useGridOrder` is for. Its footer
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
  It draws **two** grids rather than three on an engine with no referential constraints of any kind:
  a pane whose Save could only fail is a pane worth leaving out — see `EngineInfo.hasForeignKeys`.
- `ObjectListView` neither pages, hides nor edits: its rows are one namespace's tables and it maps
  nothing. It is the one site whose rows *stand for* something openable, so it is the one that
  passes `onActivateRow`, and the only read-only site with a toolbar of its own — over the grid,
  where a surface's controls go, acting on the table the selection names. Its wording is the
  engine's (`engineWords`), so a document store's list says Collection throughout; only the wording
  changes, every button running the same bound method.
- `TableDataView` pages on the server, and can hide columns and arrange them, so it maps the grid's
  *visible* column index back to the real one on the way in — `arrange` first, then the hidden names
  out of what is left, and one list answers for both. **It is the site that stores an order**, under
  the connection, namespace and table (`columnOrderKey`), because a table is a thing that outlives
  the app. **It sorts and filters on the server too** — both travel
  in `TablePageRequest` and become the read's `ORDER BY` and `WHERE`, because ordering or narrowing
  the hundred rows in hand would only shuffle what the reader can already see, and the total is
  counted through the same conditions or the pager would walk pages that are no longer there. Its
  Cell Editor panel stages a value on the focused cell
  exactly as the grid's own inline editor does; the footer still commits, and the cell menu's **Edit
  in Cell Editor** is what opens that panel from the grid. It is also the only site
  that adds to the header menu, since `ColumnPicker` and that menu toggle one set of hidden names,
  and the only one that turns the head strip off (`headStrip={false}`), its own toolbar already
  carrying both **Filter & Sort** and **Export**. Its value picker asks about the table it is
  showing, which is the one table it could possibly mean.
  Over a **key space** it is also the one site that maps *rows*: the keys group into a tree, so the
  grid's row *n* is not the page's row *n*, and one `realRow` answers for an edit, `isDirty`,
  `isDeleted`, a delete and the record on the status line alike — the same arrangement its column
  mapping already is. A **branch** of that tree stands for a prefix and not for a key, so it maps to
  `-1` and every write to it is dropped: there is no key there to write to. It also passes **no
  `onEditCell`** there, so those rows are read and not typed: a click shows the key in the panel below
  and nothing else. Inline editing is right on a table store — a cell is one of forty and the reader
  is pointing at it already — and wrong on a key space, where a click is how a reader *walks* the
  keys and the value is a container the panel lays out; an editor opening under every click was the
  grid's behaviour applied where it does not belong. The footer still writes, so the four editing
  icons stay: what changed is where an edit is spelled, not whether there is one. It passes `outline`
  on the key column (the second site to, after a comparison), `cellMenu`, and no `filtering` at all.
  What an
  **export** writes stays the *flat* page: a tree draws a key by its tail, the prefix being on the row
  above, and a file of tails would be a file of keys no server has. A `Ctrl`+`C` still copies what is
  drawn, which is the grid's own rule — **Flatten** is one press away for a copy of whole keys.

- The **comparison** is `ComparePanes` twice over — two grids that map nothing, built from **one**
  list so row *n* of one is row *n* of the other by construction. Its *shape* half draws that list
  as a **tree**: the table is the root, the three sections fold under it, and each pane carries the
  name and the declaration **its own side** holds. It is the one site that passes `outline`, and
  `structureTree.ts` is what flattens the tree into the list both panes read; a caret belongs to the
  row and not to a pane, so folding either folds the pair. Its *data* half draws the same list as
  rows. Both are `dense` and both pass `markRow` and `markCell`.
- **The ticks are a third grid**, `strip`, standing between the two panes and belonging to neither:
  a tick is a fact about the comparison and not about either table, and a tick column inside a pane
  slides out of sight the moment that pane is scrolled sideways. It is the only writable grid of the
  three — the panes are `readOnly` throughout — so what is acted on is exactly the tick and nothing
  beside it can be typed over. Stacked panes have no between, so there the tick goes back to being
  each pane's first column. `ComparePanes` is also the one site that takes a `viewportRef` — its
  panes are useless apart, and **sideways is carried only between scrollers that have somewhere to
  go sideways**, or the strip's own nought would drag both panes back to the first column.

- The three **plan** surfaces — `Summary`, and `Grid` and `Statistics` inside `ExplainPane` — are the
  plainest sites there are: a `GridData` built by `features/results/plan.ts` from `QueryResult.plan`,
  and nothing else. No paging, no hiding, no editing, so nothing to map. `Grid` does not even build
  one: the plan carries the engine's own EXPLAIN as columns and rows, and they are handed over as they
  arrived. The one thing worth knowing is that **a cell is left empty where the engine gave no
  number** — a step it does not price, a share of nothing — since `0.00` reads as free where blank
  reads as unpriced.

Both mappings exist because the grid always reports the coordinates it drew, never the ones the
server knows. Neither one lets the *other* axis drift: a paged surface leaves columns alone and a
column-hiding surface leaves rows alone.
