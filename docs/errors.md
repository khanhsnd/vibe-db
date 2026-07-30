# Errors across the bridge

A Go `error` reaching JavaScript degrades to a bare string. `internal/app/errmap.go` therefore
serialises every error as a JSON envelope (`code`, `message`, `field`, `detail`), and
`frontend/src/bridge/errors.ts` rehydrates it into a typed `AppError`.

**Never return a raw error from a bound method — wrap it with `wrapError`. Never match on message
text in the UI; switch on `code`.**

A new failure mode needs a `domain` sentinel, a mapping in `errmap.go`, an entry in the
`ErrorCode` union, and a row in the table below. `errmap_test.go` guards the JSON contract the
frontend depends on.

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
