# The row limit

[← Features and UI behaviour](../features.md)

There are two row-count controls, and they are shaped by what they size rather than by each other.
The **query toolbar's** is `shared/ui/RowLimit`: a dropdown of presets plus `All` and `Custom…`,
where `Custom…` reveals a number input beside it for any count. Picking `Custom…` seeds that input
with the count already in force and applies it at once, so the number on screen is always the one
that will run; typing commits per keystroke, and clearing the box keeps the last valid count rather
than fighting the typist.

The **grid pager's** is the gear at the end of the pager, and it is a *checkbox* plus a count —
`Limit records`, then how many `records per page`. Lifting the bound is a different kind of read
rather than a bigger one, and a checkbox says that where one more entry in a list of sizes would
have hidden it. The count survives the bound being lifted, so turning it back on restores what it
was; it commits per keystroke for the same reason the dropdown's does.

| Control | Sizes | Default | What it sizes |
|---|---|---|---|
| Query toolbar `Limit` | 100, 1 000, 10 000, or typed | 1 000 | The rows one **Execute** may return — the first prefix, not a ceiling, since the pager can replay for more. Hitting it is reported as a `+` on the status line's total rather than passed off as the whole answer. It sizes the *next* Execute, so a pager that grew past it leaves it alone. |
| Grid pager gear | any typed count | 100 | One page of the grid. Clearing `Limit records` collapses the walk to a single page, so the pager sizes itself by the row total and the arrows go dead — and on a query result it first fetches every row, because otherwise it would only mean all of the prefix. On a **table** tab that read is streamed and watched, below. |

Lifting the bound is deliberately the *only* way to fetch unbounded, so the bounded defaults of
[decisions.md](../decisions.md) cannot be lost by accident. It travels the wire as the sentinel
`app.AllRows` (`-1`) from both controls, and `driverRows` translates it into the driver's own
convention of a non-positive limit meaning "read everything". A missing, zero or otherwise
nonsensical count still falls back to `app.DefaultQueryRows` / `app.DefaultPageSize` — the Go zero
value can never mean unbounded.

Two consequences of that convention are worth knowing:

- `buildSelect` in either adapter **drops the `LIMIT … OFFSET …` clause** for an unbounded page,
  which makes its offset moot. `MaxRows` needs no equivalent, since `sqlkit.ScanRows` already reads
  to the end on a non-positive cap. It **binds nothing at all**: a limit and an offset are numbers
  this side counts rather than anything a reader typed, so they are written into the statement, and
  one string is then both what runs and what the status line shows — the only arrangement in which
  the two cannot drift apart.
- There is no ceiling on a typed count in either control. One would be theatre next to an unbounded
  read, and silently answering a different question than the one asked is worse than a slow grid.

## An unbounded read on a table tab is streamed, watched and stoppable

A table of a million and a half rows broke both halves of the first attempt at this: the read outran
the 15-second deadline every other read is given, and the answer would have had to cross the bridge
as one JavaScript string, a bound method's result being delivered to the WebView as exactly that. So
a table read that asked for every row and minted a **run id** behaves differently from every other
read:

- **It has no deadline.** How long every row of a table takes is not knowable ahead of time, and a
  clock that cuts it off is the very failure an unbounded read exists to avoid. `StopTableRows`
  bounds it instead.
- **Its rows arrive in batches.** `LoadTableRows` answers with the columns and the total alone and
  emits the rows as `app.EventRows` — `rows:batch`, `sqlkit.BatchRows` (5 000) rows at a time.
  `useTableData` stitches them back together and shows the grid once the call resolves. It is still
  **one statement and one scan**: the batches are how the answer is delivered, not a walk of
  `LIMIT … OFFSET …` pages, so nothing is counted twice and no row is skipped by a write landing
  mid-read.
- **Its count arrives separately, on a clock.** `app.EventRowsCounted` — `rows:counted`, every
  `sqlkit.CountEvery` (50 ms) — and that separation is the whole reason there are two events. A batch
  fills on a round number, so a count handed over with one would step in thousands however fast the
  read ran; on its own clock it is whatever the read has *truly* reached, so it climbs the way the
  number in any other client does. The last count sent is the exact one, whatever the clock had said
  a moment before.
- **A panel over the grid reports it while it runs** — `shared/ui/FetchProgress`: how long it has been
  going, how many rows have arrived, and `Stop`. It sits *over* the rows rather than replacing them,
  because whatever the grid already held is still true; `Loading` stays out of its way. The clock is
  the panel's own, so counting seconds costs the grid nothing.
- **Stopping keeps every row the read counted** — not only the batches that happened to fill.
  `ScanRows` hands the part-filled batch over on its way out of *any* ending, a stopped read
  included, because the alternative is a count of 60 031 against a grid of 60 000: the read visibly
  losing 31 rows it had already claimed. A stop is a choice and not a failure, so those rows become
  the grid's and the total becomes the number actually held. The columns are the ones already on
  screen, so a *first* read stopped before it answered shows nothing — it had named no columns yet.
  Leaving the tab or changing the limit stops the read too, rather than leaving a scan of a million
  rows emitting batches at a listener that has gone.
- **The count where a stop lands is ahead of the last one shown**, and honestly so: `Stop` has to
  reach the server and the scan only notices between rows, so a read pulling 300 000 rows a second
  gets tens of thousands further in that time. What the grid shows is what was truly read, not what
  the panel last had room to say.

Two limits are worth knowing before pointing this at a very large table. The rows are held **twice**
while they arrive — once as the batches queue for the WebView, once as JavaScript — so memory is the
real bound, which is the trade `All` is asking for. And a **query** result's `All` is unchanged: it
still replays the statement under the ordinary statement timeout, since a script's outcome is one
answer rather than a stream.
