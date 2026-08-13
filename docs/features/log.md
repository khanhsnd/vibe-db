# The History Log

[← Features and UI behaviour](../features.md)

`Tools ▸ History Log` opens a tab holding **every statement this window has run since it started**,
oldest first, across every connection at once. It is the thing to open when something ran that the
reader did not ask for, or when two connections have to be read against one clock.

It is **not** the `History` pane in a query tab, and the two are worth keeping apart:

| | History Log | [The history pane](results.md#the-history-pane) |
|---|---|---|
| Answers | what has happened, and when | which statement do I want to run again |
| Entries | one per **run** — a statement run twice is two lines | one per **statement**, with a `Runs` count |
| Order | oldest first, as it happened | newest first |
| Scope | every connection | the pane's own connection |
| Kept | in memory, for as long as the window is open | on disk, in `history.json` |
| Drawn as | numbered lines | a `ResultGrid` |

## What is in it

An entry is one statement sent by **Execute** — a query tab, a console, the same bound method
behind both. Nothing else is in it, and deliberately:

- **Reading the tree, paging a grid, editing a cell** and every other read the app makes of its own
  accord are not statements the reader wrote. Putting them here would bury the ones that are.
- **A file run and a restore are not in it either.** Either is ten thousand statements, and one of
  them would push everything else out of a log that holds a thousand. Both already report
  themselves, statement by statement, where they were asked for — see
  [the console and Execute SQL File](console.md) and [backup](backup.md). It is the same rule that
  keeps a file run out of the history pane's log.

## What a line says

Four lines to an entry, with a blank between them:

```
[2026-08-07 13:58:08.575][db-gold][db-gold.public][POSTGRESQL]
SELECT id, name FROM users WHERE created_at > now() - interval '7 days'
Time: 0.213s
```

- **The heading is when, on what, where and which engine.** The clock is the reader's own time zone
  down to the **millisecond**, because a log is read to find what ran between two things they
  watched happen and a tenth of a second is usually the gap in question. A fact the app has not got
  is left out rather than drawn empty: an engine with no schemas has no second half to its
  namespace, and a connection closed under a run leaves no name — the line is still written, what
  ran being the fact the log exists for.
- **The statement is written as it was written**, newlines and indentation kept, which is why this
  is lines and not a grid. A statement is not a cell.
- **A statement the server refused carries its wording**, in `--danger`, on its own line above the
  time. It is the server's own words and nothing is added to them.
- **The lines are numbered down the side**, continuously, so a reader can point at one. The number
  is of the line drawn: filtering renumbers, because what a reader means by "line 40" is the fortieth
  line in front of them.

## The toolbar

- **`Clear`** empties the log and starts the numbering again. It asks nothing first, unlike
  `Clear History` beside the query pane: this log is memory, nothing on any server was ever in it,
  and everything in it was already reported where it happened.
- **`Word Wrap`** is on when the tab opens. Turned off, long statements run off the side and the tab
  scrolls across — and the line numbers **ride the scroll**, staying against the left edge, because
  a line the reader has scrolled across to read is one they still have to be able to name.
- **`Show Errors Only`** keeps the entries a server refused and drops the rest.
- **`Pause`** stops the **reading**, not the running. What runs while it is held goes on being kept;
  letting go asks for all of it at once, which is why it is a read rather than a filter.
- **The box on the right** narrows to the entries holding its text, matched however either is cased
  and across everything a line says — a connection's name, a namespace, the statement, the server's
  refusal. With either filter on, the toolbar says how many entries are left.

**A new line carries the reader down with it only if they were already at the foot.** A reader who
has scrolled up is reading something, and a log that yanked them back every second could not be read
at all.

## How it is kept

- **The log holds 1 000 statements** and drops the oldest past that. What it holds after one more is
  `domain.Note`'s rule and is written down once, exactly as `domain.Remember` is the history's.
- **Every entry is numbered**, and the numbers count up for as long as the app runs and are never
  reused. That is what lets the tab ask for *what it has not already drawn* rather than being handed
  the whole log every second — and it is why a gap in the numbers is left visible: a reader who
  paused for long enough lost the middle, and the log says so rather than pretending.
- **The tab reads once a second while it is in front**, and stops when it is not: only the tab in
  front is mounted. What it has read lives in `stores/log.ts` rather than in the view, so looking at
  another tab and coming back does not lose the place the reader had scrolled to, nor which of the
  toolbar's switches they had thrown.
- **A read that fails is dropped rather than reported.** The log is a window on what has already
  happened, and a strip of red over it would be reporting on the reader's own log rather than on
  anything they asked for.
- The tab **belongs to no connection**. Its `profileId` and namespace are empty, the strip draws
  neither, and there is only ever one of it. [Automation](automation.md) is the other tab shaped
  that way, and for the same reason: a job names the connection it dials, and the list holds jobs on
  all of them.
