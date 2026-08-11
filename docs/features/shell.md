# The window shell

[← Features and UI behaviour](../features.md)

## The menu bar

The strip above the command bar carries `File`, `Edit`, `View`, `Favorites`, `Tools`, `Window` and
`Help`. **Only `Tools` and `Window` do anything yet** — the other five are names in the place a
reader of a desktop client looks for them, and nothing behind them. Items are `MenuItem`s drawn by
`MenuButton`, the same object the tree's right-click menu is built from, so an action offered both
ways stays one object, and a menu given no items draws its name alone.

- **`Tools` opens the History Log**, which is asked for here and nowhere else: it belongs to the
  window rather than to a connection, so there is no node in the tree it could hang off. What it
  shows is [features/log.md](log.md).
- **`Window` lists every open tab**, in the order the strip draws them, and picking one is exactly
  what clicking it in the strip is. It is the one place that names all of them at once: the strip
  truncates each title and scrolls the moment it holds more tabs than it can draw. Each row wears
  its tab's kind glyph in the tree's colour for that kind, and says **where** the tab is —
  `users — shop.public · Local MySQL` — because a strip has room for the title alone and two tabs
  called `users` on two servers read there as one tab. **The tab in front wears a tick** in place of
  its glyph, which is the one fact the strip stops telling the reader once it has scrolled. With
  nothing open the menu says `No open tabs` rather than opening empty.

## Every divider in the window is dragged

There are five, and they behave alike: **the sidebar against the work area**, **the editor against
the output panel** in a query tab, in a table tab **the column list against the grid**, and **the
grid against the cell editor** wherever there is one — a table tab and a query result both. Each is
the one-pixel line between the two panels, and it goes
`--accent` while the pointer is over it or holding it. The pointer is caught within ten pixels of the
line, so the target is a good deal wider than the line looks.

- **Double-clicking a divider puts it back where it opened**, which is the way out of a drag that
  went too far without hunting for the size that was there before.
- **A divider takes the keyboard**: `Tab` reaches it, the arrow keys across its orientation move it,
  and `Home` / `End` take it to either end.
- **Dragging the sidebar's divider past its minimum closes the sidebar**, which is the same thing the
  command bar's toggle does — and the toggle is told, so the next press opens it rather than being the
  press that appears to do nothing.

**A layout is remembered per window and per combination of panels**, in `localStorage`, and only when
a *reader* moved it: a panel opening or the window being resized is not a new preference. Every
divider is bounded so a drag cannot leave a panel too small to read what is in it — the cell editor
stops at its own chrome plus a line of value, the column list at 420 px, and the grid keeps three
quarters of the height whatever the editor under it is given.

Column widths inside a grid are dragged too, and are the grid's own business —
see [grid.md](../grid.md).

## Tabs

**Every tab says in full what it is showing when the pointer rests on it**: what kind of thing and
its name, then `Database:`, `Connection:` and `Server:` as `user@host:port`. The strip truncates to a
name at 220 px, and two tabs called `users` on two connections are otherwise the same tab — the
tooltip is where that is settled. It is lines rather than a sentence because a reader is looking for
one of those facts, not reading prose, and it is the platform's own tooltip rather than a component,
which is what makes it appear over a tab the pointer merely passes.

**Only the tab in front is mounted**, so anything a tab is holding in component state is taken down
with it the moment the reader looks at another one. What each kind of tab keeps across that is
therefore a deliberate list, kept in a store under the tab's id and dropped when the tab closes:

- A **table tab** keeps *what was asked of it* (`stores/tableViews.ts`): the panels open — Cell
  Editor, Column, Filter & Sort — the columns hidden, the page and its size, the order and the
  conditions. It does **not** keep the answer: the rows are read again on the way back, so a table
  something else has changed under it reads fresh rather than stale, and the status line says when.
  The column *order* is remembered against the table itself and outlives the tab entirely.
- A **query tab** is the other way round and keeps its rows (`stores/queries.ts`), because a
  statement cannot be re-run behind the reader's back: it may write, and it may be expensive. It is
  also the one kind that outlives the *app* closing, buffer and all — see
  [features/query-tab.md](query-tab.md#the-tabs-are-still-there-next-time).
- A **compare tab** keeps only *which half* it was showing — its two tables and its mode are the
  tab itself. The diff is read again on the way back, for the reason a table's rows are: a
  comparison that had gone stale would be worse than the second read that replaces it.
- A **Find in Schema tab** keeps its hits (`stores/searches.ts`), for the reason a query tab keeps its
  rows: a search of a whole schema is far too long a read to lose by looking at another tab. What was
  *asked* — the text, `Look In`, `Search Mode`, the box — is the tab itself, exactly as a query tab's
  SQL is, and so is where it is pointed.
- The **History Log** keeps both its lines and its switches (`stores/log.ts`), which no other tab
  does: it is one log for the whole window rather than one per tab, and re-reading it on the way
  back would lose the place the reader had scrolled to. See
  [features/log.md](log.md#how-it-is-kept).
- A **Backups tab** keeps nothing at all. The list is the tree's own — one folder, drawn twice —
  and it is read again on the way back for a stronger reason than a table's rows are: another
  window of this app, or the reader with a file manager, changes the folder without this one
  hearing. See [features/backup.md](backup.md#the-list).
- A **Diagram tab** keeps the picture (`stores/diagrams.ts`) — which tables are on the canvas, where
  each sits, the notes beside them, and the steps to undo. It is the only one whose state also
  outlives the *window*, written down per namespace rather than per tab, so it is read from disk
  when the tab opens rather than started empty. What is *in* the boxes is not kept at all: that is
  the catalogue's, and is read like any other tab's answer. See
  [features/diagram.md](diagram.md#where-it-is-kept-and-taking-it-away).

**A tab is named by what it shows, and a comparison by both sides**: `users ⇄ users_v2`, and the
tooltip carries `Against:` as well, because two tabs comparing `users` against two different servers
are otherwise the same tab. Asking for a pair that is already open **reveals that tab** — the mode
is state inside it, not part of what names it, so `Compare ▸ Data Only` on a comparison showing
structure switches that one rather than opening a second on the same pair.

**The wheel closes a tab**, as it does in every strip of tabs there is, and the `✕` that appears
under the pointer is the other way. The press itself is refused: Chromium's middle-click autoscroll
starts there and swallows the click that would follow, and the strip scrolls the moment it holds
more tabs than it can draw — so without that the gesture worked on a short strip and quietly stopped
working on a full one.

## Options

The gear at the right of the command bar opens **Options** (`features/settings/SettingsDialog`): a
rail of sections on the left, the one in force on the right, which is the shape a reader already
knows a settings dialog by. Sections are added to that rail — never a second dialog.

| Section | Holds |
|---|---|
| General | **Theme** — dark or light, the same switch the command bar's sun/moon is. |
| Editor | The **data font** — family, size, weight, with a preview line — and **word wrap** for the SQL editor, which reconfigures a live editor through a CodeMirror compartment rather than rebuilding it. |
| Records | What a **new tab** starts with: records per page (a table read and a query result alike) and the row limit a query tab caps its read at. Both are defaults, read once when a tab mounts, so changing one never moves a result the reader has already paged through. |

**A pick applies as it is made**, not behind an OK: the app behind the dialog is the preview, and a
dialog that could be dismissed half-set would need a draft of every preference to do it. The footer
therefore offers **Restore Defaults** and **Done**, and nothing that cancels. Everything is kept in
`localStorage` under one key, `vibe-db.settings`, by `stores/settings.ts` — the one place a
preference is saved and put in force.

### The font list is the machine's

The family list is **every font installed on this machine**, not a list the app ships: `SystemAPI.
ListFonts` reads the Windows font registry — the machine-wide hive *and* the reader's own, since a
font installed for one user is registered only there — strips the `(TrueType)` tag, folds a family's
`Regular`, `Italic`, `Bold` and `Oblique` variants back into it, and answers sorted. `Segoe UI Light`
and `Arial Black` survive whole, because those are families a stylesheet can name; `Arial Bold` is
Arial asked for in bold and is folded away. A few hundred families is normal, so the list is
**searched rather than scrolled**, and every row is drawn in its own family — the only honest preview
of a pick. Outside the desktop shell (a browser dev run) the host answers with nothing and the picker
offers the six families in `FALLBACK_FAMILIES`.

The default is **Cascadia Mono light 13 px**: a screenful of rows reads heavier than a paragraph of
the same weight, so data is set a weight below regular. A weight only draws as itself where the
family has it — Cascadia is variable and does, Consolas and Lucida Console ship 400 and 700 alone, and
Windows then rounds to the nearest they have. Sizes run 10–16 px, which is what a 28 px row can hold.

The pick is written onto the root element as `--font-data`, `--font-data-size` and
`--font-data-weight`, ahead of `--font-data-fallback`, which is why nothing rebuilds, why a family the
machine turns out not to have still lands on something monospaced, and why a column fit measures the
font the reader actually chose.
