# Deliberate choices — do not "fix" these

- **A PostgreSQL session keeps one connection pool per database.** A PostgreSQL connection is bound
  for life to the database it was opened with, and `pg_namespace` only ever describes that one. But
  a client has to show every database, so `driver.Namespace` carries `(database, schema)` and the
  adapter dials a database on first use, keeping its pool until the session closes. The profile's
  `database` is the one it bootstraps from and is therefore required; `Databases()` is read there,
  and `Schemas(database)` needs that database's own pool. MySQL fills `Database` alone, because its
  database *is* its schema, and its tree has one level fewer — which the frontend decides from
  `ENGINES[…].hasSchemas`, not from a null check.
- **A statement is pinned per namespace, not per session.** One pooled connection is held for the
  call and pointed at the namespace — `USE` on MySQL, `SET search_path` on PostgreSQL — so the
  pool's other connections keep whatever they had.
- **`multiStatements` stays `false`** in the MySQL DSN. Statements are split client-side, which
  removes an injection surface and allows per-statement timing.
- **TLS modes keep libpq's meaning on PostgreSQL.** The four modes the dialog offers map onto
  `disable` / `prefer` / `require` / `verify-ca`, so `Verify CA` checks the chain but not the host
  name. MySQL's `tls=true` checks both. The label names what the *engine* will do, rather than
  papering over a difference the server decides.
- **Connection pools are capped at 4 open / 2 idle.** A GUI needs no more, and it is half of the
  memory story. The cap is per pool, so a PostgreSQL session browsing three databases holds three
  pools of four.
- **Reads are bounded by default** — 1 000 rows for a query, a 100-row page for table data — rather
  than streamed unbounded into the UI. That bound is the mechanism keeping the memory claims in
  [memory.md](memory.md) true. A toolbar can lift it, through an explicit `All` or a typed count,
  but the *default* must stay bounded and nothing may opt in on the user's behalf. See
  [features.md](features.md#the-row-limit) for how the sentinel keeps the two apart on the wire.
- **Passwords never enter `connections.json`.** Profile metadata goes to
  `%APPDATA%/vibe-db/connections.json`, written atomically via temp-file + rename so a failed write
  leaves the previous file intact. The secret goes to the OS credential store via `internal/secret`.
  `repo_test.go` asserts both. The repo requests mode `0600`, which is only meaningful on POSIX —
  on Windows the file relies on inherited ACLs, so the file is not a safe place for secrets
  regardless.
- **Wails v2 is intentional over v3**, which is still alpha.
- **The clipboard goes through the Wails runtime, not `navigator.clipboard`.** Reading in a WebView2
  wants a `clipboard-read` permission the page has no way to ask for, so `bridge/clipboard.ts` calls
  `ClipboardGetText` / `ClipboardSetText` and keeps the browser API — then a hidden textarea — as
  fallbacks. That is also why it lives in `bridge`: it is the only directory allowed to import
  `wailsjs/**`.
