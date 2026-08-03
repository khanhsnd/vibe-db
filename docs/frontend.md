# Frontend conventions

- **One click selects, two clicks open — outside the grid.** A single click never loads anything,
  never opens a tab and never dials a server, so a stray click in the tree is free: it moves the
  selection, and opening is a double-click (or a context-menu item, or `Enter`). `ObjectTree` is
  what implements it. The **disclosure caret** is the one control that does both: it selects its
  node *and* expands it, because that is what a caret is for.
- **In a writable grid, one click opens the cell.** Nothing there loads or dials anything — an
  editor is staged, not sent — so making the reader click twice to change a value they are already
  pointing at buys nothing. `Escape` closes the editor and hands the keyboard back to the grid, which
  is how a reader gets to a plain selection; see [grid.md](grid.md).
- **Selecting must not expand, and expanding must not select** anywhere but that caret. Two actions
  hidden in one setter cancel each other out: `selectNamespace` used to expand the node it named, so
  the caret's select-then-toggle collapsed what it had just opened. Expansion lives in `toggleNode`
  and nowhere else, and every expand goes through `toggleTreeNode` so the level's children are
  loaded with it.
- Path aliases: `@/*` → `frontend/src/*`, `@wailsjs/*` → `frontend/wailsjs/*`. Declared in both
  `vite.config.ts` and `tsconfig.app.json`; changing one requires changing the other.
- **Three surfaces, in order of depth.** `--surface-canvas` is content, `--surface-panel` is the
  app's own chrome — command bar, tab strip, sidebar — and `--surface-bar` is a strip of controls:
  a toolbar over a grid, the action bar under one, a dialog's header and footer. Two bars never
  meet edge to edge, which is the whole reason the third shade exists.
- **Colour and type live only in `shared/theme/tokens.css`.** Raw values sit on `:root`, are
  overridden under `:root[data-theme='light']`, and are mapped into Tailwind utilities via
  `@theme inline` — the `var()` indirection is what makes runtime theme switching work without a
  rebuild. Do not hardcode hex values in components.
- Row height is shared between the object tree and the result grid via `shared/theme/density.ts`
  (the virtualizers need it as a number).
- `shared/ui/Split.tsx` wraps `react-resizable-panels`, which renamed its entire public surface
  between v3 and v4. Import splitters from that wrapper, never from the library directly. Note its
  `Panel` inner div carries `overflow: auto`, which surfaces small child overflows as scrollbars.
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
