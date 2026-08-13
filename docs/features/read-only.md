# Read-only connections

[← Features and UI behaviour](../features.md)

## What it is

**A connection says what it is allowed to do, and a read-only one refuses to send anything that
writes.** It is picked in the connection dialog beside the environment — `Access`: *Read & write*, or
*Read-only* — and it is the second fact about a profile that changes how the app *behaves* rather
than how it connects.

The two are counterparts, and the difference between them is the whole design:

| | What it does to a hazard |
|---|---|
| **Environment** (Production) | Turns it into a **question**: the statement runs once the reader has answered for it — see [connections.md](connections.md#what-gets-confirmed-before-it-runs) |
| **Access** (Read-only) | Turns it into a **refusal**: nothing is sent, and there is nothing to answer |

**It is a guard on this app and not on the server, and it says so.** Nothing reaches the database to
be rejected by it, which is exactly what protects a reader who ran the wrong tab against production —
and exactly what does not protect the database from anybody who points another client at it. **A
read-only account on the server is the protection**; this is the guard. The hint under the picker
says as much rather than leaving a reader to assume otherwise.

It is saved on the profile like any other field — `access` in `connections.json` — so a connection
opened before this existed reads back as read *and* write, which is what it always was.

## What is refused

The rule is `domain.Access.Refuses`, and it is read off the same `Hazard` the confirmation rule is —
so what a read-only connection refuses is what the editor was already underlining.

- **Every write, every schema change, every account change and every kill.** A `driver.Session` on a
  read-only profile is wrapped once, in `session.Manager.Open`, and the wrapper is what the whole app
  is then handed: the rows, the schema, the accounts, the monitor. There is no second seam.
- **A typed statement is decided by the engine's own reviewer.** `driver.Text.Review` reads the
  script, and one statement that would write refuses the **whole script** — a run is not half done.
  The reader is told which: `TRUNCATE was not sent to the server`.
- **`WITH` is refused**, even though the confirmation rule lets it by. `WITH gone AS (DELETE FROM …)
  SELECT …` is one statement whose verb is inside a parenthesis, and PostgreSQL and SQL Server both
  run it. This is where the two rules part company on purpose: **a confirmation defaults to allowing
  and a refusal defaults to refusing**, because being wrong costs the opposite thing in each — a
  question nobody needed is a nuisance, and a write nobody expected is the mistake the mode exists to
  prevent.
- **A routine call is refused.** Its body is a program this side has not read and cannot vouch for,
  so it is a write whatever it turns out to do.
- **A restore is refused**, both this app's own and the vendor's — `mongorestore`, `mysql`,
  `pg_restore` put rows and objects back. The matching *dump* only reads and is offered as usual.

## What is not refused

- **Every read**, however expensive.
- **Every preview.** `AlterStatements`, `SourceStatements`, `AccountStatements`, `ProcessStatements`,
  `CallStatements` and `CreateStatements` run nothing, so they all still answer. A reader who cannot
  apply a migration can still be shown the statements that would make it, which is most of what a
  read-only connection is *for* — see [schema-changes.md](schema-changes.md).
- **Every editor.** The designer, the definition editor and the account editor all open: opening one
  asks a server nothing. It is their **Save** that is refused, and it is refused with the same
  sentence as everything else.
- **A transaction.** `Begin`, `Commit` and `Rollback` are left alone — nothing inside one could
  write, and on the engines that offer it a transaction is how a reader holds a consistent view
  across several reads. See [transaction.md](transaction.md).

## What the reader sees

- **The status bar says `Read-only`**, beside the server it is about rather than out at the
  right-hand end with what the app is holding. A reader who cannot see the mode finds out by being
  refused, which is the wrong moment to learn it.
- **The tree offers no item it would refuse.** Every menu item that changes something on a server is
  **left out** — not greyed — for the reason a shut connection's items are: a column of dead items
  says only that. A submenu left holding nothing goes with them. What decides it is one flag,
  `MenuItem.writes`, read in one place (`withoutWrites` in `features/connections/useTreeMenu.ts`).
- **A grid on a read-only connection does not edit.** No cell opens, no row is added or deleted —
  a cell that took what was typed and then threw it away would be worse than one that never opened.
  See [../grid.md](../grid.md).
- **Anything that slips through is answered plainly**: `READ_ONLY`, with the statement or the change
  it held back — see [errors.md](../errors.md). The frontend never re-derives what is refused; the
  session says `readOnly` and the Go side says what it would not send.

## Failing

| What | What is said |
|---|---|
| A script holding a write | `This connection is read-only. DELETE was not sent to the server.` |
| A change asked for anyway | `This connection is read-only. dropping a column was not sent to the server.` |
| An unknown access mode in a saved profile | `The access field is not a known access mode.` — refused where every other invalid field is, in the dialog |

## Where the rule lives

- `domain.Access` — the two modes and `Refuses`. **The rule, and the only copy of it.**
- `session.readOnly` — the wrapper, and the only place that refuses. A method added to
  `driver.Session` and left unclassified fails `readonly_test.go`, which walks the interface and asks
  of every method whether it is refused.
- `app.Session.readOnly` — what the frontend draws itself from.
