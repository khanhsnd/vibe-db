# Frontend conventions

- **One click selects, two clicks open — outside the grid.** A single click never dials a server and
  never opens a table, so a stray click in the tree is free: it moves the selection, and opening is
  a double-click (or a context-menu item, or `Enter`). `ObjectTree` is what implements it. The
  **disclosure caret** is the one control that does both: it selects its node *and* expands it,
  because that is what a caret is for. The **`Tables` folder** is the one row whose selection also
  opens a pane — its table list, which is what that folder means to a reader; it costs one
  catalogue read and no more than reopening the tab it reveals.
- **In a writable grid, one click opens the cell.** Nothing there loads or dials anything — an
  editor is staged, not sent — so making the reader click twice to change a value they are already
  pointing at buys nothing. `Escape` closes the editor and hands the keyboard back to the grid, which
  is how a reader gets to a plain selection; see [grid.md](grid.md).
- **Selecting must not expand, and expanding must not select** anywhere but that caret. Two actions
  hidden in one setter cancel each other out: `selectNamespace` used to expand the node it named, so
  the caret's select-then-toggle collapsed what it had just opened. Expansion lives in `toggleNode`
  and nowhere else, and every expand goes through `toggleTreeNode` so the level's children are
  loaded with it.
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
- **The status bar reports the tab in front**, not whichever session happens to be first: it names
  the connection, its server, the namespace that tab is pointed at and the server's version, and
  against the right how many connections are open at all. Reporting an arbitrary session would name
  one server while the grid under it drew another's rows.
- **It is also where a failure nothing was waiting on is said.** A rejection a *dialog* asked for is
  answered in that dialog and never here; the bar is for the ones asked for from a menu, which has
  nowhere to put one — a refresh that could not read, a dump that could not be written. While nothing
  is connected the wording stands where the server would be, and while something is it takes a fact of
  its own in the danger colour. There is one slot, so it holds the last outcome: an action that
  succeeds takes the previous failure down with it.
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
- **State that must outlive a tab being looked away from belongs in a store keyed by the tab id**,
  never in the component: `WorkArea` mounts the tab in front and no other, so a component's own state
  goes down with it. `stores/queries.ts` keeps a query tab's rows that way and `stores/tableViews.ts`
  keeps a table tab's — the panels it has open, the columns it hides, and what its read was asked for.
  Both are dropped by `tabs.close`, which is the one place a tab's belongings are let go, and a table
  tab's view is started by `tabs.open` so a tab cannot exist without one. What each kind of tab keeps,
  and why the two differ, is in [features.md](features.md#tabs).
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
  own mark in white on `--engine-<name>`, because MySQL and PostgreSQL are both databases and a
  `Database` glyph twice over says nothing. The dolphin and the elephant are what a reader already
  knows the two by, and the shape still reads where the colour cannot. The marks are two vendored
  path strings in `engineGlyphs.ts` (Devicon, MIT) rather than an icon-set dependency; the
  elephant's eyes and tusks are subpaths filled `evenodd`, so they are holes onto the chip.
- **Nothing is selectable, and every failure is.** `body` carries `user-select: none` because this
  is a client and not a document, so text is opted back in with `.selectable` — and anything
  reporting a failure always opts in: `ErrorStrip` (the one-line strip four surfaces used to
  hand-roll), `ResultPane`'s error panel, `Empty` (whose hint is often a server's own wording),
  `Banner`, `Field`'s error note, the console transcript and the status bar's facts. The panel adds
  `shared/ui/CopyButton` — the same tick-on-copy control the grid's status line uses — because the
  code, the message and the engine's detail are one answer and are wanted as one paste.
- **A read the reader waits on is `shared/ui/Loading`**, which fills its pane the way `Empty` does:
  the two make the same claim about the same area — nothing to show, or nothing *yet*. A surface
  draws it only while it has no rows at all; once rows are on screen a re-read leaves them there.
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
