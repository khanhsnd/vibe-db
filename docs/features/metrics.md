# Live charts

[← Features and UI behaviour](../features.md)

## What it is

The `Charts` pane of the [Server Monitor](monitor.md) is the same server read for what is **moving**
rather than for what is there: four charts, redrawn on the same clock the panes beside it re-read
themselves on, over every reading taken since the tab was opened.

It is the pane a plain open lands on, and it is first on the toolbar, because it is the one that
answers *is this server all right* without the reader already knowing what to look for. The four
lists beside it — the [processes](monitor.md#reading-it), the
[slow queries](profiler.md), the status and the variables — are read by somebody who already has a
question.

| Panel | What it is asking |
|---|---|
| Sessions | Who is connected, and how many of them are doing something |
| Traffic | How much data the server is moving |
| Queries | How much work is being asked for |
| Throughput | What that work actually moved |

**The panels are fixed and the lines inside them are the engine's own.** It is
[the process list's arrangement](monitor.md#what-it-is) — one shape, whatever columns that catalogue
keeps — and it is here for a sharper reason: no two of these engines count the same things. MySQL
counts bytes on its own sockets and PostgreSQL counts pages off the disk; Oracle counts logical reads
and Redis counts key-space hits. A vocabulary that insisted the six were one number would be this app
inventing a fact, so what a driver hands over is **its own name for its own counter**, with a label,
sorted into the question it helps answer.

The last panel is the reason the first three are not enough. A hundred statements an hour that each
read a million rows is the busiest server there is, and every one of the other panels would call it
quiet.

## What each engine counts

| Engine | Sessions | Traffic | Queries | Throughput |
|---|---|---|---|---|
| MySQL | `Threads_*` | `Bytes_received`, `Bytes_sent` | `Questions`, `Slow_queries`, `Com_commit`, `Com_rollback` | `Innodb_rows_*` |
| PostgreSQL | `pg_stat_activity` by state | `blks_read`, `blks_hit` × `block_size` | `xact_commit`, `xact_rollback`, `deadlocks` | `tup_*` |
| SQL Server | `dm_exec_sessions` by status | `dm_io_virtual_file_stats` | `SQL Statistics`, `Transactions/sec` | `Access Methods`, `Buffer Manager` |
| Oracle | `v$session` by status | SQL\*Net bytes | `execute count`, `parse count`, commits, rollbacks | logical reads, physical reads, block changes |
| MongoDB | `connections`, `globalLock.activeClients` | `network.bytesIn`, `network.bytesOut` | `opcounters` | `metrics.document.*`, `metrics.queryExecutor.*` |
| Redis | `connected_clients`, `blocked_clients` | `total_net_input_bytes`, `total_net_output_bytes` | `total_commands_processed`, `total_error_replies` | `keyspace_hits`, `keyspace_misses`, expiries, evictions |

**Traffic is not bytes on the wire on two of them, and the labels say so.** PostgreSQL counts nothing
about its own sockets and SQL Server counts packets rather than bytes, so what is honestly there is
the traffic each moved to *answer* — pages off the disk, bytes read from and written to its own
files. A panel that called either of them `Sent to clients` would be a number the server never kept.
The panel title is the question; the **series label** is the answer.

**PostgreSQL has no statement counter** without an extension, so its Queries panel is what a bare
server does count: transactions, and the deadlocks it resolved. What ran *slowly* is
[the Slow Queries pane](profiler.md), which is where `pg_stat_statements` is read.

**A counter this server's version has not got is left out**, never sent as a zero: a flat line at
nothing and a line the server will not draw are opposite answers, and a panel that filled itself with
falsely flat lines would be one a reader could not tell a quiet server from a missing one on. A panel
with nothing in it says so in words.

**SQLite has none of this**, for the reason it has no monitor at all: a database there is a file, and
a file is not doing anything. See [`hasMonitor`](engines.md#what-each-engine-can-be-asked-to-do).

## How a number becomes a line

A metric arrives as one of two kinds, and **only the driver can say which**: `Threads_connected` and
`Bytes_received` sit in the same list and mean opposite things.

- A **gauge** is true at the moment it was read, and is drawn as it stands.
- A **counter** has only ever gone up since the server started, so what is drawn is the difference
  between two readings **over the seconds between them** — a rate.

That is why every counter panel is empty until the *second* reading arrives, and why the first point
of every line on one is a gap rather than a zero. One reading of a counter says what the server has
done since it started, which is a number about last week.

**A reading lower than the one before it is dropped rather than drawn as a fall.** A counter only
goes up, so a smaller number means the server restarted or was told to forget — `CONFIG RESETSTAT`,
`FLUSH STATUS` — and a plunge to the floor would say the server did something it did not do.

**The moment a reading belongs to is stamped on the Go side**, at the moment the answer came back,
because that interval is what a rate is divided by: a clock read after the answer had crossed the
bridge would fold this app's own latency into whatever the server was doing.

**Everything on the four panels is one read.** `LoadServerMetrics` is a single bound method and a
single query per engine — even where the numbers live in three catalogues, as they do on SQL Server —
because a session count from one moment drawn beside a rate from another would be two servers on one
screen.

**A panel is one kind, one unit and at most six series**, and every adapter's table is held to all
three. A panel takes its axis and its wording from the first series it holds, so a gauge listed among
counters would be a level divided by the seconds between two readings, a size among tallies would
have a megabyte labelled `50/s`, and a seventh line would be handed the first one's colour. None of
the three is anything a compiler can see and all three fail *quietly* — what reaches the reader is a
chart that looks perfectly ordinary — so `driver.PanelFault` writes the rule down beside the
`MetricSpec` it is about, and each adapter's own test checks its table against it.

## Reading it

**The clock is the tab's**, the same `Auto: 5s` the panes beside it use, and `Off` freezes the charts
where they are rather than emptying them. Nothing is asked while nobody is looking at a chart, so
opening a monitor on the process list costs no metrics read, and neither does a monitor tab left
behind another one — a production server is asked only while somebody is watching the answer.

**What has been collected outlives the view.** The readings live in a store keyed by the tab rather
than in the pane, because only the tab in front is mounted: held in a component they would go the
moment the reader looked at a query tab. That costs the four lists beside this one nothing — a process
list is a picture of now, and reading it again is one read — and it is the whole of what a chart is.
So the readings go when the **tab** closes, which is what "they live in the tab" has to mean.

**A stretch nobody was watching is drawn as the stretch it was.** The clock stops while the tab is
behind another one, so coming back to a chart leaves a gap in it — and because the axis is time, that
gap is as wide as the time it took. The line across it is one reading's worth of average over the
whole of it, which is the honest reading of a counter nobody sampled in between.

**A server slower than the clock is asked once and waited for.** A chart is a fixed run of readings,
so a pile of overlapping reads would answer in whatever order they finished and put the wrong number
of seconds between two points.

**The last 240 readings are kept, and none older than two hours.** The count is eight minutes at the
fastest clock and two hours at the slowest; the two hours is a *second* ceiling, because a count alone
stopped bounding the time once the readings began outliving the pane — a tab glanced at each morning
for a week would otherwise hold seven readings a week apart and squash all of today into the last
pixel of the axis. Anything longer is what a monitoring system is for, and this app is not one. The
footer says how many readings are held and how long they cover, and says when either ceiling has been
reached: a reader watching a long stretch can see it is about to start losing the front of it.

**A failed re-read leaves every chart where it is** and puts the reason underneath, which is what
[the panes beside it do](monitor.md#reading-it) with the same failure. The one exception is a failure
before anything has come back at all, where there is no history standing behind it to protect.

**A point sits at its own instant, not at its place in the run.** The clock is a thing the reader
changes while readings accumulate, so twenty readings taken 30s apart and twenty taken 2s apart are
the same *count* and forty times the stretch: spaced evenly they would draw ten minutes and forty
seconds the same width, and the two clock times under the chart would be labelling a picture that did
not honour them. Every value on these panels is already divided by the seconds it actually took, and
the axis is drawn from the same stamps — nothing here is spaced by counting.

**The pointer reads a moment off, on all four panels at once.** Moving across any chart draws a
crosshair at the nearest reading **by time** and moves every number on the pane — the dots on every
line, the times under them and all four legends — to that instant. Off the charts, they are all the
latest reading. It is an **index** into the one clock the four panels share, which is what lets them
be read against each other at all: a panel answering for the moment under the pointer beside three
answering for now would be the shape a reader is comparing and the numbers of a different second.

There is **no tooltip**, because the legend is one: a panel this size already has its numbers written
out three lines below the pointer, and a box that followed the cursor would be the same figures twice
over the lines they are about.

**A legend entry is also the control that cuts the panel down to it.** One axis over series of
different magnitudes is an axis that hides most of them: on a busy MySQL server `Innodb_rows_read`
runs four orders of magnitude above `Innodb_rows_updated`, and against a ceiling taken from the first
the second is drawn along the floor, where a real trickle and a dead stop look identical. Pressing a
line's entry draws that line alone and re-takes the ceiling from it; pressing it again brings the rest
back. Every line stays in the legend while one is isolated, dimmed rather than dropped, so the way
back is where the way in was — and a series the server stops reporting un-isolates itself rather than
leaving an empty panel.

**A line is a colour and a pattern**, cycling solid, dashed and dotted over the six colours. Colour
alone is one channel, and six hues are six distinguishable lines for most readers and fewer for a
reader who cannot separate the green from the amber — which on these panels are neighbours. Three
patterns over six colours means no two adjacent series share either, and the legend's mark carries the
pattern as well as the colour: a key to half of what distinguishes the lines is half a key.

**The axis is rounded up to 1, 2 or 5 times a power of ten** rather than to the largest value on the
chart. An axis that ended at the exact maximum would move on every reading, and gridlines that
shuffle while a chart is being watched are gridlines nobody can read a level off.

## What is not here

**The box and [Filter & Sort](../grid.md#the-filter--sort-panel) are dropped on this pane.** Both
narrow *rows*, and there are none — the four panels are already the four questions, and a condition on
a line would be a `WHERE` over a picture. The one thing here there is to hide **is** a line, and it is
hidden by pressing it in the legend rather than by a control on the toolbar: the legend is the key to
the picture, so it is where a reader is already looking when they start wondering about one line.

**Nothing here is a grid, and nothing here is written down.** The readings live in a store keyed by the
tab and go when it closes — no file, no retention, nothing anybody could open later. A client that
kept a server's numbers on disk would be a monitoring system with one node and no retention policy,
and the [Slow Queries pane](profiler.md) is already where "what happened while I was away" is
answered — by the *server's* own record, which is the only one that was actually there.

**A number cannot be changed from here**, for the reason
[a variable cannot](monitor.md#what-is-not-here): there is nothing on any of these four panels that is
a setting.
