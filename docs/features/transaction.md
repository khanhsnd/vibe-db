# The manual transaction

[← Features and UI behaviour](../features.md)

Ordinarily every statement is true the moment it runs: a cell edited in the grid is written and
committed before the reader has taken their hand off the mouse. **A manual transaction turns that
off.** The reader says when it starts, does as much as they like, and then says whether any of it
happened.

It is the answer to the thing every other client is asked for first — *edit a hundred rows, look at
them, and put them all back if the shape is wrong*.

## Where the button is

**Beside the Cell Editor button, on the table tab's toolbar**, and beside `Format` on a query tab's.
Those are the two places a reader writes, so they are the two places the control belongs; it is one
component (`features/results/TransactionMenu.tsx`) in both, exactly as `CellEditorButton` and
`ExportMenu` are.

It is a menu of three, and only ever one of them is available:

| Item | Available when | What it does |
|---|---|---|
| `Begin Transaction` | nothing is open | Takes a connection out of the pool and starts a transaction on it |
| `Commit` | one is open | Makes everything done inside it true for everybody |
| `Rollback` | one is open | Throws all of it away |

The button says which state the connection is in, in words — `Transaction` against `Transaction ·
Open` — and is drawn pressed and in the warning colour while one is held. It says *connection* and
not *tab* deliberately; see below.

An engine with no such notion is offered nothing at all: the menu draws `null`, from
`EngineInfo.hasTransactions` and never from a comparison against an engine's name. See
[engines.md](engines.md#what-a-transaction-is-worth).

## It belongs to the connection, not to the tab

**A transaction is one connection, held open.** That is not an implementation detail, it is the
whole of what a transaction is: a row written inside one exists on that connection and nowhere else
until it commits. Everything else follows.

- **Every statement of the session runs inside it while it is open** — the table tab's reads, its
  cell edits, a query tab's statements, and a schema change from the tree. One rule, no exceptions,
  because a statement that quietly went round it would be a statement `Rollback` could not undo.
- **A grid reading that table shows the reader their own uncommitted rows**, which is the point. A
  read off a second connection would show them nothing had happened.
- **Two tabs open on one connection are inside one transaction**, and both buttons say so. They are
  reading the same store keyed by session id — `stores/transactions.ts`.
- **A second connection to the same server sees none of it** until it commits. That is the server's
  rule, not this app's.
- The pool is four connections, so the transaction holds one and the catalogue reads behind the tree
  still have three.

Committing or rolling back from a table tab **re-reads the page**, the rows on screen then being
about a truth that has moved on. A query tab's rows are the reader's to run again.

## What the engines do with it

The rule above is uniform; what the *server* does with a schema change inside a transaction is not,
and this app does not pretend otherwise.

- **PostgreSQL, SQL Server and SQLite** roll DDL back like anything else. A column added inside a
  transaction and rolled back is gone.
- **MySQL and Oracle commit implicitly** the moment a DDL statement runs. That is the engine's
  behaviour and it is what a reader of those engines already expects; nothing here can undo it, and
  inventing a guard against it would mean refusing statements the server would happily accept.
- **PostgreSQL aborts the whole transaction at the first failure inside it.** Every statement after
  that answers `current transaction is aborted` until the reader rolls back — the server's own
  wording, shown as it is.

## Failing

Two refusals are this app's own, and both are about the state rather than about a server:

| Code | When |
|---|---|
| `TRANSACTION_ACTIVE` | `Begin` while one is already open |
| `NO_TRANSACTION` | `Commit` or `Rollback` with none open |

Neither is reachable through the menu, which greys out whatever does not apply; they exist because
the state is real and a bound method that guessed would be worse than one that refuses. See
[errors.md](../errors.md).

**A transaction is over the moment it ends, however it ends.** A commit the server refused has still
ended it, so the connection goes back to the pool either way and the button stops offering
`Rollback` — the Go side is asked what is actually held rather than assumed, so a failure never
leaves the button disagreeing with the connection.

**Disconnecting rolls back whatever is still open.** A transaction nobody can reach is one the
server would hold every lock of until the connection died, so `Close` ends it before the pool goes.

## Where it lives

| Layer | What is there |
|---|---|
| `internal/driver/driver.go` | `driver.Transaction`, embedded in `Session` — four methods |
| `internal/driver/sqlkit/txn.go` | `sqlkit.Txn`: the whole of it for the five SQL engines |
| `internal/driver/sqlkit/{ddl,table}.go` | `DDL.exec` and `Data.queryer` — the two seams that ask the transaction before reaching for a pool |
| `internal/app/transaction_api.go` | Four bound methods, each answering with whether one is now open |
| `frontend/src/stores/transactions.ts` | What is open per session, and what the last call failed with |
| `frontend/src/features/results/TransactionMenu.tsx` | The button, in both toolbars |

Nothing in `sqlkit.Txn` spells a statement: `BEGIN`, `COMMIT` and `ROLLBACK` are `database/sql`'s to
send, which is why there is no `Dialect` flag here and no adapter override. What an adapter supplies
is the same `pinned` that a definition is run on — [rule 17](../../CLAUDE.md#rules-that-must-not-be-broken)
— and nothing else.
