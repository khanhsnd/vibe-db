# Bringing objects down, and putting them up

[← Features and UI behaviour](../features.md)

One rule holds this whole feature up and everything else follows from it:

> **An object's bytes never pass through a bound method.** They go from the socket to the file and
> back, a fixed buffer at a time. What crosses the bridge is a *number* — how far it has got.

So a forty-gigabyte download costs what a hundred-megabyte one does: 256 KiB per range in flight, and
sixteen ranges at the very most. Nothing on the frontend side ever holds an object, which is why the
panel can watch a transfer nothing on this side is holding.

## What a download actually does

**A small object is one request.** Below sixteen megabytes the bytes go past in order into a
`.vibedl` file beside the destination, and the file is put in place by a **rename** when it is
complete — `jsonstore`'s own bargain: a file that is there is a file that is whole.

That path is also the only one that can **verify**: the sum is taken as the bytes go by and costs
nothing. Where the service's ETag is [a plain checksum](object-storage.md#the-etag-is-not-always-a-checksum)
the file is checked against it, and a file that fails is **removed** rather than left where something
else would open it.

**A large object is read in ranges, four at a time.** A single connection to an S3 service is
throttled well below what a link can carry, so this is the number that makes a large download fast.
Each range is written with `WriteAt` into a file given its final length up front — four goroutines
writing four disjoint stretches need no lock between them and nothing is stitched together
afterwards.

A parallel read cannot hash what it writes out of order, so such a download reports its **length**
checked and not its sum. It says so rather than claiming otherwise.

### It picks up where it left off

Beside the partial file is a few hundred bytes of JSON — the object's identity and one bit per range —
so a lid closed at nineteen gigabytes costs nineteen gigabytes rather than twenty. Ask for the same
download again and the ranges that landed are not asked for again.

**An object rewritten under the same key is a different object.** A plan whose size or ETag no longer
match is thrown away rather than resumed into a file that would be half of each.

A download that failed **keeps** its partial file and its plan: that is what they are for, and the
next attempt is what removes them by finishing. A download that was **stopped** keeps them too — a
reader who pressed Stop has not asked for anything to be deleted, and the files already written stay
written.

### A request that failed is asked again

Five attempts with a doubling wait — about six seconds of patience — for the two kinds of failure
worth another go: the service asked to be slowed down or was briefly unwell (which only the adapter
can tell, so only the adapter says so), or the connection dropped halfway through a body, which is
the commonest failure of a long download. A **cancelled** context is never retried: a reader who
pressed Stop is not waiting to be tried harder for.

### What it does about a file that is already there

The reader's answer, never a default this app picks:

| | |
|---|---|
| **Skip** *(the default)* | leave the file that is there. It is the one of the four that loses nothing |
| **Only if newer** | overwrite where the object is newer than the file — which makes a second run of the same folder a **sync** rather than a repeat |
| **Keep both** | write beside it as `report (2).csv`, which is what a browser does |
| **Overwrite** | replace it |

A skipped file still counts towards what was moved, or the bar under a second run of the same folder
would never fill.

### A key is a string a stranger chose

`../../.ssh/authorized_keys` is a legal key. So is `C:\Windows\...`, and so is a key that is nothing
but dots. **Every segment of a key goes through one sanitiser and the result is checked to be under
the chosen folder even so** — two guards, because the first one being wrong is not something anybody
would notice until it mattered.

The same function makes a key into a name Windows will take: a control character or one of
`<>:"/\|?*` becomes `_`, a trailing dot or space is trimmed (Windows trims them silently, so two keys
differing only there would land on one file — which the reader's own overwrite rule then decides),
and a reserved device name gets an underscore: `con.log` is written as `con_.log` rather than failing
at the last moment with an error nobody can act on.

## A folder is a walk, streamed

Downloading a folder does **not** list it first. The walk hands each object to the queue as it is
read, and the queue's own back pressure is what bounds how far ahead the listing runs — so a folder of
a million objects starts moving bytes before its listing has finished, and costs a page of names
rather than a million.

That is why the panel has a `counting` state, and why it matters: while the listing is still running
the total is climbing, so what is drawn is a **count and not a percentage**. A bar drawn from a
climbing total is a bar that goes backwards, which is worse than no bar at all.

A folder keeps the shape it has in the bucket. `logs/2026/` downloaded into `D:\pull` puts
`2026/03/a.log` at `D:\pull\2026\03\a.log` — the prefix the reader was standing in is taken off the
front, which is the rule an archiver follows about the folder it was pointed at.

## Uploading

The same thing the other way about, with one asymmetry worth naming: **a download can be resumed and
an upload cannot**, because the object a download reads does not change and the file an upload reads
can.

Below sixteen megabytes it is one request. Above it, the file is read with `ReadAt` in parts and sent
four at a time — `WriteAt`'s counterpart — and nothing is held in memory but the buffers the requests
are sending from.

**An upload that fails is aborted.** Parts a service has already accepted are parts somebody is paying
rent on until they notice, so the abort goes out on a context of its own with a short deadline: the
commonest reason to be there is a reader who pressed Stop, and the pieces have to go whether or not
their context has.

**The content type is this app's own list first**, and the machine's table second. The machine's table
is the Windows registry or somebody's `mime.types`, so the same CSV uploaded from two laptops would
otherwise be served as two different things — one of them `application/vnd.ms-excel`, because Excel
claimed the extension when it was installed. An object's type is a fact about the object and not about
whoever happened to upload it.

A folder among the paths is walked on **this** machine, streamed into the queue exactly as the remote
half is, and keeps its own name at the far end: `D:\logs` becomes `<prefix>logs/…`.

## The queue

Four files move at once across the whole app, each large one using four ranges — sixteen sockets at
the ceiling, which is a link busy and a service unbothered.

**A job is a producer and not a list**, which is the design decision the queue exists for: a folder
holding a million objects must not become a million-entry queue before the first byte moves.

**It belongs to the window and not to a tab.** A download outlives the tab it was started from, which
is the first thing a reader finds out by closing one — so the panel is a dock at the foot of the work
area, and closing it leaves a strip that says what is still going. Closing the *window* stops
everything: the bytes go through this process, and the docs say so rather than implying a service is
left behind.

A finished job gets one of four sentences, because they are four different things to have happened:

| | |
|---|---|
| **done** | every file went, and how many were left alone |
| **partial** | some went and some failed, with the service's own words about the last failure |
| **failed** | none went |
| **cancelled** | stopped after *n* of *m* — and what was written is still there |

The panel names **the file in flight**, not just a percentage: a job of ten thousand files that says
only `43%` gives a reader nothing to recognise when it stops on one of them.

**Nothing polls.** The report is pushed while anything is moving — twice a second, which is what a
progress bar needs and far below what a byte counter changes at — and read once when the panel opens.
The alternative is an event per buffer, which is five hundred a second.

## A copy is not a transfer

Copying inside a store, and moving, send **no bytes through this machine at all**: a copy is one
request naming both keys, so a hundred gigabytes moves at the service's own speed and over none of the
reader's bandwidth. That is why neither appears in the transfer panel, and why a move is a copy
followed by a delete — in that order, a delete that went first being a delete of the only copy.

An object past five gigabytes cannot be copied in one request, and this app answers
`domain.Unsupported` naming it rather than starting something it cannot finish. A folder copied into
itself is refused before it starts: it would find its own output and go round for ever, and a walk
cannot notice.

## Where it lives

| Part | Where |
|---|---|
| one object down, with its ranges and its resume | `internal/objectstore/download.go` |
| one file up, with its parts and its abort | `internal/objectstore/upload.go` |
| a key made safe to be a path | `internal/objectstore/paths.go` |
| the queue, and what a job reports | `internal/objectstore/queue.go` |
| a folder deleted or copied in batches | `internal/objectstore/folder.go` |
| retrying what is worth retrying | `internal/objectstore/retry.go` |
| the bound methods and the pushed report | `internal/app/store_api.go`, `EventTransfers` |
| the dialog and the dock | `frontend/src/features/storage/DownloadDialog.tsx`, `TransferPanel.tsx` |
