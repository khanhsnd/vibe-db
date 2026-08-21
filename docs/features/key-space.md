# Browsing a key space

[← Features and UI behaviour](../features.md)

**A Redis connection's tree is short, and everything missing from it is missing on purpose.** The
connection holds its databases — numbered `0` upwards, not named — each holds a `Data` folder, and
that folder holds one entry: `(All Data)`. There is no Views folder and no Functions folder because a
Redis server has neither; there is no New Database and no Delete Database because its databases are
fixed when it starts; and there is no New, Rename, Duplicate, Design or Delete on the key space
itself, because it is not declared and never was. What is left is what a key-value store actually
offers, and every one of them works.

**Opening it opens a grid of keys, five columns wide.** They are not read from a catalogue and not
inferred from a sample: they are the same five on every Redis server there has ever been.

| Column | What it is |
|---|---|
| `Key` | The key, and the **primary key** of the row — it identifies exactly one, which is what makes the grid writable at all |
| `Type` | What `TYPE` answered: `string`, `list`, `set`, `hash`, `zset`, `stream` |
| `Value` | A string as itself; the five container types as **JSON** — a list as an array, a hash and a sorted set as objects, a stream as its entries. A value larger than a row can carry ends with `… 50 of 4321`, which says it is a sample |
| `TTL` | Seconds until the key expires, and **empty** for a key nobody put a clock on. Empty is not zero: zero would mean it has already gone |
| `Size` | What `MEMORY USAGE` says the key costs, and empty on a server that will not answer that |

**Four of the five can be written, and each is a different command.** Writing `Value` writes
the value **whole** — the JSON in the cell is what the key becomes — and the key's expiry survives
it. Writing `TTL` is `EXPIRE`, and *clearing* it is `PERSIST` rather than an expiry of zero, which
the server reads as *now* and answers by deleting the key. Writing `Key` renames it, and is refused
if a key of that name is already there rather than taking it. `Type` is written **with** the value
rather than on its own — a key's type is what its value is, so the two are one fact and one write; see
[the panel](#the-key-panel-is-always-open). `Size` is nobody's to change.

A value the grid could only **sample** refuses to be written back, and says so: whatever it did not
show would go with the edit.

**Adding a row writes a new key** — its `Type` decides what is written and defaults to `string` — and
is refused if the key is already there, an insert being for a row that is not. **Deleting a row** is
`UNLINK`, which frees a million-element list on a background thread instead of stopping the server
with it. **Empty Key Space** is `FLUSHDB`.

## The keys are drawn as the tree they spell

**A key space is flat and a key is not.** `user:1:name` is a path, every server and library there is
spells one with colons, and a reader looking at sixteen thousand keys is looking at a few dozen
prefixes. So the grid **groups them**, on the colon and on nothing else: a prefix is a branch, and a
key is a leaf named by its **tail**, the prefix being on the row above it. A key with no colon in it is
a leaf at the top, whole.

How many keys a branch holds is a **pill beside its name** rather than part of it — the same pill the
sidebar's own tree draws its counts as, muted mono against the name's own weight, so the number is read
at a glance and never mistaken for the key. It is chrome and not the value: nothing copies it, nothing
measures it, and nothing can rename a key into it.

**A branch is a folder, not a key.** Clicking one opens nothing below — it is a prefix the reader is
walking through, and there is no value to show. That holds even for a prefix that is *itself* a key,
which is the one thing the tree cannot say in a row; **Flatten** is where such a key is reached, every
row there being a key.

It is the **default**, and **Flatten** on the toolbar is how it is turned off — that is the page
exactly as it was read, one row per key, whole names. Which it is, and which branches are folded, is
the tab's and survives being looked away from, like everything else it was set up with.

The grouping is a *reading* of the page and asks the server nothing: it is the key column drawn as an
[outlined column](../grid.md), which is the same mechanism a comparison's shape half uses. Because a page
is cut from a **sorted** scan, every prefix is contiguous and the tree of a page is a coherent slice
of the tree of the whole. **Branches stand above leaves** at every level — the folder-first order a
[bucket's own page](object-storage.md#the-grid) is listed in, this being the same flattener — and it
is a partition of a level rather than a sort, so each half is still in the order the scan answered
in. What a **branch** row is not is a key — nothing can be written to one, and
an export writes the flat page rather than the tails the tree drew.

## A key space is searched, not filtered

**The Filter & Sort panel is not there, and a search bar stands where its button does.** A type, and a
fragment of a key's name. That is not a narrowing of the feature but the shape of the engine: Redis
narrows a read exactly one way — a glob against a key's name — so most of the builder's twenty
operators would be twenty ways of asking for something that could only be answered by reading every
value on the server.

Underneath it is **an ordinary condition tree** — `Key contains …` and `Type = …` — so the read that
follows is an ordinary read and the driver is unchanged by any of it. A name typed in the box becomes
the server's own glob, so the narrowing happens *there* and the app never sees the rest:
`SCAN 0 MATCH *user* COUNT 1000`, and the statement under the grid says so. The type is answered
here, over the keys the glob let through.

**Nothing in the bar waits to be applied**, which is the other half of leaving the builder out: a
letter typed and a type picked each narrow the read as they happen, where the panel they replace
stages a draft and commits it on Apply. The type is the app's own menu of the six and *All Types*
rather than the platform's list box, so picking one is one press and not a press to open, a walk and
a press to commit.

### How the name is read is picked in the box

**The magnifier in front of the box opens a menu of three**, because the same three characters are
three different questions and the box is where they are typed.

| Mode | What is typed | The condition, and the glob it becomes |
|---|---|---|
| **Keys Contain** | a fragment of a name, case ignored | `Key containsfold email` → `*[eE][mM][aA][iI][lL]*` |
| **Keys Contain (Match Case)** | a fragment, letter for letter | `Key contains email` → `*email*` |
| **Keys Match Pattern** *(the default)* | the server's own glob | `Key matches *email*` → `*email*`, as typed |

**The glob is what the box starts in**, because it is the notation the engine is already in: the
statement under the grid is a `SCAN … MATCH`, and a reader who types `*email*` over a key space
means the pattern rather than a key with stars in its name. The two Contains modes are for the
reader who means the characters.

All three are **one mechanism**: a condition on `Key` differing only in its operator, spelled as the
glob the scan is narrowed by and applied again here to the keys that came back. Two of those
operators are ones the [filter builder](filter-sort.md) does not offer, and deliberately:
`matches` reads its value as a pattern in the **engine's own** notation, which a key space has and a
SQL table has not, so an engine that does not know them refuses them like any other operator it has
not got.

**Only Match Pattern lets a wildcard through.** In the two Contains modes a `*` is a star and
nothing more — a key called `report*2024` is a key that can be searched for — because a reader
typing a fragment is typing a name. In Match Pattern nothing is escaped: `*`, `?`, `[0-9]` and
`[^a]` are the server's own, `\*` is a star itself, and what the statement shows is what was typed.

**Case is folded into the pattern rather than asked of the server**, Redis having no
case-insensitive matching of its own: `user` becomes `[uU][sS][eE][rR]`. It is exact rather than
approximate, which is what lets the server keep doing the narrowing for a search that ignores case —
the same spelling [Find in Schema](#what-a-right-click-on-a-key-offers) uses.

**Which mode it is, is the tab's** and survives being looked away from, like Flatten and everything
else it was set up with. It is held there rather than in the condition it writes because an empty
box writes no condition at all, and a mode picked before a word is typed would go with it.

The rest of what the driver can be asked is still what it always was, and is worth knowing because it
is why the bar is only two boxes. A filter on `Key` — begins, contains, ends or equals — becomes the
glob; everything else — a type, a clock, a cost, a value — is answered here, so those cost a read per
key and a filter on the **value** is bounded at five thousand keys and says so if there are more. A
condition inside an `OR` never becomes the glob: `Key begins user OR Type is hash` admits every hash
in the database, and narrowing the scan by half of it would hide rows that were asked for.

## The key panel is always open

**Under the grid sits the key itself, and there is no toggle for it.** On a table store a cell editor
is one of several ways to reach one cell of forty, so it is a panel the reader asks for; here there
are five columns, three of them one line long, and the fourth is a container — so there is nothing for
a toggle to save and a grid row is the wrong shape to edit one in.

**Which is also why the grid itself does not edit.** Clicking a key **shows** it — in the panel, whole
— and changes nothing. On a table store one click opens the cell, because the reader is already
pointing at the one value they came for; on a key space a click is how the keys are *walked*, and an
editor opening under every one of them is in the way. So editing has exactly one place here, and the
footer still writes: **Add**, **Delete**, **Apply** and **Discard** are unchanged.

**A click picks the key, so it bands the whole row** rather than filling the one cell under the
pointer. A row here is one key and its five facts; picking the `Type` cell of it and picking the key are
the same act, and the band says so.

It holds the four things a key is: its **name**, its **type**, its **value** and its **clock**. Its
layout is four fixed rows with the value taking the slack, so nothing rides over the clock however
short the panel is dragged.

**The clock is said three ways**: **No expiry**, **Expire In (seconds)**, and **Expire At (Local
Time)** — a moment, picked in the platform's own calendar as every other date in the app is. Only the
first is a different fact; the other two are one number said two ways, so switching between them keeps
what was said and the panel shows how many seconds off the moment picked is. What the key actually
holds is always the **seconds**, because that is what a Redis clock is: converting here rather than
storing a date is what stops the two drifting apart between the write and the next read. A moment
already **past** is refused rather than sent — the server reads an expiry of zero as *now* and answers
by deleting the key.

**The type is picked, and picking one is a promise about the box below it.** A type is what the value
*is*, so there is no converting a key: what happens is that the key is **emptied and refilled** as the
type chosen, from the text standing in the value box. A hash written as a list is that list; a value the
new type cannot read — a hash's own JSON asked to be a list — is refused *before* a single command is
sent, so the key is left exactly as it was.

That is also why the type is **not a cell of its own**. Writing it apart from the value could not work
in either order: a value in the new type's shape is refused by the old type's parser, and a type change
on its own has nothing but the old value to re-express. So the two travel as one write
(`driver.CellEdit.ValueType`), which is what makes changing a key's type and its value **one** press of
Apply. Editing the `Type` cell directly says so rather than half-working.

A **stream** is the one type that cannot be written: its ids are the server's and it is append-only.
Picking it says so before anything is sent.

**The two values whose members are pairs are laid out as those pairs; the rest are read whole as
text.** A hash's members have names the reader thinks in and a sorted set's have ranks, so each is two
columns — **Field** and **Value** for a hash, **Element** and **Score** for a sorted set, the score
drawn as the number it is — with `+` and `−` to add a member and take one out, and a count under them.
A list and a set are positions rather than pairs, so they are read in the ordinary
[value viewer](value-viewer.md#reading-a-value-whole) like every other value in the app, laid out
and syntax-coloured, and so is a string.

Either way it is the **same JSON**: the members are read out of what the driver wrote and written back
into what it parses, so the table is a *layout* and never a second notation. Text that cannot be read as
members — which is what the box holds the moment a reader starts retyping a value into some other shape
— falls back to the viewer, so there is always a box under them.

**The panel reads the whole value, even where the grid could only carry a sample of it.** The grid's
own bound is what makes a page answerable — a hundred keys must not be a million members — but the
panel is looking at *one* key, and there is nothing left there for the bound to protect. So a key whose
row ends `… 50 of 117` opens with all hundred and seventeen, and is fully editable.

It is an ordinary row read narrowed to that one key, not a call of its own. Two things make it cheap:
a filter naming exactly one key is **looked up** rather than scanned for — `SCAN` walks the whole key
space however narrow its pattern, so reading one value would otherwise cost a walk of a million — and
nothing is read at all where the row already carried the value whole, which is nearly every key.

While it is on its way the sample stands, so the panel never goes blank; when it lands the panel starts
again on it, a draft typed against a sample never having been writable anyway.

**Apply stages, and the footer's Apply writes** — the same two steps the cell editor has on every
other surface, and the reason it matters here is that a rename, a new value and a new clock are three
cells and become **one** press: three commands in the order they have to run, a rename last, because
every command before it names the key by the name it had. It is `Ctrl+S` as well as a button, on the
panel and not on the window, for the reason [the cell editor's is](table-tab.md).

A value the grid could only sample, a **stream** and a module's own type all refuse to be written, and
the panel says which before anything is typed rather than after Apply.

**What such a value refuses is itself, and nothing else about the key.** A rename is `RENAME` and a
clock is `EXPIRE`, and neither so much as reads what the key holds — so a key whose value is a sample
can still be renamed and still be given an expiry, and Apply is live for exactly those. Blocking them
too would mean a key that happened to be large could never be given a clock at all, which is the
opposite of what the refusal is protecting.

## What a right-click on a key offers

**A shorter menu than every other grid's**, because the long one is built for a table store: there is
no NULL to set on a key that would then not exist, no UUID worth minting into one, no condition to add
— the bar above is where a key space is narrowed — and no sort worth offering over a page cut from a
sorted scan. What is left is what a reader does to a key.

**Edit Key** puts it in the panel below, **Add Key** appends one, **Delete Key** marks it — all three
being the footer's own buttons reached without travelling to the bar — and then **View Value**,
**Copy**, **Copy Field Name**, **Copy As**, **Save Data As** and **Refresh**.

**A key is added inside the folder the reader is looking in**, so the box opens with **that prefix and
the colon** rather than empty, and the whole of what is left to type is the tail. It is the same answer
whichever row is picked — a branch *is* that folder, and a leaf is a key in one, so its prefix is
everything up to its last colon — which is what makes the button mean one thing wherever the reader
happens to be standing. A key with no colon in it is at the top and offers nothing, which is the empty
box.

**The rows are in key order** before a page is cut from them, because `SCAN` has no order of its own
and a page cut from an unordered list is a different page every time it is read. Sorting by any other
column sorts what was read for it. A database holding more than a million keys refuses to be ordered
whole and asks for a filter instead, rather than showing a window that lies about how big it is.

**Find in Schema searches key names.** That is not a narrowing of the feature but the whole of what
the server can answer — there is no index over values and no LIKE — so `Contains` and `Prefix` are
answered exactly, case folded into the pattern itself, and the two modes a glob cannot say say so
rather than guessing. Double-clicking a hit opens `KEYS <glob>`, which returns precisely the keys it
counted.
