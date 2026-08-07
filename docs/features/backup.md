# Backups

A backup is **a file of the statements that would put a namespace back**, kept in a folder this app
owns rather than one the reader picks. That is the whole difference between it and a
[dump](transfer.md#dumping-a-table-or-a-schema): a dump is carried somewhere, and a backup is found
again by being where backups are.

It is offered on **the five SQL engines**. A backup here is SQL, so the engines that have one are the
engines whose statements declare a table and write a row; MongoDB and Redis get no Backups node at
all rather than a node that could only refuse — see `hasBackups`, and
[architecture.md](../architecture.md#backing-a-database-up) for where that rule lives.

It is also not [the engine's own dump program](tools.md). Those write the *vendor's* format to a
file the reader names and have to be installed first; this writes statements this app spelled, into
a folder the app keeps. MongoDB has the second without the first, which is exactly the difference.

## Where a backup is reached

Three places, all naming the same namespace:

- **`Backups`** in the tree, under `Functions` in every database or schema that owns tables.
- **`Backup`** on the command bar, beside `New Query`, which backs up wherever the reader already is
  — the selected row's namespace, falling back to whichever connection is open. It is dead where
  there is nothing to back up: no live connection, or an engine that has none.
- **`New Backup`** on the list's own toolbar.

## The branch

`Backups` is a folder like `Tables` is, and **a press opens it two ways at once**: the branch
expands onto the names, and the list opens in a tab beside it. The tree has room for the names and
the grid has room for the date, the size and the comment, and a reader reaching for a backup takes
whichever of the two is nearer. Pressing the caret expands without opening a second tab.

It is filled by **a read of its own**, not by the catalogue read the other three folders share: a
folder on this machine is not something a server was ever asked about, which is also why it has a
`Refresh` of its own.

Right-clicking the folder offers **New Backup…**, **Backup List** and **Open Containing Folder** —
the last showing the folder in Explorer, Finder or whatever the desktop opens a directory with.
Restore is not there: a restore is addressed to *one* file, and the file is what carries it.

Right-clicking **one backup** offers the whole of what there is to do with it, and the same actions
the list's toolbar runs:

| | |
|---|---|
| `Restore Backup…` | Runs its statements back at the server, or the tables picked out of it. Asked for first — see [what a restore does](#what-a-restore-does). |
| `Extract SQL…` | Writes a copy wherever the reader points. |
| `Copy <name>` | A second file beside it, named `<name>-copy`. The name is not asked for: the original's is a *moment*, and a copy of it has no new one. |
| `Rename…` | Only the file moves. A name already taken is refused rather than written over. |
| `Delete Backup` | Removes the file. Asked for first, and nothing brings it back. |
| `Open Containing Folder` | As the folder's. |

## The list

Every backup kept for the namespace, newest first, in a [`ResultGrid`](../grid.md) of four columns:
**Name**, **Modified Date**, **File Size**, **Comment**. It is read-only; the toolbar over it acts
on whichever row the selection names, and every button on it is one of the branch's own items above
— built from the same `backupActions`, so what a confirmation says and what a change re-reads cannot
differ between the two surfaces. **Extract SQL** is a copy and not a conversion: the backup is
already SQL, and the only thing this adds is a name and a place the reader chose.

**There is one list, and both surfaces draw it.** The files live in `stores/connections` beside the
tree's other names, so a backup deleted from the branch leaves the grid in the same breath — two
copies of one folder would be two things to keep in step, and the one thing worse than a stale list
is two lists disagreeing about which files exist.

The tab **re-reads on every visit**, which the branch does not: what is in the folder can change
without this app doing it — another window of it, a shared drive, a file dropped in by hand — and a
directory listing on this machine is cheap enough to pay for on a tab switch. A file with no header
of ours still lists, with an empty comment, because it still restores. A folder that could not be
read at all lists as empty with the reason in the status bar.

## New Backup

Four tabs, and **not a wizard**. The two transfer wizards ask one question a screen because their
answers depend on each other; none of these do. Every tab is already answered, so a reader who wants
the default presses `Backup` on the screen it opened on, and reaching the third answer never means
passing the second.

| Tab | |
|---|---|
| **General** | The server and the namespace, which are facts and not boxes, and the **Comment** — free text kept in the file itself. |
| **Object Selection** | Which tables the backup covers, all ticked. What is remembered is what was *unticked*, so a table created after the dialog opened is still included. |
| **Advanced** | **Include structure** and **Include records**, both on. Both together is a whole backup; the structure alone restores onto an empty server, and the records alone onto one that already has the tables. Turning off both is refused rather than written. |
| **Message Log** | What the run did, line by line. Switched to when `Backup` is pressed, so the reader is looking at what is happening. |

There is no `Save Profile`. A saved set of answers is a second thing to keep, and every answer here
already survives being retyped in four clicks.

## What a backup holds, and what it does not

The tables of the one namespace, and nothing else — no views, no functions, no query history. Each
table's declaration, then its rows, exactly as a dump writes them: the rows are **streamed**, so a
table of a million rows costs one batch of memory rather than a table's worth.

**The file says which table each part of it is.** A comment line above each one names it, and the
header names them all — so a restore can be narrowed without anything reading a statement to work
out what it touches, and without the file being scanned to find out what is in it. Both lines are
SQL comments, so a backup carrying them is still an ordinary script: `psql` restoring one has no
idea they are there. A dump gets none — it is carried somewhere and run whole, and there is nothing
at the far end to narrow it with.

A table the server refuses **stops the backup, and the half-written file is removed**. A backup
missing a table is the one kind of file whose existence is a lie worth avoiding — it would sit in
the list looking exactly like a whole one, and be found out only by a restore.

## What a restore does

Runs the file, in order, through the same reader and the same `Split` a picked script goes through.
Nothing about it knows what a backup is.

**One screen, and not a wizard.** A restore has two answers — which file, and which of its tables —
and the first was given by the row it was raised on, so there is nothing to page through: the tables
are listed on the same screen as the warning, and `Restore` is under both. The log fills in below
them as it runs, in the same lines the Message Log shows.

The list is **read off the file's own header**, which the backup listing already read — so the
choice costs no read of the file and asks no server anything. All of them are ticked, and what the
reader unticks is dropped from the text before a statement is split out of it. A file that names no
tables — one written before there were any, or dropped into the folder by hand — offers no narrowing
and restores whole, which is what it did before there was a line to read.

A narrowed restore is still exactly the statements a whole one would have run for those tables:
nothing is re-spelled, and what is left after the narrowing is still a script.

A rejected statement **stops it, and there is no transaction around it**: a restore's statements
depend on each other, and a table that failed to be created has nothing for the thousand inserts
behind it to land in. What has already run stays run, and the confirmation says so before it starts.

A restore **adds to what is there** rather than replacing it — its `CREATE TABLE` will be refused by
a server that already has the table, and its records land beside whatever the table already holds.
Restoring onto a namespace that has moved on is emptying or dropping first, which the tree already
offers.

## Where the files are

`<config>/vibe-db/backups/<connection id>/<database>/` — and `<database>@<schema>` where the engine
has schemas, so two schemas of one database keep their backups apart. The connection's **id** names
the folder rather than its name, so renaming a connection does not lose its backups.

The name is the moment it was taken, to the second: `20260429110321`. A second backup inside one
second is given a suffix rather than written over the first, and so is a copy — **nothing here ever
writes over a backup**, which is why Rename refuses a name already taken instead of taking it.
