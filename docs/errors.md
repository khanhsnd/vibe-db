# Errors across the bridge

A Go `error` reaching JavaScript degrades to a bare string. `internal/app/errmap.go` therefore
serialises every error as a JSON envelope (`code`, `message`, `field`, `detail`), and
`frontend/src/bridge/errors.ts` rehydrates it into a typed `AppError`.

**Never return a raw error from a bound method — wrap it with `wrapError`. Never match on message
text in the UI; switch on `code`.**

A new failure mode needs a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table below. `errmap_test.go` guards the JSON contract the
frontend depends on.

| Code | MySQL | PostgreSQL | MongoDB |
|---|---|---|---|
| `INVALID_PROFILE` | Validation failed; `field` names the offending input. It is also what a request that is well-named but not well-formed answers with — an export in a format nobody offers, or a foreign key naming a different number of columns on each side | as MySQL, plus a missing `database` | as MySQL. A missing `user` is *not* one of them: a server started without access control takes no credentials at all |
| `INVALID_NAME` | A schema change arrived with a blank identifier; `field` says which — `table`, `column`, `column type`, `new column`, `index`, `index column`, `database`, `view`, `routine`, `foreign key`, `referenced table`, `referenced column`. Every identifier is quoted on its way into a statement, so having one is the only rule left to enforce above the server | as MySQL | as MySQL, with `collection` and `field` in place of `table` and `column`. Nothing is quoted into anything here — a name reaches the server as a *value* — so having one is the only rule there ever was |
| `BAD_CREDENTIALS` | errno 1045 / 1698 | SQLSTATE 28P01 / 28000 | code 18 / 11, or 8000 from a hosted cluster; also a handshake failure, which arrives as a connection error carrying no code and is read from its text |
| `UNKNOWN_DATABASE` | errno 1049 | SQLSTATE 3D000 | — (a database that is not there reads as one holding nothing, which is the server's own answer) |
| `UNKNOWN_SCHEMA` | — (a MySQL database *is* its schema) | SQLSTATE 3F000, which PostgreSQL raises for DDL against a missing schema; a *read* through one is 42P01 below | — (a MongoDB database holds its collections directly) |
| `UNKNOWN_TABLE` | errno 1146, or no columns in `information_schema` | SQLSTATE 42P01, or no rows in `pg_attribute` | code 26 (`NamespaceNotFound`), from a command against a collection that is not there. A *read* is not one of them: `find` over a missing collection answers with no documents, and inventing an error the server does not raise would be inventing one |
| `QUERY_CANCELLED` | Stop was pressed, or the run context was cancelled | as MySQL, plus SQLSTATE 57014 | as MySQL |
| `TLS_UNSUPPORTED` | `mysql.ErrNoTLS` | pgx refusing a TLS-less server | — (the driver has no opportunistic mode: TLS is on or it is not, so there is nothing to fall back from) |
| `INVALID_PROFILE` on `tlsMode` | A certificate file named under a mode that never opens one. A file the app quietly ignores is a reader believing they have configured something, so it is refused instead | as MySQL | as MySQL |
| `SQL_ERROR` | Any other server rejection; `message` is its own wording, `detail` the errno | the same, `detail` the SQLSTATE | the same, `detail` the server's own *name* for the failure — `NamespaceExists` — or its number where it sent none. It is also what a command **this side** could not read answers with, under the code `SHELL`: nothing was sent anywhere, but the wording is still the only useful thing to show |

`domain.StatementError.Code` is a string for exactly that last row: MySQL numbers its errors,
PostgreSQL gives a SQLSTATE like `42P01`, and MongoDB gives a name like `NamespaceExists`.

**`UNSUPPORTED` is not a failure.** It is a thing the engine has no concept of — a foreign key on
MongoDB, a primary key other than `_id`, a stored routine — and it is its own code so the UI can say
so plainly rather than drawing it as something the reader could retry. It comes from
`domain.UnsupportedError`, which names *which* thing. The alternative is worse: letting the server
reject the request reports a design decision as a server failure.

| Code | Cause |
|---|---|
| `UNSUPPORTED` | The engine has no such concept. `message` names it: `a foreign key`, `a primary key other than _id`, `a stored routine`, `changing the type of _id`, `dropping _id`, `cloud identity authentication on MongoDB`, `explaining <method>` |

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
| `TABLE_NOT_EDITABLE` | Cell edit or row delete attempted on a table with no primary key, or with only part of one supplied — `sqlkit.EditableKeys` refuses both, MongoDB included, where the key is always `_id` |
| `EMPTY_STATEMENT` | Run requested with nothing the splitter could find a statement in — whitespace, semicolons, or comments alone |
| `HOST_UNREACHABLE` | Dial failed (refused / no route). On MongoDB it is also every way the driver fails to *select* a server — a replica set with no primary, a name that does not resolve — which is one answer because a reader fixes all of them in the same box |
| `TIMEOUT` | Dial or context deadline exceeded |
| `TLS_NOT_VERIFIED` | Certificate chain or hostname did not verify |
| `PROFILE_NOT_FOUND` / `SESSION_NOT_FOUND` | Stale id from the frontend |
| `PASSWORD_NOT_FOUND` | Nothing stored in the credential store |
| `TUNNEL_FAILED` | The bastion could not be dialled at all — refused, no route, no such host |
| `TUNNEL_CREDENTIALS` | The bastion refused the user, the key, or the pass phrase; also a key file that cannot be read, and an agent that is not running |
| `TUNNEL_HOST_KEY` | The bastion is not in `known_hosts`, or the key there is not the one it presented. The one failure of the three that may mean something is *wrong* rather than mistyped, which is why it is its own code |
| `TOKEN_UNAVAILABLE` | A cloud identity this machine could not produce a token from. Nothing was sent to a database, so it is not a rejection; `detail` names the variable to set or the command to run |
| `INTERNAL` | Anything unmapped; `detail` carries the original message |

**The three tunnel codes are raised before the database is reached at all.** They come from
`internal/sshtunnel`, which maps them from `x/crypto/ssh`'s own failures — and that package is one of
two in the app that read an error's *message*, because `x/crypto/ssh` offers no code to switch on. It
is the same trade an engine adapter makes with an errno: the text is the library's and not anything a
reader typed, and what the UI switches on is still the code it produces.

**The other is `driver/mongo`**, for the same reason: a handshake that fails at authentication
arrives wrapped in a connection error whose type carries nothing to switch on, so `authWordings`
reads it. Both are *adapters* reading a library's text. The UI still never does.
