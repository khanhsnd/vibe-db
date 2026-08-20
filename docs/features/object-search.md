# Finding an object in a bucket

[← Features and UI behaviour](../features.md)

A bucket has no columns, so there is nothing to filter. What it has is names, and this is how they
are searched.

**There is no index and there will not be one.** An index of somebody's bucket kept on this machine
is a copy of their data that is stale from the moment it is written and wrong in the one way that
matters: it would answer confidently about an object that is no longer there. What this does instead
is make the *service* do as much of the work as it can, and stream the rest past without keeping it.

## The one rule about matching

**What the reader's text matches is a `domain` rule** — `domain.TextMatch`, the same one a [Find in
Schema](search.md) is spelled from — so `Contains` means of a key exactly what it means of a column
name. Four modes, and the bar over the grid is where they are picked:

| Mode | What is typed |
|---|---|
| **Name contains** *(the default)* | any part of a key, folders included. `%` is any run of characters and `_` is one |
| Key starts with | the front of the **whole key** — see below |
| Name holds the word | the text between two word edges, taken literally |
| Regular expression | the reader's own |

### A key is a path, and that is why the default is Contains

`Key starts with` is the only mode a service can narrow, so it is the quick one — and it was the
default until it turned out to be the wrong one for the commonest thing anybody types.

The text is matched against the **whole key**, folders and all. So in a bucket holding
`private/77/b5867e28….docx`:

| Typed | `Key starts with` | `Name contains` |
|---|---|---|
| `private/77/b5867e28….docx` | found | found |
| `b5867e28….docx` | **nothing** | found |
| `b5867e28…` | **nothing** | found |

Nothing was wrong in either case: a key beginning `private/` does not begin with a file name. But
"nothing matches" about an object that is right there is the worst answer a search box can give — one
mode costs seconds and the other sends a reader off believing their object has gone. So the default is
the one that finds it, and speed is the reader's to choose.

**A search that finds nothing in `Key starts with` says this in the pane**, and offers the other mode
in one press, rather than leaving the reader to work it out.

Beside them are three bounds: **larger than**, **smaller than**, **written after** and **written
before**. They are free — every one of them is a fact the listing already carries — which is exactly
why they are there and not a second read, and they are folded away because free is not the same as
often wanted.

## What the service can be asked, and what it cannot

A service narrows a listing exactly one way: *the key begins with these characters*. That is the
whole of it — the same bargain [a key space](key-space.md#a-key-space-is-searched-not-filtered)
strikes with its glob — so a search's speed is decided by whether its front is **literal**.

`PruneTo` is the function that decides, and it is worth reading the two cases it gives up on:

- **A match that ignores case prunes to nothing.** Keys are case sensitive at every service there is,
  so handing `logs/` to the service would miss `Logs/` — a pruned listing that is *missing rows the
  reader asked for* is worse than a slow answer. Where the reader is standing still prunes, and the
  box says so.
- **A regular expression that is not anchored prunes to nothing.** `[0-9]+\.log` may match anywhere
  in a key, so nothing about its front is known. One that opens with `^` is anchored and its literal
  head is used, which is what makes an anchored expression the fast one to type.

So `Key starts with logs/2026-01` at the top of a forty-million-object bucket is **one request**, and
`Name contains invoice` in the same bucket reads every key under the folder. The bar says which
happened: it draws how many keys were read, and the answer carries the prefix the service was
actually narrowed by.

**Standing somewhere prunes whatever the mode is.** A `Contains` search run inside `logs/2026/` reads
that folder and not the bucket, which is the cheapest thing a reader can do about a slow search and
costs them one double-click.

## It is bounded, and it says so

Two bounds, because a bucket has no size a reader can know before asking:

- **Two hundred matches** and it stops, and says *there are more of these*.
- **A quarter of a million keys** looked at and it stops, and says the answer is a window rather than
  the bucket.

Both of those are in the answer rather than inferred: `whole` is whether every key under the prefix
was seen, and `more` is whether it stopped because it had found enough. They are different sentences
and a reader acts differently on each.

**A search has no deadline of its own.** How long a bucket takes is the bucket's business, so the
honest bound is the reader's own Stop — the arrangement [a read of every
row](row-limit.md) and a Find in Schema both have.

## What it costs while it runs

**A quarter of a million keys go past one at a time and the ones that do not match are forgotten as
they go.** Only the hits are held, and there are at most two hundred of them, so searching the
largest bucket in the world costs a page of names in memory. That is `Walk`'s bargain, which
`internal/transfer`'s readers and `dump.go` already struck for rows.

The cheap question is asked first, too: a size and a date are two integers and a regular expression
is not, so the three bounds are applied **before** the matcher over every key.

## What the answer looks like

The grid draws the same six columns, with one difference: a hit is named by its **whole key** rather
than by its tail. A search answers across folders, and twelve rows all called `index.json` would say
nothing about where any of them is. They are grouped on the slash into an
[outlined column](../grid.md), so a hundred hits under four prefixes read as four prefixes.

Everything a reader can do to a row of a folder they can do to a row of a search: download it, copy a
link to it, delete it. `Escape` in the box, or the cross in it, puts the folder back.

## It commits rather than narrowing as it is typed

This is the other half of what makes it different from a [key space's search
bar](key-space.md#nothing-in-the-bar-waits-to-be-applied), which narrows on every letter.

That bar re-reads **one page**. This one re-reads a **bucket** — so a letter that started a quarter of
a million requests would be a letter nobody could afford. `Enter` runs it, and the button beside the
box says `Search` while nothing is going and `Stop` while something is.

## Where it lives

| Part | Where |
|---|---|
| the rule about matching | `internal/domain/search.go` (shared with Find in Schema) |
| the search, the pruning, the bounds | `internal/objectstore/search.go` |
| the streamed listing it is built on | `internal/objectstore/walk.go` |
| the bound method | `StoreAPI.SearchObjects`, `StoreAPI.StopObjectRun` |
| the bar and what it draws | `frontend/src/features/storage/ObjectSearchBar.tsx` |
