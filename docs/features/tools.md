# The engine's own tools

Three engines ship a pair of command-line programs that carry a whole database out to a file and put
it back, and this is the app running them: **mongodump / mongorestore**, **mysqldump / mysql**,
**pg_dump / pg_restore**. They are the engine vendor's, not this app's, and they are not on the
machine unless the reader put them there.

That is the whole difference between this and a [backup](backup.md), which is a file of statements
*this app spelled*, kept on a shelf the app owns. A MongoDB collection is the case that proves the
difference is worth having: a script of ours could carry only the fields
[a sample of two hundred documents](../grid.md) happened to hold, and mongodump carries the BSON —
every field of every document, the indexes and the collection options with it.

| | A backup | An engine tool |
|---|---|---|
| What is in the file | Statements this app spelled | The vendor's own format |
| Where the file goes | A folder the app owns, found again by being there | Wherever the reader points |
| What puts it back | Running the file | The vendor's other program |
| Which engines | The five SQL engines | MySQL, PostgreSQL, MongoDB |
| Needs anything installed | No | Yes, and says so |

A dump is also one of the four jobs a [clock](automation.md) can fire, which names its file for the
moment it was taken so that two firings never write over each other.

## Where they are reached

Right-clicking a **database**, a **schema**, or the **Tables** folder, below the two transfer
wizards: `mongodump…` and `mongorestore…` on MongoDB, `mysqldump…` and `mysql…` on MySQL,
`pg_dump…` and `pg_restore…` on PostgreSQL.

**The items are named after the programs**, because that is what the reader is looking for: somebody
who has installed MongoDB Database Tools knows the word `mongodump` and does not know what this app
would otherwise have called it.

Both are drawn whether or not the program is on this machine. An item nobody can see is a feature
nobody finds, and a missing program is something the dialog says and answers — see below.

An engine that ships no such pair has neither item, and that is `dumpTool` in
`frontend/src/bridge/connections.ts` mirroring `domain.Engine.Toolkit`, never a comparison against
an engine's name. Three engines have none, each for its own reason: **SQLite** is one file, so
copying it *is* the dump; **SQL Server**'s backup is a statement the *server* runs, writing to a
path the server can see; **Oracle**'s Data Pump runs inside the instance. Redis persists itself.

## Finding the program

Looked for on the `PATH`, and at **the file the reader pointed at** — `Change…` beside either
program on the dialog's first tab, remembered per *program* in `<config>/vibe-db/tools.json`.

**The file and not the folder it is in.** A folder cannot say which of the things in it was meant,
and a reader who has built one from source, or who keeps two versions of the MongoDB tools, is
pointing at *this* mongodump.

**One program to a dialog, and choosing it sets that one alone.** The dump dialog shows mongodump
and the restore dialog shows mongorestore; neither fills the other in, even when the other is
sitting in the same directory. Filling it in would be this app deciding which mongorestore was meant
on the strength of it being next door — and a path nobody chose is a path nobody thinks to check
when the wrong version runs.

`Change…` is **always** there, whether or not the program was found: a path that was right last
month is exactly the thing that needs changing today. `Forget` drops a chosen path and falls back to
the `PATH`, which is the way back from having pointed at the wrong copy.

A program is looked for **every time the dialog opens** rather than remembered, so a reader who
installs the tools while it is up presses `Change…` and carries on rather than restarting the app.

Three states, and each reads differently:

| | |
|---|---|
| Found on the `PATH` | The path, and nothing to undo |
| Found because it was picked | The path, with `Forget` beside it |
| **Picked, and no longer there** | The file that has gone, named — never a quiet fall back to a different copy on the `PATH` |

## The dialog

Four tabs and **not a wizard**, the shape [New Backup](backup.md#new-backup) already uses and for the
same reason: every tab is answered before it is opened.

| Tab | |
|---|---|
| **General** | The server, the namespace, **the program this direction runs and where it is**, and the file — typed or picked. |
| **Object Selection** | Which tables the run covers, all ticked. |
| **Advanced** | The options **this pair actually has** — see below. |
| **Message Log** | What the program printed, line by line, as it printed it. |

**A box that is not on Advanced is a thing the program has not got**, never a box that does nothing:

| | mysqldump / mysql | pg_dump / pg_restore | mongodump / mongorestore |
|---|---|---|---|
| Structure only / records only | ✓ | ✓ | — a collection declares nothing |
| Compress as written | — | already is | ✓ |
| Drop what is there first | — the client only runs a script | ✓ `--clean --if-exists` | ✓ `--drop` |
| Dry run | — | — | ✓ |
| Stop at the first refusal | ✓ | ✓ | ✓ |

## What the log holds

**The command line, first.** It is the whole truth about what ran, and it can be shown at all only
because no secret is ever an argument — see below. What is missing from it is exactly the credential
file, which is written for the run and gone before the log is read.

Then **the program's own lines**, unchanged. mongorestore knows why it refused a document and this
app does not. They are pushed as they are printed rather than gathered at the end, because a dump of
a hundred gigabytes says nothing for an hour otherwise, and that is the run a reader stands and
watches. Past five hundred lines the log says it has stopped listening — the file itself is
unaffected — rather than growing to something nobody can scroll.

`Stop` kills the program. What it had already written stays written.

## The rules that do not bend

**No secret is ever an argument.** A command line is readable by every process on the machine, so
the password travels in a file this process wrote at `0600` in the temporary directory and removed
the moment the run ended — `--defaults-file` for MySQL, `PGPASSFILE` for PostgreSQL, `--config` for
MongoDB. All three programs are told **never to prompt**: a child process asking for a password on a
terminal nobody is looking at is a dump that hangs for ever.

**The bytes never pass through this app.** Each program is told the path and writes it itself, so a
dump of a hundred gigabytes costs one log's worth of memory. Nothing here reads a byte of what was
written either — which is the point of running them.

**The password is the one the connection saved.** These programs make a connection of their own, and
there is nowhere for a prompt to go, so a profile that does not keep its password is run without
one. Two profiles are refused outright rather than run wrongly:

- **A connection over an SSH tunnel.** The tunnel is an in-process dialer and
  [listens on nothing](../architecture.md), so there is no address a child process could be given.
  Running the program against the far side's host name would reach whatever is at that name *here*.
- **A connection signing in with a cloud identity.** The token is minted for one connection at the
  moment it is made, and nothing here makes one.

Both answer [`UNSUPPORTED`](../errors.md) naming which.

## Two things worth knowing about the files

**A MongoDB archive carries the namespace it came from.** mongorestore is not given a `--db`, so the
documents go back where they came from however the tree is pointed when the restore is run.

**Whether a MongoDB archive is compressed is read off the file, not off a box.** `--gzip` has to
match how the dump was written or mongorestore refuses the first byte, and a reader restoring a file
from last month has no reason to remember which it was — so the first two bytes are looked at, which
is the one place that answer is written down.

**A PostgreSQL dump is the custom format**, not plain SQL, and that is what makes the pair worth
having: one file, compressed as it is written, out of which pg_restore can put back the whole
database, one schema or one table. It is not a script and `psql` will not run it.

## Object Selection, and what it can honestly claim

On a **dump** it is exactly what it says: leaving them all ticked covers whatever the namespace holds
at the moment it runs. Two of the three programs are told what to *take* — `mysqldump db t1 t2`,
`pg_dump --table` — and mongodump is told what to **leave out**, `--excludeCollection`, that being
the only narrowing it has. Which is also what the picker already remembers: the unticked list.

On a **restore** the list is the names the *server* holds now, and the file may hold a name that is
not among them. All ticked is therefore the only setting that cannot leave something behind, and it
is the one it opens on; unticking narrows the restore to the names left ticked. The dialog says so
under the list rather than letting a reader find out afterwards.
