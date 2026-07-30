# Deliberate choices — do not "fix" these

- **A PostgreSQL schema is browsed where a MySQL database is.** `driver.Session` has one notion of
  a namespace a table is qualified with, and a PostgreSQL session cannot cross to another database
  the way `USE` crosses to another MySQL one. So the adapter answers `Databases()` with the schemas
  of the one database the profile names, and pins a statement with `SET search_path` where MySQL
  uses `USE`. The alternative — listing real databases — would mean a pool per database and a
  reconnect on every click, for a level of the tree the connection cannot leave anyway. The
  profile's `database` is therefore required for PostgreSQL, and the UI says *Schema* wherever the
  engine calls it that.
- **`multiStatements` stays `false`** in the MySQL DSN. Statements are split client-side, which
  removes an injection surface and allows per-statement timing.
- **TLS modes keep libpq's meaning on PostgreSQL.** The four modes the dialog offers map onto
  `disable` / `prefer` / `require` / `verify-ca`, so `Verify CA` checks the chain but not the host
  name. MySQL's `tls=true` checks both. The label names what the *engine* will do, rather than
  papering over a difference the server decides.
- **Connection pools are capped at 4 open / 2 idle** per session. A GUI needs no more, and it is
  half of the memory story.
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
