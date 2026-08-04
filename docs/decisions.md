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
- **TLS modes keep libpq's meaning on both engines.** The five modes map onto
  `disable` / `prefer` / `require` / `verify-ca` / `verify-full`, and `Verify CA` therefore checks
  the chain and *not* the host name — the only reading that works against a server addressed by an
  address. Go verifies the chain and the name together or not at all, so MySQL's two verifying modes
  are a `*tls.Config` this side builds, with `verify-ca` doing its own chain-only check in
  `VerifyPeerCertificate`. `Required` is encryption and no verification, on both, and the label says
  so rather than implying a weaker check.
- **A certificate file named under a mode that never opens one is refused, not ignored.** A file the
  app quietly drops is a reader believing they have configured something.
- **The SSH tunnel forwards no local port.** It dials the bastion and opens connections *from* it
  through the SSH channel, which the driver receives as a `driver.Dialer`. A listening port would be
  one more thing to collide, one more thing anything on the machine could reach, and one more thing
  to tidy up when a session ends. It also means the profile's host and port are resolved on the far
  side, which is what makes `127.0.0.1` mean the bastion's loopback.
- **A bastion not in `known_hosts` is refused by default.** A tunnel whose far end is not identified
  protects the credentials it carries from nobody, which is the one thing it exists to do. The
  checkbox that turns it off exists for a host that genuinely cannot be enrolled.
- **The host-key algorithms offered are the ones `known_hosts` already holds** — `hostKeyAlgorithms`
  in `internal/sshtunnel`. Do not delete this and trust the library's defaults. A server usually
  offers three host keys (RSA, ECDSA, Ed25519) and presents whichever the client asks for first;
  `x/crypto/ssh` asks in *its own* preference order, RSA and ECDSA above Ed25519, while
  `known_hosts` commonly holds only one of the three — so the server hands over a key the file has
  never seen and the check reports **`knownhosts: key mismatch` for a host whose key has not
  changed**. OpenSSH, `ssh` on the command line and every client built on libssh avoid this by
  putting the known algorithms first, which is why such a host connects everywhere else and failed
  only here. The list is read by *probing* the callback with a key no server has, because
  `x/crypto/ssh/knownhosts` exposes its entries no other way. An `ssh-rsa` entry expands to
  `rsa-sha2-512` and `rsa-sha2-256` as well, since it names a key and not a signature algorithm and
  asking for `ssh-rsa` alone asks for SHA-1, which OpenSSH 8.8 and later refuse.
- **A cloud token is never stored.** AWS IAM and Google IAM passwords are minted per connection and
  last minutes; keeping one in the credential store would only produce a failure later that looked
  like a bad password. `SavePassword` is forced off for them and the connect prompt is skipped.
- **No cloud SDK is pulled in for either.** An RDS token is a documented SigV4 signing algorithm and
  a Google access token is a documented OAuth exchange; both are under a hundred lines of standard
  library, against two dependency trees larger than this application. The cost is that AWS
  credentials are read from the **environment** alone — a named profile or an SSO cache is resolved
  by exporting them, which is what the dialog says.
- **Only production is guarded.** A confirmation a reader meets on every environment is one they
  stop reading, and that habit is exactly what the label exists to prevent. An *unfiltered* `UPDATE`
  or `DELETE` is confirmed everywhere, because that one is about what the statement is rather than
  about where it is going. The rule is `domain.Environment.NeedsConfirmation` and the frontend
  receives its answer rather than re-deriving it.
- **The reviewer is a lexer, not a parser.** It reports only what it can prove — an unclosed quote,
  comment or bracket — and leaves every question about names, clause order and types to the server.
  A client that guessed at those would be wrong often enough that a reader learned to ignore it,
  which costs more than the warning saved.
- **The formatter changes nothing a server would notice**, and hands back untouched any script it
  cannot lex. A formatted script that no longer runs is worse than an unformatted one, and a test
  compares the token stream before and after to keep that true.
- **A value that is not valid UTF-8 crosses the wire as `\x…` hex.** It is not a preference: a bound
  method's result reaches the WebView as one JSON string, and bytes that are not text are replaced
  character by character on the way, irrecoverably. Bytes that *are* text stay text, because a
  `BLOB` holding JSON is commoner than one holding a PNG. `sqlkit.TextOrNil` is the only place that
  decides it, and it decides from the bytes rather than from the column's declared type — which is
  also why it cannot be wrong about a `TEXT` column an engine reports as a `BLOB`.
- **A GeoJSON value is drawn as a plain SVG and never on a map.** A basemap means tiles, and tiles
  mean sending the coordinates in a reader's database to a third party to find out where they are.
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
- **MongoDB's query tab reads the shell as a query language, not as a language.** A command, its
  Extended-JSON-ish arguments and the cursor methods bounding it — and no variables, no arithmetic and
  no control flow. Half a JavaScript is worse than none: a reader could not tell which half they were
  in, and every answer would carry that doubt. Anything past the list `query.go` understands is
  `db.runCommand({…})`, which reaches the server verbatim and is a smaller thing to learn than an
  interpreter's limits.
- **A MongoDB collection's field list is a sample, and says so.** There is no catalogue to ask, and the
  alternatives are worse: an aggregation over every document to find every field is a full scan on
  opening a table, and `$sample` gives a different column list on every refresh. The first few hundred
  documents in natural order is incomplete but *stable*, and a bounded page fills in what it missed.
  See [grid.md](grid.md#where-the-columns-come-from).
- **What an engine has not got is refused here, not by the server.** A foreign key on MongoDB, a
  primary key other than `_id`, a stored routine: each answers `UNSUPPORTED` from
  `domain.Unsupported`. Letting the server reject the request would report a design decision as a
  server failure, and approximating it — a unique index standing in for a primary key — would be
  worse still: the reader would believe they had something they had not.
- **MongoDB's `Truncate` is not offered at all**, rather than being made an alias of `Empty`. A
  collection has no storage to reset, so `deleteMany({})` is the only way to empty one; `drop` and
  recreate would be faster and would silently lose every index. Two menu items running one command
  under two names is a lie about what the second does.
- **A new MongoDB collection gets no `$jsonSchema` validator**, however many fields the designer was
  given. Building one from them was tried and it is a trap: a validator has to be kept in step by
  every field change that follows, and the moment one is not — `AlterColumn` converting a field the
  schema still calls an `int` — the collection starts rejecting the very edits the designer offers. A
  collection that will not accept its own designer's writes is worse than one that declared nothing,
  which is what a MongoDB collection normally is. A new collection's fields arrive with its first
  document, and the Fields grid reads them back off it like every other collection's.
- **A `system.` collection is not listed.** `system.views` appears the moment a view is made and
  `system.profile` the moment profiling is turned on. Neither is something the reader created, and a
  rename or a drop of one is a corrupted database — so the prefix the server reserves is the whole
  rule, and nothing under it reaches the tree.
- **A MongoDB field's *type* is changed by converting the documents**, with `onError: "$$REMOVE"`. A
  value the server cannot convert is removed rather than left wrong, because that is the only outcome
  in which the collection afterwards holds what the reader declared it holds. It is the one field
  operation with no SQL-shaped answer, and it is the honest reading of what `ALTER COLUMN … TYPE`
  means where there is no column.
- **The clipboard goes through the Wails runtime, not `navigator.clipboard`.** Reading in a WebView2
  wants a `clipboard-read` permission the page has no way to ask for, so `bridge/clipboard.ts` calls
  `ClipboardGetText` / `ClipboardSetText` and keeps the browser API — then a hidden textarea — as
  fallbacks. That is also why it lives in `bridge`: it is the only directory allowed to import
  `wailsjs/**`.
