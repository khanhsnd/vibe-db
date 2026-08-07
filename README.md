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

Connection profiles, connect / disconnect, database and table browsing, table data with pagination
and cell editing, query tabs with schema-aware completion, writable query results, passwords in
Windows Credential Manager, dark and light themes.

Schema browsing stops at tables, and one Execute runs one statement.
[docs/features.md](docs/features.md) has the detail.

## Quick start

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
wails doctor    # verify the toolchain
wails dev       # hot-reload dev build
wails build     # production build -> build/bin/vibe-db.exe
go test ./...   # backend tests
```

Full prerequisites, the command table and how to drive the backend from a browser are in
[docs/development.md](docs/development.md).

## Documentation

| Document | Contents |
|---|---|
| [docs/architecture.md](docs/architecture.md) | Layers, the one-way dependency rule, adding an engine, the two Wails constraints |
| [docs/errors.md](docs/errors.md) | The error envelope and the full error-code table |
| [docs/features.md](docs/features.md) | What works, as an index over [docs/features/](docs/features/) — one file per feature: connections, the tree, query tabs, the table tab, the grid's panels |
| [docs/editor.md](docs/editor.md) | CodeMirror setup, the four completion sources, the load-bearing colours |
| [docs/frontend.md](docs/frontend.md) | Path aliases, theme tokens, `Split`, TypeScript strictness, Zustand selectors |
| [docs/decisions.md](docs/decisions.md) | Deliberate choices that should not be "fixed" |
| [docs/memory.md](docs/memory.md) | Measured memory baselines, and why a webview shell is not light |
| [docs/development.md](docs/development.md) | Toolchain, commands, verifying against a real database |
