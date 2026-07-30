# vibe-db

A desktop database client in the Navicat Premium mould. MySQL first, other engines behind the
same port.

## Stack

| Layer | Choice |
|---|---|
| Shell | Wails v2.13.0 (Go host + OS WebView2) |
| Backend | Go 1.26 |
| Frontend | React 19, TypeScript, Vite 7 |
| Editor | CodeMirror 6 (`@codemirror/lang-sql`, MySQL dialect) |
| Styling | Tailwind CSS v4 over CSS custom properties |
| State | Zustand |

## Status

Working: connection profiles (create / edit / delete), Test Connection with server version and
latency, connect / disconnect, database listing, table listing per database, table data browsing
with pagination and single-cell editing, passwords in Windows Credential Manager, and the full
layout shell with dark and light themes.

Clicking a database marks it as the active one for that connection and reveals its `Tables` folder,
which loads on first expand and is cached until disconnect.

Clicking a table opens a data tab: 100 rows per page by default (500 and 1 000 also offered, which
is the hard cap), ordered by primary key so paging is stable. Above the grid sit two independent
toggles, since they occupy different edges:

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft,
  Apply issues `UPDATE … WHERE <primary key> LIMIT 1` and reloads, Refresh reloads. A table with no
  primary key is read-only, because no single row can be identified.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes.

A query tab carries its own toolbar, editor buffer, database and row limit, so several tabs can sit
on the same connection without treading on each other. The toolbar picks the database, then
**Execute** (`F5` or `Ctrl`+`Enter`) or **Explain** runs the statement; **Stop** cancels it.

- The database is applied with `USE` on a connection pinned for that one statement, so the pool's
  other connections keep whatever they had.
- A statement that returns rows fills the grid; one that does not reports its affected-row count.
  Which path a statement takes is decided by its leading keyword, read past any leading comments.
- Results are capped at the toolbar's row limit (1 000 by default, 10 000 at most). Hitting the cap
  is reported in the status strip rather than passed off as the whole answer.
- **Errors surface in the output panel below the editor**, with the code, the server's own wording
  and the errno, never as a dialog. A cancel is reported as `QUERY_CANCELLED`, not as a failure.
- Progress shows in two places: a spinner beside the toolbar buttons and a `Running…` status strip
  under the output panel. Previous rows stay on screen until the new ones land.

One Execute runs one statement — a pasted multi-statement script is rejected by the server, since
`multiStatements` stays off and the client-side splitter is not written yet. Schema browsing stops
at tables; views, procedures and functions are not listed yet.

### Editor and completion

The editor is CodeMirror 6 with the MySQL dialect: line numbers, syntax highlighting off the same
`--syn-*` tokens as the rest of the app, bracket matching, undo history and soft wrapping. Each tab
owns its editor, so the caret and the undo stack do not follow a tab switch.

**A selection narrows what runs.** Highlight one statement of a script and Execute sends only that;
with nothing selected it sends the whole buffer.

Completion fires as you type and on `Ctrl`+`Space`, over four sources:

- **Tables** of the tab's database, with their column count as the hint.
- **Qualified columns**, after `table.` or `alias.` — `FROM orders o … o.` resolves the alias.
- **Bare columns** anywhere else in the statement — after `SELECT`, after `WHERE`, after
  `ORDER BY` — drawn from every table that statement names, each labelled with its table. A join
  offers both sides. This is a source of ours, not lang-sql's, which only completes behind a dot.
- **Keywords** of the dialect, offered upper-case, and **statement snippets** (`select`, `insert`,
  `join`, `group by`, `with`, …). `Tab` walks the snippet's fields.

The last two are suppressed after a dot and inside strings and comments, where only the qualified
schema has anything useful to say.

`ConnectionAPI.LoadSchema` reads a whole database's tables and columns in one
`information_schema.columns` scan, cached per connection-and-database until disconnect. Switching
the toolbar's database reconfigures completion in place rather than rebuilding the editor.

One upstream limitation is worth knowing: a table whose name is a reserved MySQL keyword — `user`,
`plugin` — is tokenised as a keyword, so `FROM user u` does not register the alias `u` and `u.`
falls back to keyword matches. Quoting it (`FROM \`user\` u`) restores column completion. Bare
column completion is unaffected, because it matches words against the schema rather than trusting
the parser's token types.

Two things about the editor's colours are load-bearing, not cosmetic:

- `--selection` is applied through
  `&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground`. CodeMirror's own
  focused-selection rule is five classes deep, so the obvious `&.cm-focused .cm-selectionBackground`
  loses the cascade and the library's default lavender wins in both themes.
- `--line-highlight` (the active line) **must stay translucent**. The selection layer sits at
  `z-index: -2`, behind the lines, so an opaque active-line background paints over the selection.

### Grid behaviour

`ResultGrid` is shared by every table in the app, so these apply everywhere it is used:

- **Cells select, text does not.** Dragging paints a rectangular cell range instead of running a
  text selection across column boundaries; shift-click extends from the anchor. `Ctrl`/`Cmd`+`C`
  copies the range as tab-separated rows, with `NULL` written out the way the grid shows it.
- **Columns resize** by dragging the right edge, which reveals an expand icon on hover.
  Double-clicking that edge fits the column to its widest cell (capped at 520 px), measured with
  a canvas against the `--font-mono` / `--text-sm` tokens. Widths are keyed by column *name*, not
  position, so hiding or showing a column does not reset them.

The grid keeps its own selection when left uncontrolled, so a new call site gets this for free; the
table data view passes `selection` in only because the Cell Editor needs the anchor.

## Prerequisites

- Go 1.26+
- Node 22+ and pnpm 10+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`
- WebView2 runtime (preinstalled on Windows 11)

Verify the toolchain with `wails doctor`.

## Development

```sh
wails dev      # hot-reload dev build
wails build    # production build -> build/bin/vibe-db.exe
go test ./...  # backend tests
```

`frontend/wailsjs/` is generated by Wails on every build and is committed so a fresh clone
type-checks without first running a Go build.

## Architecture

Ports and adapters. The dependency rule is one-way:

```
app  ->  session / profile / secret  ->  driver  ->  domain
```

- `internal/domain` — pure types and validation. Imports nothing of ours.
- `internal/driver` — the `Driver`/`Session` ports; `driver/mysql` is the first adapter.
  A new engine is a new adapter, not a change to anything above.
- `internal/app` — the only package bound to the frontend. Speaks `dto` types, never domain
  types, and maps every error to a stable code (see `errmap.go`).
- `frontend/src/bridge` — the only frontend directory allowed to import `frontend/wailsjs/**`.
  Wails regenerates those bindings on every build, so wrapping them once keeps a signature
  change to a one-file fix.

### Errors across the bridge

A Go `error` reaching JavaScript degrades to a bare string, so `internal/app/errmap.go`
serialises every error as a JSON envelope (`code`, `message`, `field`, `detail`) and
`frontend/src/bridge/errors.ts` rehydrates it into a typed `AppError`. **The UI switches on
`code` and never matches on message text.**

| Code | Cause |
|---|---|
| `INVALID_PROFILE` | Validation failed; `field` names the offending input |
| `BAD_CREDENTIALS` | MySQL errno 1045 / 1698 |
| `UNKNOWN_DATABASE` | MySQL errno 1049 |
| `UNKNOWN_TABLE` | MySQL errno 1146, or the table has no columns in `information_schema` |
| `TABLE_NOT_EDITABLE` | Cell edit attempted on a table with no primary key |
| `EMPTY_STATEMENT` | Run requested with nothing but whitespace or semicolons |
| `SQL_ERROR` | The server rejected the statement; `message` is its own wording, `detail` the errno |
| `QUERY_CANCELLED` | Stop was pressed, or the run context was cancelled |
| `HOST_UNREACHABLE` | Dial failed (refused / no route) |
| `TIMEOUT` | Dial or context deadline exceeded |
| `TLS_UNSUPPORTED` | Server has no TLS but a TLS mode required it |
| `TLS_NOT_VERIFIED` | Certificate chain or hostname did not verify |
| `PROFILE_NOT_FOUND` / `SESSION_NOT_FOUND` | Stale id from the frontend |
| `PASSWORD_NOT_FOUND` | Nothing stored in the credential store |
| `INTERNAL` | Anything unmapped; `detail` carries the original message |

Adding a database engine means adding `internal/driver/<engine>` implementing `driver.Driver`
plus an errno translation, and registering it in `internal/app/app.go`. Nothing above the
driver layer changes.

### Notes

- Bound methods deliberately do **not** take `context.Context`. Wails v2 would expose it as a
  real JavaScript argument rather than injecting it, so `ConnectionAPI` holds the startup
  context and derives per-call timeouts from it.
- `multiStatements` stays off in the DSN. Statements are split client-side, which removes an
  injection surface and allows per-statement timing.
- Connection pools are capped at 4 open / 2 idle per session; a GUI needs no more.

## Memory baseline

Measured on Windows 11 (26200), Intel i5-12400, 24 GB RAM, production build, empty window,
13 other Edge/WebView2 processes already running:

| Process | Private WS | Total WS |
|---|---|---|
| `vibe-db.exe` (Go host) | 38 MB | 66 MB |
| `msedgewebview2.exe` × 6 (browser, gpu, renderer, 2× utility, crashpad) | 105 MB | 352 MB |
| **Total** | **~143 MB** | **~418 MB** |

Read the private column: total working set double-counts Chromium pages that are genuinely
shared with the system WebView2 runtime and with Edge.

**This is higher than a webview shell intuitively suggests, and it is worth being blunt about:
WebView2 *is* Chromium.** Choosing Wails over Electron buys a 11 MB binary instead of a ~150 MB
one, no bundled Node runtime, and a browser engine shared with the rest of the OS — it does not
buy a fundamentally smaller resident set. Any Chromium-webview shell (Wails, Tauri, Neutralino)
lands in the same place. Only a native-UI toolkit avoids the Chromium floor.

Re-measure after each milestone and record it here, so a regression is visible:

| Milestone | Private WS | Total WS |
|---|---|---|
| Step 0 — empty window | 143 MB | 418 MB |
| Step 2 — MySQL connect | 116 MB | 399 MB |

Treat differences under ~30 MB as noise: the shared-page total moves with how much
Edge/WebView2 is already running, so runs are not directly comparable at that resolution.
