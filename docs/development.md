# Development

## Prerequisites

- Go 1.26+
- Node 22+ and pnpm 10+ (pnpm is what `wails.json` invokes — do not switch to npm)
- WebView2 runtime (preinstalled on Windows 11)

The Wails CLI is not vendored. Install it once, and note `$GOPATH/bin` may not be on PATH:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
export PATH="$PATH:$(go env GOPATH)/bin"   # PowerShell: $env:PATH += ";$(go env GOPATH)\bin"
wails doctor
```

## Commands

| Task | Command |
|---|---|
| Dev build, hot reload | `wails dev` (from repo root) |
| Production build | `wails build` → `build/bin/vibe-db.exe` |
| Go tests | `go test ./...` |
| Single Go test | `go test ./internal/driver/postgres -run TestBuildDSNTLSModes -v` |
| MySQL against a real server | `VIBE_MYSQL_DSN=root:mysql@127.0.0.1:3306 go test ./internal/driver/mysql -run TestLive -v` |
| PostgreSQL against a real server | `VIBE_POSTGRES_DSN=postgres:postgres@127.0.0.1:5432/postgres go test ./internal/driver/postgres -run TestLive -v` |
| MongoDB against a real server | `VIBE_MONGO_DSN=root:mongodb@127.0.0.1:27017 go test ./internal/driver/mongo -run TestLive -v` |
| Redis against a real server | `VIBE_REDIS_DSN=127.0.0.1:6379 go test ./internal/driver/redis -run TestLive -v` |
| Go vet + format | `go vet ./...` and `gofmt -l internal main.go` |
| Frontend typecheck | `pnpm typecheck` (in `frontend/`) |
| Frontend build | `pnpm build` (in `frontend/`) — runs `tsc -b` then `vite build` |

There is **no frontend test runner installed**. Do not write frontend tests assuming vitest or
Testing Library exists — add the dependency first, or verify frontend behaviour by running the app.

## Verifying against a real database

All seven engines are exercised through the same UI, but each has a failure mode the others cannot
show — `USE` versus `SET search_path` versus `ALTER SESSION` versus neither versus a client per
database, an errno versus a SQLSTATE versus an `ORA-` number versus a result code versus a first
word, `LIMIT 1` on an update versus none. A change to any adapter deserves a run against that
server; a PostgreSQL profile needs a database name before it will connect at all, an Oracle one
needs a *service* name, a MongoDB one needs neither a database nor a user, and a Redis one takes a
database *number*.

**SQLite is the exception, and it is exercised by `go test`.** It is a file, so
`internal/driver/sqlite` opens one in a temporary directory and drives the whole adapter against it
— the catalogue reads, every mode of a row write, the two schema changes a file takes, and the
plan. That run is worth watching after **any** change to `sqlkit`: `sqlkit.Data` and `sqlkit.DDL`
build those statements for all five SQL engines, so a mistake in the paging, the placeholders or the
quoting shows up there as a failure rather than as a string that merely looks right.

**MongoDB is where a run against a real server matters most**, because the two things that cannot be
unit-tested are exactly the two that make it different: a field list is inferred from documents, so it
is only right if real documents say so, and a command is interpreted rather than passed through, so
only a server can confirm the method did what its name says.

`TestLiveAgainstARealServer` in `internal/driver/mongo` is that run, and it is worth doing before the
UI: it drives the whole adapter — every catalogue read, every field change, every command the query tab
understands — against a server, in a database of its own that it drops afterwards. It **skips** unless
`VIBE_MONGO_DSN` names one, so `go test ./...` stays offline. The user and the password are both
optional there (`127.0.0.1:27017` alone is a server with no access control).

**Redis has two of its own**, and `TestLiveAgainstARealServer` in `internal/driver/redis` covers
both: a page is cut from a *scan*, which only a server has, and a value is read by a different
command per type, which only a server holds. It works in database 15 and empties it first, and it
**skips** unless `VIBE_REDIS_DSN` names a server — `127.0.0.1:6379` alone is a server with no
password, and `user:password@host:port` is one with an ACL.

Then, by hand in the app:

| What | Why it needs a server |
|---|---|
| A collection whose documents have **different fields** | The header is a union of a sample; a bounded page must show a field the sample missed, and an `All` read must not claim to |
| A collection holding **nested documents and arrays** | The grid draws them in the shell's notation, **View Value** reads them whole, and a cell edited in that notation has to go back as the document it showed |
| An `_id` that is **not** an ObjectId — a string, an int | Editing and deleting address a document by `_id` read into its sampled type |
| A **view** | `$collStats` refuses one, so its row in the collection list must carry nulls rather than zeroes |
| `Empty`, then **Add Field** / **Edit Field** / **Delete Field** | Each is an update over the documents, and an empty collection is the case with nothing to update |
| A **replica set through an SSH tunnel** | The connection is pinned `Direct`, or discovery hands back addresses that resolve only on the far side |

And for the server monitor, which is the one pane whose rows come from a *server's own state* rather
than from a catalogue of objects:

| What | Why it needs a server |
|---|---|
| A **second connection running something slow** | The process list is only itself with somebody else in it, and a statement column is what the reader picks a row by |
| **Cancel Statement**, then **End** on that connection | The two are different statements on the engines that have both, and only a server says whether the connection survived the first |
| The row marked as **this app's own** | It is read from a second scalar question beside the list; on a pool of four, which connection answers is `database/sql`'s choice, and the mark has to land on one row and not none |
| **Status** and **Variables** on each engine | Every alias in those reads is what the grid draws as a header — a name spelled wrongly is not a failure, it is a wrong word on the screen |
| A reader **without the privilege** | Each engine refuses differently: MySQL shows their own threads, PostgreSQL blanks the statements, the rest refuse in the server's own words |

And for Redis:

| What | Why it needs a server |
|---|---|
| A database holding **tens of thousands of keys** | The names are all held so a page can be cut from a sorted list; how that feels is not something a unit test can say |
| A key of **each of the six types**, one of them large | The value column is a different command per type, and one past `previewItems` has to be marked as the sample it is — and then refuse to be written back |
| **Editing a hash or a list in the grid** | The JSON the cell shows has to go back as the value it drew, and the key's expiry has to survive it |
| A key with **a colon-heavy name** and one holding a space | An argument is a run of adjacent tokens, and only a real key proves the run was rebuilt whole |
| `CONFIG GET`, `XRANGE`, `HGETALL`, `INFO` in a query tab | The four reply shapes: a map, a list of lists, a list, and one long string |
| A **database other than 0** | Every database has its own client, and a read of database 3 that answered from 0 would look like data |

`wails dev` serves the frontend *with* the Wails runtime and bindings on `http://localhost:34115`,
so `window.go.app.ConnectionAPI` is callable from an ordinary browser. This is the most effective
way to exercise the backend end-to-end without driving the native window. Use
`wails dev -browser=false` to suppress auto-opening a browser.

A plain `pnpm dev` (Vite alone) does **not** expose the bindings — the app renders but every
backend call fails.

## Generated bindings

`frontend/wailsjs/` is generated by Wails on every build and is committed, so a fresh clone
type-checks without first running a Go build. See [architecture.md](architecture.md) for why it is
excluded from `tsconfig.app.json`'s `include`.
