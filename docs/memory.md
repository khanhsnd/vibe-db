# Memory baseline

WebView2 *is* Chromium. Wails buys a ~11 MB binary and an engine shared with the OS, **not** a
small resident set. Do not repeat a claim that this shell is lightweight on RAM: choosing Wails
over Electron buys a 11 MB binary instead of a ~150 MB one, no bundled Node runtime, and a browser
engine shared with the rest of the OS — it does not buy a fundamentally smaller resident set. Any
Chromium-webview shell (Wails, Tauri, Neutralino) lands in the same place. Only a native-UI toolkit
avoids the Chromium floor.

Measured on Windows 11 (26200), Intel i5-12400, 24 GB RAM, production build, empty window,
13 other Edge/WebView2 processes already running:

| Process | Private WS | Total WS |
|---|---|---|
| `vibe-db.exe` (Go host) | 38 MB | 66 MB |
| `msedgewebview2.exe` × 6 (browser, gpu, renderer, 2× utility, crashpad) | 105 MB | 352 MB |
| **Total** | **~143 MB** | **~418 MB** |

Read the private column: total working set double-counts Chromium pages that are genuinely
shared with the system WebView2 runtime and with Edge.

Re-measure after each milestone and record it here, so a regression is visible:

| Milestone | Private WS | Total WS |
|---|---|---|
| Step 0 — empty window | 143 MB | 418 MB |
| Step 2 — MySQL connect | 116 MB | 399 MB |

Treat differences under ~30 MB as noise: the shared-page total moves with how much Edge/WebView2 is
already running, so runs are not directly comparable at that resolution.
