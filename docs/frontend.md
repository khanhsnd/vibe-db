# Frontend conventions

- **One click selects, two clicks open — outside the grid.** A single click never dials a server and
  never opens a table, so a stray click in the tree is free: it moves the selection, and opening is
  a double-click (or a context-menu item, or `Enter`). `ObjectTree` is what implements it. The
  **disclosure caret** is the one control that does both: it selects its node *and* expands it,
  because that is what a caret is for. The **`Tables` folder** is the one row whose selection also
  opens a pane — its table list, which is what that folder means to a reader; it costs one
  catalogue read and no more than reopening the tab it reveals.
- **`Ctrl`/`Cmd` picks a second row beside the first**, in the tree as in the grid — that modifier
  means "and this one as well" everywhere. The tree keeps a *list* of picked nodes with the last as
  the focused one, so it has the grid's two depths: a picked row is washed, the focused one is bold.
  It differs from the grid in one way and deliberately — `Ctrl` **toggles** here, because a
  rectangle cannot be split and a list of rows can. A right-click that lands *inside* a pick of
  several leaves it standing, which is the rule the cell menu already sets and the whole reason a
  pair survives being right-clicked. Today the one thing that reads the pair is
  [Compare](features/compare.md).
- **A grid may be writable in one column and read-only in every other** (`ResultColumn.readOnly`).
  It exists for the one arrangement a single `onEditCell` cannot express: rows that are *read*, with
  one column that is acted on. A comparison is that grid — a **Take** tick beside the columns of
  facts — and without it, passing a handler for the tick would open a text editor on every fact.
- **In a writable grid, one click opens the cell.** Nothing there loads or dials anything — an
  editor is staged, not sent — so making the reader click twice to change a value they are already
  pointing at buys nothing. `Escape` closes the editor and hands the keyboard back to the grid, which
  is how a reader gets to a plain selection; see [grid.md](grid.md).
- **Selecting must not expand, and expanding must not select** anywhere but that caret. Two actions
  hidden in one setter cancel each other out: `selectNamespace` used to expand the node it named, so
  the caret's select-then-toggle collapsed what it had just opened. Expansion lives in `toggleNode`
  and nowhere else, and every expand goes through `toggleTreeNode` so the level's children are
  loaded with it.
- **One tree, two builders, two menus.** The sidebar holds two kinds of connection now — a database
  and an [object store](features/object-storage.md) — and they share the *row* and nothing else:
  `buildTree` and `buildStoreTree` each answer with `TreeNode`s and `App` concatenates them, so
  `ObjectTree`, the filter, the colour wash and the selection are written once and never learn there
  were two. The same split runs through the menu (`useTreeMenu` beside `storeMenuItems`) and it is the
  right shape for the same reason: not one of that file's twenty node kinds is a kind here, and every
  item in it names a namespace or a table. A store's rows stop at a **bucket** — a prefix inside one is
  drawn in the grid, because the folders come off the page the reader is already looking at.
- **What a store connection may do arrives on the session, not in an `EngineInfo`.** `ObjectRules` is
  one entry per action — what it is called, how dangerous it is, whether it is confirmed, whether it is
  refused — computed once by the Go side from `domain.ObjectHazard` and `domain.Access`. A component
  reads the entry; nothing re-derives it, and an action a read-only store refuses is **left out** of
  the toolbar rather than greyed, which is the rule `withoutWrites` already sets for the tree.
- **An engine's differences are `EngineInfo` fields, never its name.** A component that asked
  `engine === 'mongo'` would be a component that had to be found again for the next engine, so what
  the UI needs to know is declared once in `bridge/connections.ts` — `needsDatabase`, `hasSchemas`,
  `databaseHasOwner`, `documents`, `databaseNeedsCollection`, `hasForeignKeys`, `hasRoutines`,
  `hasTruncate` — and read through `engineInfo`. A capability the UI has to branch on is a new field
  there, and its comment is where *why* lives.
- **What a thing is *called* is `engineWords`.** A collection is not a table and a document is not a
  row, and a menu that said otherwise would tell the reader something untrue about the server in front
  of them — the truncate warning especially, which on a table store warns about a foreign key that a
  document store cannot have. It is words alone: a `NodeKind` is still `tables` and the bound method is
  still `DropTable`, so one name means one thing on the wire whatever it says on the screen.
- Path aliases: `@/*` → `frontend/src/*`, `@wailsjs/*` → `frontend/wailsjs/*`. Declared in both
  `vite.config.ts` and `tsconfig.app.json`; changing one requires changing the other.
- **Three surfaces, in order of depth.** `--surface-canvas` is content, `--surface-panel` is the
  app's own chrome — command bar, tab strip, sidebar — and `--surface-bar` is a strip of controls:
  a toolbar over a grid, the action bar under one, a dialog's header and footer. The grid's column
  header takes it too, because rows are content and the names over them are not: read against
  canvas rows, that shade is what says where the data starts. A bar never meets the *canvas* on
  both sides of another bar, which is the whole reason the third shade exists — the one place two
  do meet is a surface's own toolbar over the grid's head strip, and they are meant to read as the
  single strip they look like.
- **A kind is a colour, and a folder wears its contents'.** `--kind-*` is one colour per kind of
  thing the catalogue holds — a database, a schema, a table, a view, a routine, a field, an index, a
  foreign key — and `--type-*` one per class of column type, the seven `columnType.tsx` sorts a type
  into. A sidebar six levels deep in one grey says only that it is six levels deep, so the tree, the
  tab strip, the query toolbar and the grid's header all draw from these and mean the same thing by
  them. A **folder takes the colour of what is inside it** rather than one of its own — `Indexes` and
  the indexes under it read as one thing — and it is drawn as a folder, open or shut, never as the
  thing it holds: a `Tables` folder wearing a table would be the same row as the tables beneath it.
  A new kind is a token and a row in the `COLOURS` map beside `ICONS`, never a colour in a component.
  The one **exception** is a folder in a bucket, which is not the catalogue's and is drawn a hue away
  from its contents rather than a shade of them — the reason is
  [the bucket grid's](features/object-storage.md#the-grid).
- **A connection's colour is inherited, not repeated.** `buildTree` puts it on the connection node
  alone and `flatten` carries it down every branch, so a `FlatNode` knows which connection's colour
  its row is under without every node builder having to be told. The wash itself is mixed in
  `features/connections/ConnectionColour.tsx` from one token — `--conn-tint` — over the `--conn-*`
  palette, and it is drawn as a `-z-10` overlay inside the row rather than as the row's background:
  a background set inline would beat the `hover:` and `bg-selected` classes and the row would stop
  responding to the pointer. The same file draws the swatch the menu is picked from, as a *function*
  returning an element, because `useTreeMenu` builds its items in a `.ts` file.
- **A tree badge is a count or a fact, and it is drawn as whichever it is.** A count — how many
  tables, how many fields — is a pill; a fact about the row itself — a field's type, an index's
  columns, the server a connection points at — is faint mono text. `isCount` tells them apart by the
  badge being all digits, which every count the tree draws is and nothing else it draws can be.
- **Which connection the reader is on is worked out once**, in `App`, and drawn by two surfaces: the
  banner across the middle of the command bar and the bar along the bottom. One shape — `StatusInfo`
  in `app/layout/status.ts` — so the two can never disagree about the connection they describe, the
  banner carrying *which* it is and the bar what the last read of it cost. What each of them says is
  [features/shell.md](features/shell.md#which-connection-you-are-on).
- **Both report the tab in front**, not whichever session happens to be first: reporting an arbitrary
  session would name one server while the grid under it drew another's rows. Where no tab is open it
  is the row selected in the tree.
- **Colour and type live only in `shared/theme/tokens.css`.** Raw values sit on `:root`, are
  overridden under `:root[data-theme='light']`, and are mapped into Tailwind utilities via
  `@theme inline` — the `var()` indirection is what makes runtime theme switching work without a
  rebuild. Do not hardcode hex values in components.
- **Data is drawn in `.data-font`, chrome is not.** The class is in `index.css` and carries the three
  `--font-data*` tokens together — family, size, weight — so the font the reader picked in Options
  cannot land on a cell and miss the editor beside it. A grid cell, a cell editor and an SQL line take
  it; a row number, a type line, a toolbar box and the status bar stay `font-mono` at their own size,
  because they are chrome about the data rather than the data.
- **A preference lives in `stores/settings.ts`, transient UI state in `stores/ui.ts`.** The settings
  store is the only place a preference is saved (one `localStorage` key) and put in force, through
  `applyTheme` / `applyDataFont` in `shared/theme/appearance.ts` — a module with no store import of
  its own, so nothing has to reason about which of the two loads first. A component reads
  `s.settings.<field>`; `useTheme` is a thin hook over the same store. What a *new* tab starts with is
  read with `getState()` in a `useState` initialiser rather than subscribed to, because a default that
  followed an open tab around would not be a default.
- **A preference about *one thing* is kept per thing, by the module that owns it.** What the Options
  dialog edits is everybody's and lives in the settings store; what a reader has done to one table or
  one connection is that thing's, and is stored under its own key beside the code that reads it —
  `features/results/columnOrder.ts` for a table's column order,
  `features/connections/databaseFilter.ts` for [which databases a connection
  draws](features/schema-tree.md#filtering-a-connections-databases), and
  `features/diagram/persist.ts` for [a namespace's
  diagram](features/diagram.md#where-it-is-kept-and-taking-it-away). All three are keyed by what they
  are about, all three survive the app closing, and none goes to the backend: a preference the
  *server* would have to be told about is a field on the profile, which is what a connection's colour
  is. The diagram is the largest of them and so is the one written on a **timer** rather than on the
  change, for the reason the query strip is.
- **What the reader was *doing* is kept the same way.** `stores/savedQueries.ts` writes the strip's
  [query tabs down as they are edited](features/query-tab.md#the-tabs-are-still-there-next-time) and
  `stores/tabs.ts` builds itself from them, so a buffer is where it was left. It differs from the
  three above in what it costs to write — a keystroke changes a buffer and a script can be a megabyte
  of them — so it writes on a timer rather than on the change, and flushes on `beforeunload`.
- **State that must outlive a tab being looked away from belongs in a store keyed by the tab id**,
  never in the component: `WorkArea` mounts the tab in front and no other, so a component's own state
  goes down with it. `stores/queries.ts` keeps a query tab's rows that way, `stores/tableViews.ts`
  keeps what a table tab's read was asked for — the panels it has open, the columns it hides, the
  page, the order, the conditions — `stores/tableRows.ts` keeps what came back, and
  `stores/gridEdits.ts` keeps what is waiting to be written back to *either* grid. All four are
  dropped by `tabs.close`, which is the one place a tab's belongings are let go, and a table tab's
  view is started by `tabs.open` so a tab cannot exist without one. What each kind of tab keeps is in
  [features/shell.md](features/shell.md#tabs).
- **A new kind of tab adds state, not a second way of running things.** `stores/console.ts` keeps a
  console's transcript and nothing else: the run goes through `stores/queries.ts` under the same tab
  id, which is why a console's rows are paged, exported, edited and stopped by the code that already
  does it for a query tab. Where a tab *points* is `tabs.retarget`, one action for every kind, so the
  namespace picker on a strip is one component (`features/connections/NamespacePicker.tsx`) rather
  than one per tab that has one.
- Row height is shared between the object tree and the result grid via `shared/theme/density.ts`
  (the virtualizers need it as a number).
- `shared/ui/Split.tsx` wraps `react-resizable-panels`, which renamed its entire public surface
  between v3 and v4. Import splitters from that wrapper, never from the library directly. Note its
  `Panel` inner div carries `overflow: auto`, which surfaces small child overflows as scrollbars.
  Two things about it are easy to get wrong and both are load-bearing:
  - **A handle is styled off its `data-separator` attribute, never `:hover` or `:active`.** The
    library grabs the pointer within ten pixels of the one-pixel line and reports `hover`, `active`
    or `focus` on the element; a CSS hover rule fires on the pixel alone, so the handle stays
    invisible right up to the drag the reader is already performing. `Group` also sets its own
    `display: flex`, `height: 100%` and `overflow: hidden` — a `flex` or `h-full` class on a `Split`
    is repeating the library.
  - **A `Split` whose panels come and go passes the mounted ids as `panelIds`.** The layout is then
    remembered per *combination*, so opening a side panel does not resize what is beside it with a
    layout saved when it was absent — `features/table/TableDataView.tsx` is the example.
- TypeScript runs with `strict`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`.
  The last one means an optional prop must be typed `foo?: T | undefined` to accept an explicit
  `undefined`.
- `frontend/src/bridge` is the only directory permitted to import `frontend/wailsjs/**`, and it is
  where constants shared with Go live — `bridge/rows.ts` mirrors the const block of
  `internal/app/dto.go`. A component should never hardcode a value the backend also knows.
- **One action, one glyph.** `shared/ui/menuIcons.tsx` holds the icon every context-menu item is
  drawn with, so a delete looks the same in the tree as in a grid header — and a menu built in a
  `.ts` file can name one without becoming a component. A menu whose items carry no icon keeps its
  old width; one icon in the list gives every item the gutter, so the labels stay aligned.
- **One menu, two ways in.** `ContextMenu` is raised by a right-click and `MenuButton` by a control,
  and they share the `MenuItem` type and their look (`MENU_CONTENT`, `menuItemClass`, `MenuGlyph`,
  all exported from `ContextMenu.tsx`). An action offered both ways is one object drawn one way.
- **An engine is a chip, not a line icon**: `features/connections/EngineIcon.tsx` draws the engine's
  own mark in white on `--engine-<name>`, because every engine is a database and a `Database` glyph
  seven times over says nothing. The dolphin and the elephant are what a reader already knows those
  two by, and the shape still reads where the colour cannot. Three of the marks are vendored path
  strings in `engineGlyphs.ts` (Devicon, MIT) rather than an icon-set dependency, and the other four
  are drawn there: at sixteen pixels a logo is a **shape**, so what each of those draws is the thing
  the engine *is* — a stack of discs, a turned page, a window pane, a ring. A subpath filled
  `evenodd` is how a hole onto the chip is made, which is the elephant's eyes and the page's corner.
- **Nothing is selectable, and every failure is.** `body` carries `user-select: none` because this
  is a client and not a document, so text is opted back in with `.selectable` — and anything
  reporting a failure always opts in: `ErrorStrip` (the one-line strip four surfaces used to
  hand-roll), `ResultPane`'s error panel, `Empty` (whose hint is often a server's own wording),
  `Banner`, `Field`'s error note, the console transcript and the status bar's facts. The panel adds
  `shared/ui/CopyButton` — the same tick-on-copy control the grid's status line uses — because the
  code, the message and the engine's detail are one answer and are wanted as one paste. What a drag
  paints on the text that *is* text is the **editor's own selection colour**, and it leaves the words
  their own colour: a solid accent block with the text knocked out of it reads as a web page being
  copied rather than as a selection, and a NULL drawn in red stops being red inside one. Anything
  with rows and columns is not this at all — it is [the grid](grid.md), and the grid selects cells.
- **A read the reader waits on is `shared/ui/Loading`**, which fills its pane the way `Empty` does:
  the two make the same claim about the same area — nothing to show, or nothing *yet*. A surface
  draws it only while it has no rows at all; once rows are on screen a re-read leaves them there.
- **A read the reader waits on *over rows already there* is `shared/ui/BusyBar`** — the strip under
  a tab's toolbar, holding a cat running on the spot, the word for what is happening, and a
  four-pixel line sweeping under both while the answer is out. It is the other half of `Loading`'s
  sentence: that one fills a pane with nothing in it, and this says a pane that is *full* is waiting
  for more. It exists because **a spinning icon is where a reader is not looking** — the one on a
  footer's Refresh is thirteen pixels in a bar of thirteen-pixel icons, so a statement taking forty
  seconds read, from a foot away, exactly like one that had already come back.
- **It is a strip and not a chip in the toolbar**, which is the second thing tried and the reason
  for the first: a toolbar is a row of controls of one height, and a cat with a word beside it
  wedged between two of them is neither — it pushed its neighbours about and read as the alignment
  having gone wrong. Under the toolbar there is nothing to be out of line with, and the full width
  belongs to the one thing whose job is to be noticed. It is drawn where the *tab's* toolbar is
  rather than over the grid, because a run belongs to the tab: the three that draw it are [the query
  tab](features/query-tab.md), the console and [the table tab](features/table-tab.md), each passing
  the words it wants — `Running…`, or `Loading <table>…`. Nothing re-renders while it runs
  (`.busy-bar` and `.cat-run` in `index.css`), and its two colours are the accent and the danger
  tone the app already uses for *go* and *stop*, so a line in motion cannot be read as a red strip
  saying something is wrong. A reader who asked for less motion (`prefers-reduced-motion`) gets the
  segment standing still across the whole track and the cat standing on the spot, the query being no
  less out either way.
- **The cat is `shared/ui/RunningCat`**, and it is only ever the strip's: a sixteen-pixel silhouette
  running on the spot where a `Loader2` used to spin. A spinner is the same shape on every surface
  of every app and so reads as furniture; a cat mid-stride is a thing a reader *notices*, which is
  the whole job. It is a **silhouette** because that is what survives sixteen pixels — a body, two
  ears, a tail, four legs and one `evenodd` hole for an eye, the trick `engineGlyphs.ts` already
  uses for the elephant's — and it takes `currentColor` rather than a colour of its own: **a black
  cat on a dark bar is an invisible cat**, so it wears the theme's own foreground and is black only
  where black is what reads. The run is **two frames swapped with `steps(1)`** and no tween, a leg
  interpolated across two pixels being a smear rather than a stride; only the tail is swept, being
  long enough for the movement to show. It runs **in place**, the thing that travels being the line
  beneath it. **One tab draws one cat**: the console's transcript says `Running…` in words alone,
  because the strip above it is already running one.
- **"Which of these am I on" is `shared/ui/Segmented`, and "is this on" is a pressed `Button`.** The
  two are different sentences and are drawn differently on purpose: a `Button` held down with
  `aria-pressed` wears the hover shade, which reads beside its neighbours as *one button shaded* — fine
  for a toggle like Word Wrap, and wrong for a row of mutually exclusive choices, where a reader is
  looking for where they are rather than for what is switched on. `Segmented` joins its options into
  one bordered group and gives the chosen one the accent outright. A comparison's two halves and
  [the monitor's three panes](features/monitor.md) are both it.
- Control styling is shared through `shared/ui/Field.tsx`: `CONTROL` for a form control in a dialog,
  the exported `TOOLBAR_CONTROL` for the shorter ones in a toolbar strip. Height is kept out of the
  shared `CONTROL_BASE`, because `cn` only joins class names — two height utilities on one element
  would be settled by the stylesheet's order rather than by that file's. `Field` takes an `inline`
  prop for a settings form, which puts the label beside the control rather than over it.

## Zustand selectors

Selectors must return referentially stable values. A selector that builds a new object or array
per call makes React's snapshot comparison always fail and produces an infinite render loop with a
blank screen. Derive such shapes in `useMemo` from primitive slices instead — `buildTree` in
`stores/connections.ts` is the reference example.
