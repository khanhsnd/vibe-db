# Automation

A **job on a clock**: back this namespace up every night at two, export it to CSV every hour, run
mysqldump into a folder every Sunday, or watch two tables and say when they have drifted apart.

It is opened from **`Tools ▸ Automation`**, and from nowhere else. A job belongs to the *window*
rather than to a connection — it names the connection it dials, and the list holds jobs on all of
them, including ones nobody has opened today — so there is no node in the tree it could hang off.
That is the arrangement the [History Log](log.md) has, and for the same reason.

## What a job does when it fires

Four kinds, and **every one of them is a bound method a button in this app already calls**:

| Kind | What fires | Where it goes |
|---|---|---|
| `Backup` | `RunBackup` — the same call [`New Backup`](backup.md) makes | The folder this app keeps, one file per firing |
| `Export` | `RunExport` — the same call the [Export wizard](transfer.md) makes | The folder the job names, one file per table |
| `Dump` | `RunToolDump` — the same call the [engine's own dump program](tools.md) is run by | The folder the job names, one file per firing |
| `Compare` | `CompareStructure` and `CompareData` — the same calls a [comparison tab](compare.md) makes | Nowhere; it *reports* |

There is no fifth way to take a backup. A job that fired at two in the morning wrote exactly the file
the reader would have got by pressing the button themselves, which is what makes the schedule worth
trusting — see [rule 27](../../CLAUDE.md#rules-that-must-not-be-broken).

**Nothing is put back on a schedule.** A restore and an import are not among the four: a job that
ran while nobody was looking is the wrong place to find out that a table was overwritten. Every one
of the four only ever *reads* a server, which is also why a [read-only](read-only.md) connection can
carry all of them.

## The clock

**A job fires only while vibe-db is open.** Nothing is installed on this machine, no service is left
behind, and the app is not on the machine's hook for firing anything after it closes. The list says
so, in the strip above it, because it is the one thing about a schedule a reader must not assume.

The clock is looked at every half minute. A cadence is read in **this machine's own time zone** —
`02:00` means two in the morning where the reader is.

| Repeat | Answers |
|---|---|
| `Every N minutes` | How many minutes apart |
| `Hourly` | Which minute past the hour |
| `Daily` | Which hour and minute |
| `Weekly` | Which day, and which hour and minute |

**A saved job counts from the moment it was saved.** Editing a daily two o'clock job at noon does not
fire it at noon: saving *arms* it, and the next firing is one whole cadence later. Turning a job back
on arms it too, so a job switched off for a week does not fire seven times when it comes back.

### A moment that went by while the app was shut

The reader's answer, and not a default this app picks — both readings are honest. **Run a firing that
was missed** ticked, a job whose moment passed while the app was closed runs the next time it opens;
left clear, it steps past that moment and waits for the next one. A nightly backup usually wants the
first; an export into a folder somebody is watching usually wants the second.

It is decided **once, when the app opens**, so a laptop opened twice does not fire a job twice.

### Two jobs at once

Each firing runs on its own, so a backup that takes two hours is not two hours in which nothing else
can fire. What is *not* allowed to overlap is **one job with itself**: a job still going when its
next moment arrives is passed over, and `Run Now` on one that is already running is refused with
`TASK_RUNNING` rather than started twice — the two runs would write the same file.

## The list

Every job, in a [`ResultGrid`](../grid.md), read-only:

**Name**, **Job**, **Status**, **Schedule**, **Connection**, **Target**, **Next Run**, **Last Run**,
**Last Result**. `Status` is `On`, `Off`, or `Running` — the third winning over both, being the only
one of the three that will not be true in a minute. A disabled job shows **no** next run rather than
a date it will not keep.

The list **follows the clock**: a firing pushes an event as it starts and again as it ends, so a
reader watching at two in the morning sees the job go rather than a row that changed while they were
not looking.

Below it is the picked job's history — its last 20 firings, newest first: **Ran At**, **Result**,
**Elapsed**, **Summary**, **Detail**. Only a handful are kept, because the question a reader brings
here is *did last night's run work* — and every statement any of these jobs sent is in the
[History Log](log.md) already, on the same clock as everything else.

### The toolbar

| | |
|---|---|
| `New Job` | Opens the dialog on a nightly backup of wherever the reader already is. |
| `Edit` | The same dialog on the picked job. A double-click on a row does the same. |
| `Run Now` | **Fires it immediately, exactly as the clock would** — so a nightly backup set up this afternoon is one the reader watches work this afternoon. It counts as a firing: it is written into the history and the job's next moment is counted from it. |
| `Enable` / `Disable` | Turns the picked job on or off, arming it as it goes. |
| `Delete` | Asked for first. The job stops firing and **nothing it ever wrote is touched** — the backups it took are still in the backup folder, and the files it exported are still where they landed. |
| `Refresh` | Re-reads the file. |

## Filling a job in

Three tabs, and **not a wizard**: every tab is already answered, so a reader who wants a nightly
backup of everything presses `Save` on the screen it opened on. That is the shape
[`New Backup`](backup.md#the-dialog) has, and for the same reason.

**General** — the name, which of the four kinds, the connection, and the database and schema. Each
name is a list to pick from where the connection is open and a box to type into where it is not: a
job may perfectly well be written down for a connection nobody has opened today. It is also where
`Enabled` and the missed-firing answer live.

**Schedule** — the cadence above, said back in a sentence under the boxes.

**What It Does** — the only tab where the four differ, and it asks nothing the kind does not read.
A job switched from an export to a backup and back finds the folder it had still there.

| Kind | Asks |
|---|---|
| `Backup` | Structure and records, how many backups to **keep**, a comment for the file's header, and which tables |
| `Export` | The format, the folder, rows per table, and which tables |
| `Dump` | The folder, structure and records, whether to compress, and which tables |
| `Compare` | This table, the whole of the far side — its own connection, database, schema and table — whether to compare structures, records or both, and how many rows a side |

**Which tables is empty by default, and empty means every table the namespace holds** — read at the
moment the job fires, so a table added next week is in next week's backup without anybody editing the
job.

## What each kind writes

**A backup is one file per firing**, named for the moment it was taken, in the folder
[backups](backup.md) already live in. `Keep` is the one thing a schedule does that no dialog does: it
deletes the oldest until only that many are left in **this namespace's folder**, and `0` keeps every
one. It counts the folder rather than this job's own files, because a backup carries the moment it
was taken and not the schedule that took it. A comment left empty becomes `Scheduled · <the job's
name>`, so a backup found months later says which schedule wrote it.

**A dump is one file per firing** too — `shop-public-20260813020000.sql`, the namespace and then the
moment — so two firings never write over each other. It runs the engine's own program, which has to
be on this machine; a connection through an SSH tunnel or a cloud identity cannot be dumped this way
and the run says so rather than running against the wrong host.

**An export overwrites.** A table's file is named after the table, so a nightly export is a *mirror*
of the namespace rather than a pile of dated copies — the export wizard's own behaviour, unchanged
for having been asked for by a clock. A reader who wants yesterday's kept points a second job at a
second folder, or schedules a backup, which is the kind that snapshots. The file is written with the
wizard's own defaults: UTF-8, comma-separated, with a header row. A job needing anything else is an
export run by hand.

**A comparison writes nothing at all.** It reads both tables and reports.

## What a firing comes to

Three outcomes, and the third is the reason there are not two:

| Outcome | Means |
|---|---|
| `OK` | It did what it was asked. The summary says what it wrote. |
| `Failed` | It could not. The detail is the failure in **the same words the reader would have been shown** had they pressed the button themselves — the server's own for a rejected statement, the program's own for a dump. |
| `Differs` | A comparison found the two sides apart. **Not a failure**: it is the answer the job was asked for, and drawing it beside a server that was unreachable would be the wrong sentence. |

A backup that was written but whose folder could not be pruned is `OK` with the pruning said in the
detail: the backup is what the reader came for.

## How a job connects

It dials the profile it names, does the work, and shuts the connection. **A session the reader has
open is not borrowed** — a job that fired while they were half way through a
[manual transaction](transaction.md) on that connection would run inside it.

**Nothing is typed, because nobody is there.** The password is whatever the credential store holds,
or a token minted for that connection where the profile signs in by a cloud identity. A profile that
keeps neither is dialled with no password rather than refused — a server that takes none is ordinary
— and the dialog says so where the profile has no saved password, because a run that fails at two in
the morning is an expensive way to find out.

## Failing

| | |
|---|---|
| `TASK_RUNNING` | `Run Now` on a job that is already going — see above. |
| `TASK_NOT_FOUND` | The job is no longer in the file. `Refresh` is the answer. |
| Anything else | Not thrown at all. A job that fails while it runs is an **outcome**, kept in its own history, because there is nobody standing in front of a dialog to throw it to. |

## Where it is kept

One JSON document, `<config>/vibe-db/schedules.json`, read whole and written whole — the arrangement
the [connection profiles](connections.md) and the [snippet library](snippets.md) have. It holds the
jobs and their last few firings, and **no secret**: a job names a profile, and the profile's password
is the credential store's.
