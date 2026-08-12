# Users and privileges

[← Features and UI behaviour](../features.md)

## What an account is

Every server this app talks to keeps a list of who may connect to it and what each of them may
touch. The `Users` folder under a connection opens onto that list, and the editor below it is where
one is made, changed and dropped.

**It hangs off the connection and not off a database**, which is the only honest place for it: an
account belongs to the server, and the tree's other levels are all namespaces. It is the one folder
in the tree that does.

**What one is called is the engine's word.** Five of the seven say `User`; PostgreSQL says `Role`,
because `CREATE USER` there is `CREATE ROLE … LOGIN` and nothing else — there is no second kind of
object to make. That is `EngineInfo.accountLabel`, and no component compares against an engine's
name to decide it.

**A name is not always enough**, and the two fields beside it are each one engine's way of saying
which one:

| Engine | What names an account | Why |
|---|---|---|
| MySQL | `name` **and** `host` | An account *is* the pair. `'app'@'10.%'` and `'app'@'localhost'` are two accounts, with two passwords and two sets of grants |
| SQL Server | `name` **and** `database` | A **login** belongs to the server and a **user** to a database. A login with no user in a database can sign in and reach nothing inside it |
| MongoDB | `name` **and** `database` | A user is created *in* a database and is identified by the pair |
| PostgreSQL | `name` | A role is the cluster's. `database` says which database's grants were read — see below |
| Oracle, Redis | `name` | One name, one account |

## What each engine has

| | MySQL | PostgreSQL | SQL Server | Oracle | MongoDB | Redis | SQLite |
|---|---|---|---|---|---|---|---|
| List, make, alter, drop | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| Rename | ✓ | ✓ | ✓ | | | | |
| Grant a privilege | ✓ | ✓ | ✓ | ✓ | | ✓ | |
| Grant a role | ✓ | ✓ | ✓ | ✓ | ✓ | | |
| `DENY` | | | ✓ | | | | |
| Column-level grants | ✓ | ✓ | ✓ | ✓ | | | |
| The server's own words | `SHOW GRANTS` | the `aclitem`s | | `DBMS_METADATA` | the user document | the `ACL LIST` line | |

A blank is a thing the engine has not got, and asking for it answers
[`UNSUPPORTED`](../errors.md) naming which — never a statement left for the server to refuse.

Four of those blanks are worth the sentence:

- **SQLite has no accounts at all.** A SQLite database is a file: whoever can open it can do
  everything to it, and whoever cannot can do nothing. That is the operating system's answer, not
  the engine's, so there is no `Users` folder under a SQLite connection and nothing to open.
- **Oracle cannot rename one.** A user there *is* a schema, and its name is on every object inside
  it; there is no statement that moves them. Making one adds a row to the namespace list the tree
  draws, and dropping one carries `CASCADE`, without which the statement is refused for any user
  that owns anything.
- **MongoDB grants roles and never privileges.** A privilege there belongs to a *role*
  (`{resource, actions}`), and a role is what is granted to a user. So the Privileges tab is not
  drawn at all and everything happens on Roles; a role is named `role@database`, because `read` on
  `logs` and `read` on `shop` are two different grants of one word.
- **Redis has no roles.** An ACL grants **command categories** — `+@read`, `+get` — and there is
  nothing that is granted in turn. What a rule *narrows* (`~cached:*`, `&news.*`) is an attribute
  rather than a grant, because that is what it is: a bound on the whole account and not something
  granted on a key.

## Reading one

Picking a row reads the account whole: its attributes, what it has been granted, the roles it holds,
and **the server's own words about it** — which is a read for the reason [a definition
is](definitions.md): an account put back together out of four catalogue tables is only ever as
complete as their columns, and a reader auditing one wants the server's answer.

The list itself is a [`ResultGrid`](../grid.md) in whatever columns that engine's catalogue keeps,
exactly as [the table list](schema-tree.md) is, and for the same reason: no two engines keep the
same facts about an account, and a column neither could fill would say nothing.

**A role is cluster-wide and its grants are a database's**, on PostgreSQL. `pg_roles` is the same
list whichever database you ask from, but a table grant lives inside the database that holds the
table — so the reference carries which database's grants were read, and the picker moving it is
reading something real rather than decorating a list. It is the same fact that already gives that
adapter [a pool per database](engines.md#how-deep-the-tree-goes).

**Reading who a server knows is privileged on every one of these engines.** An account without that
privilege meets an empty folder and a sentence in the bar, never a broken tree: MySQL falls back
from `mysql.user` to the grant tables, Oracle from the `dba_` views to the `user_` and `all_` ones,
and MongoDB from every database to its own.

## Changing one

Four panes, and the fourth is the point of the first three.

- **General** is the name, the host or database where the engine has one, the password, and whatever
  attributes that engine's account carries. The attributes are the **driver's** — MySQL's account
  lock and connection cap, PostgreSQL's `SUPERUSER` / `CREATEDB` / `VALID UNTIL`, Oracle's
  tablespaces and profile, SQL Server's default database and schema, Redis's key and channel
  patterns — and each is drawn as the kind of box the driver said it was. **Not one clause travels
  with them**: what an attribute is *spelled* as is a flag on `sqlkit.Dialect`.
- **Privileges** is a grid: one row per privilege the chosen level offers, and two ticks — whether
  it is granted, and whether it may be passed on. The **names are the engine's own** and are never
  translated, and they are a *read* wherever the server will answer for itself: MySQL's
  `SHOW PRIVILEGES`, SQL Server's `sys.fn_builtin_permissions`, Oracle's `system_privilege_map`,
  Redis's `ACL CAT`. PostgreSQL's are a fixed list, that engine enumerating them nowhere a query
  can reach.
- **Roles** is one row per role the server offers and one tick: whether the account is a member.
- **Changes** is exactly what Apply will run, in order.

**A tab for a thing the engine has not got is not drawn.** MongoDB has no Privileges tab and Redis
no Roles tab, because both would be tabs of empty boxes.

**Nothing is matched by position.** A grant is matched by what it is *on* and what it is — its
level, its namespace, its object, its columns and its privilege — for the reason [a comparison
matches a field by its name](compare.md): a privilege granted in the middle of a list would
otherwise report every one below it as changed. A draft is therefore what **moved**, never the whole
of what is there, so applying it cannot revoke and re-grant something the reader never touched.

## The password, and why the preview is not quite the statement

Everywhere else in this app, [the statement a preview shows is the statement it
runs](schema-changes.md). Here there is exactly one exception, and it is deliberate:

> **A password is never spelled into a preview.** The Changes pane shows `IDENTIFIED BY <password>`,
> and the literal goes in only on the way to the server.

The Changes pane is what goes on the screen, into the [History Log](log.md) and into a saved
[history](results.md#the-history-pane). A password in any of the three is a password sitting in a
file on this machine, in plain text, for as long as the log is kept. It is the same arrangement
[the vendor's own programs](tools.md) already make — a password in a file at `0600` rather than on a
command line, which is what lets the log open with the command line itself.

**It is still one speller.** `sqlkit.Auth` spells the preview and the run from the same function,
which takes the password as an argument; the placeholder is a single boolean, not a second
rendering. Everything else about the two is the same statement, character for character.

The placeholder is `<password>` and deliberately not a row of asterisks: asterisks look like a
value, and a reader who copied the statement out of the preview would send them to the server as
one.

## Applying it

Apply runs the statements in the order the pane shows them, **stopping at the first failure**. There
is no transaction around it unless the reader opened one — and if they have, every statement here
goes inside it like every other statement of the session, so a `Rollback` takes the grants back with
everything else. See [transaction.md](transaction.md).

**A change this engine has no statement for is refused before anything runs**, not halfway through:
a rename Oracle cannot make would otherwise land after the grants in front of it had already gone
through.

### What has to be answered for

Three things are confirmed before they run, and the middle one is why this exists at all:

| What | Class | Confirmed |
|---|---|---|
| Dropping an account | destructive | on a [guarded environment](connections.md#the-environment-guard) |
| A grant that names **nothing** — the whole server, `ALL PRIVILEGES`, or `WITH GRANT OPTION` | unfiltered | **everywhere**, laptop included |
| Everything else, a revoke included | write | on a guarded environment |

A grant that names nothing is unfiltered in exactly the sense `DELETE` with no `WHERE` is: it names
no object and so names all of them, and an account that may grant may grant itself. That is
`domain.GrantHazard`, the fifth grammar for one rule beside the ones a statement, an import, a shell
command and a Redis command are read by — and the frontend **receives its answer** rather than
deriving one.
