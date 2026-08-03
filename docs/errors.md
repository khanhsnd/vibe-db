# Errors across the bridge

A Go `error` reaching JavaScript degrades to a bare string. `internal/app/errmap.go` therefore
serialises every error as a JSON envelope (`code`, `message`, `field`, `detail`), and
`frontend/src/bridge/errors.ts` rehydrates it into a typed `AppError`.

**Never return a raw error from a bound method — wrap it with `wrapError`. Never match on message
text in the UI; switch on `code`.**

A new failure mode needs a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table below. `errmap_test.go` guards the JSON contract the
frontend depends on.

| Code | MySQL | PostgreSQL |
|---|---|---|
| `INVALID_PROFILE` | Validation failed; `field` names the offending input. It is also what a request that is well-named but not well-formed answers with — an export in a format nobody offers, or a foreign key naming a different number of columns on each side | as MySQL, plus a missing `database` |
| `INVALID_NAME` | A schema change arrived with a blank identifier; `field` says which — `table`, `column`, `column type`, `new column`, `index`, `index column`, `database`, `view`, `routine`, `foreign key`, `referenced table`, `referenced column`. Every identifier is quoted on its way into a statement, so having one is the only rule left to enforce above the server | as MySQL |
| `BAD_CREDENTIALS` | errno 1045 / 1698 | SQLSTATE 28P01 / 28000 |
| `UNKNOWN_DATABASE` | errno 1049 | SQLSTATE 3D000 |
| `UNKNOWN_SCHEMA` | — (a MySQL database *is* its schema) | SQLSTATE 3F000, which PostgreSQL raises for DDL against a missing schema; a *read* through one is 42P01 below |
| `UNKNOWN_TABLE` | errno 1146, or no columns in `information_schema` | SQLSTATE 42P01, or no rows in `pg_attribute` |
| `QUERY_CANCELLED` | Stop was pressed, or the run context was cancelled | as MySQL, plus SQLSTATE 57014 |
| `TLS_UNSUPPORTED` | `mysql.ErrNoTLS` | pgx refusing a TLS-less server |
| `SQL_ERROR` | Any other server rejection; `message` is its own wording, `detail` the errno | the same, `detail` the SQLSTATE |

`domain.StatementError.Code` is a string for exactly that last row: MySQL numbers its errors and
PostgreSQL gives a SQLSTATE like `42P01`.

**A script reports a rejection as a message rather than as a failure.** One statement fails the way
it always has, so the output panel shows the server's wording where the reader is looking; but a
script's statements before the rejected one did run, and throwing would hide that. `QueryAPI.Run`
therefore puts the same `code` and `message` a throw would carry into the failing
`StatementOutcome` and returns the run — see [features.md](features.md#query-tabs).

**A mapped code keeps the server's wording in `detail`.** Both adapters wrap the sentinel with the
engine's own message rather than replacing it, because the sentinel says *what kind* of thing was
missing and only the server says *which name* it went looking for — and on PostgreSQL that name has
been folded to lower case unless it was quoted. `errors.Is` still reaches the sentinel through the
wrapping, so the `code` the UI switches on is unchanged.

The rest do not vary by engine. Everything below `TLS_NOT_VERIFIED` is raised above the driver
layer, and the three transport rows are mapped once in `sqlkit.TransportError`.

| Code | Cause |
|---|---|
| `TABLE_NOT_EDITABLE` | Cell edit or row delete attempted on a table with no primary key, or with only part of one supplied — `sqlkit.EditableKeys` refuses both |
| `EMPTY_STATEMENT` | Run requested with nothing the splitter could find a statement in — whitespace, semicolons, or comments alone |
| `HOST_UNREACHABLE` | Dial failed (refused / no route) |
| `TIMEOUT` | Dial or context deadline exceeded |
| `TLS_NOT_VERIFIED` | Certificate chain or hostname did not verify |
| `PROFILE_NOT_FOUND` / `SESSION_NOT_FOUND` | Stale id from the frontend |
| `PASSWORD_NOT_FOUND` | Nothing stored in the credential store |
| `INTERNAL` | Anything unmapped; `detail` carries the original message |
