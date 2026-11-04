# vibe-db

A desktop database client in the Navicat Premium mould. MySQL first, other engines behind the
same port.

## Install

Builds are published to this repository's
[Releases](https://github.com/khanhsnd/vibe-db/releases/latest). **Nothing
there is code-signed or notarized**, so each platform asks once before it runs an unknown publisher.
How a version is built and how the app updates itself are in
[docs/features/updates.md](docs/features/updates.md).

### Windows 10/11 x64

Download `vibe-db-<version>-windows-x64-setup.exe` and run it. Defender SmartScreen shows *"Windows
protected your PC"* — choose **More info → Run anyway**. The installer then raises the ordinary
elevation prompt. Later versions install themselves from Settings → Updates.

### macOS 12+, Intel or Apple Silicon

```sh
brew tap khanhsnd/vibe-db https://github.com/khanhsnd/vibe-db
brew install --cask khanhsnd/vibe-db/vibe-db
```

Or download `vibe-db-<version>-macos-universal.zip`, unzip it and move `vibe-db.app` into
`/Applications`. Gatekeeper refuses the first launch of a direct download; open it once from the
Finder context menu with **Open**, or clear the quarantine flag:

```sh
xattr -dr com.apple.quarantine /Applications/vibe-db.app
```

### Linux x64

The archive holds a binary rather than a package, so it is unpacked wherever you keep one:

```sh
tar -xzf vibe-db-1.0.7-linux-x64.tar.gz
cd vibe-db-1.0.7-linux-x64
install -Dm755 vibe-db ~/.local/bin/vibe-db
install -Dm644 vibe-db.png ~/.local/share/icons/hicolor/512x512/apps/vibe-db.png
install -Dm644 vibe-db.desktop ~/.local/share/applications/vibe-db.desktop
```

The desktop entry runs `vibe-db` by name, so `~/.local/bin` has to be on `PATH` — otherwise put the
binary somewhere that already is, such as `/usr/local/bin`.

Three runtime libraries have to be present. On Debian and Ubuntu:

```sh
sudo apt-get install libgtk-3-0 libwebkit2gtk-4.0-37 gnome-keyring
```

Elsewhere they are that distribution's GTK 3, its **WebKit2GTK 4.0**, and any Secret Service
provider — `gnome-keyring` or KWallet. The keyring is not optional: it is where a connection
password is kept, and there is no fallback that writes one to disk.

**The published binary links against WebKit2GTK 4.0, not 4.1.** A distribution that ships only 4.1 —
Ubuntu 24.04 among them — will not run it. Build from source there instead:

```sh
wails build -platform linux/amd64 -tags webkit2_41
```

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
