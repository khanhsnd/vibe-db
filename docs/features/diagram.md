# Diagram

[← Features and UI behaviour](../features.md)

A namespace's tables drawn as boxes, with its foreign keys drawn as the lines between them. It is
opened from the tree — right-click a database, a schema or the `Tables` folder and pick `Diagram` —
and there is **one per namespace**, like the table list, so asking for it again reveals the picture
already drawn rather than a second empty one.

## What is drawn, and what is the reader's

**A box names a table and holds nothing else.** The fields inside it, the key marks beside them and
every line on the canvas are read from the same catalogue cache the tree and the designer read
(`stores/connections.structures`), so a column somebody else added appears on a `Refresh` rather than
leaving the picture quietly wrong. A diagram cannot show a constraint that is not there, and it
cannot miss one that is.

**What is stored is the arrangement**: which tables are on the canvas, where each sits, how wide it
is, what colour it wears, whether it is folded shut — and the notes and shapes drawn beside them.
That is a reading preference and no server hears about it; see [where it is kept](#where-it-is-kept-and-taking-it-away).

It follows that **Delete takes a table off the diagram and nothing else**. Dropping a table is a
schema change and is asked for where every other one is — the tree, or the designer. The item on the
box's own menu says `Remove from Diagram` for that reason, and wears the *hide* glyph rather than the
delete one.

A diagram is offered wherever a namespace's tables are the reader's to declare (`createsTables`), so
a Redis key space has none: one flat list of keys is not a set of tables and a box round it would say
nothing. A MongoDB namespace has one, and it draws the fields
[inferred from a sample](../architecture.md#the-document-store) — but no line, that engine having no
foreign key of any kind. What each engine has is [engines.md](engines.md#what-each-engine-can-be-asked-to-do).

## Getting tables onto it

The list on the left is every table of the namespace, with a filter box over it and a tick against
the ones already drawn.

- **Drag one onto the canvas** and it lands where it was dropped. This is the gesture the palette is
  for: it is the only one that says *where*.
- **The plus beside it**, or a double-click, drops it in the first free space — for a reader who does
  not care where it goes.
- **Add Every Table** — the toolbar's plus, and the canvas's own right-click — puts the whole
  namespace on at once.
- **Add Referenced Tables**, on a box's right-click, adds everything that box is joined to by a key
  in either direction. It is how a diagram is built outwards from the one table a reader came for.
- **Arrange** lays the boxes out by depth: a table nothing points at from below goes in the first
  column, a table that points at it in the second, and so on, with the boxes inside a column ordered
  by what they are joined to in the column before. Every line then runs the same way across the page.
  A cycle has no depth, so the pair share a column and the line between them runs vertically.

A double-click on a **box** opens that table's rows, exactly as a double-click in the tree does.

## Moving about it

| | |
|---|---|
| Pan | Drag empty canvas with the Hand tool, the middle button, or `Alt` with the left |
| Zoom | `Ctrl`-wheel at the pointer, or the toolbar's own buttons about the middle |
| Scroll | The wheel, and `Shift`-wheel for sideways |
| Fit | The toolbar's `Fit to window`, which frames everything drawn |
| Select | A press; `Shift` or `Ctrl` adds to the pick; a drag on empty canvas is a marquee |
| Move | Drag any picked box — the whole pick goes with it |
| Nudge | The arrow keys, by one grid step, or by a pixel with `Shift` |
| Resize | The right edge of a box; the corner of a note |
| Fold shut | The caret in a box's header, which is what a caret is for |
| Undo | `Ctrl-Z`, and `Ctrl-Shift-Z` or `Ctrl-Y` forward again |

**A drag is one step, not sixty.** The picture is written on every pointer move so the boxes follow
the pointer, and the state the drag *started* from is what goes on the undo stack when the pointer
comes up — so `Ctrl-Z` takes back the drag rather than the last pixel of it. Panning and zooming are
not steps at all: an undo that put the scrolling back would undo the looking rather than the doing.

**The tools are a letter each** — `V` select, `H` pan, `R` relation, `N` note, `B` rectangle,
`E` ellipse, `T` text — and `Escape` returns to Select and drops the pick.

**How much of each table is drawn** is one choice for the whole diagram: all fields, keys only, or
table names only. Forty tables at full width is forty field lists before it is a shape, and `Keys
only` draws exactly the rows the lines land on. A single box can be folded shut whatever the setting
says, and `Fit Width to Fields` sizes one to what is in it.

## The lines between the boxes

The notation is the one every ER diagram uses, and every part of it is read from the constraint
itself:

- The **crow's foot** is at the table that *holds* the key, which is the many side.
- The **bar** is at the table it points at.
- A **circle** beside the bar means every column of the key may be null — the relation is optional,
  so a row need not point at anything.

Pointing at a line lights it and marks the rows at both ends, so which columns a key is actually on
is answered by hovering rather than by opening anything. `Show relation names` puts each constraint's
own name along its line; it is off by default, forty names being harder to read than forty lines.

**Drawing a line declares a foreign key.** Drag from the round handle on a field — they appear on the
box the pointer is over — onto a field of another table, and the dialog opens with both columns
filled in from the drag. It takes a constraint name, the column pairs (more can be added, a composite
key being two columns pointing at two), and what happens `ON UPDATE` and `ON DELETE`. Create runs the
**same bound method the tree's own Add Foreign Key runs**, and the line appears because the catalogue
now says it is there — nothing is ever drawn that a server has not agreed to. Whatever the server
refuses is said in the dialog, which is where it was asked for.

Deleting one is `Delete Foreign Key`, on the line's own menu and in the panel, and it confirms first
with the same wording the tree uses. The `Delete` key never touches a constraint: it takes boxes and
notes off the picture, and nothing else.

## Notes, shapes and colour

Four things can be drawn beside the tables, and none of them means anything to a server: a **note**, a
**rectangle**, an **ellipse** and a line of **text**. They are what the picture is *for* — saying why
the tables are arranged the way they are. Pick the tool and drag one out, or click for one at its
usual size; double-click it to type into it, and drag its corner to resize it. They are drawn **under**
the boxes always, a rectangle round four tables being a background, and `Bring to Front` /
`Send to Back` order them among themselves.

A box and a shape both take a **colour** from the same closed palette a connection is marked with —
see [connections.md](connections.md#marking-a-connection-with-a-colour) — washed over the header
rather than filled, so a name is read against the same shade wherever it is.

**The panel on the right says what is picked** — the last button on the toolbar opens it: a box's
field count, its keys and every relation it is in; a line's constraint, its columns and its two
actions; a note's words and size. With more than one thing picked it offers the four things that only
mean anything to a pick — line up the left edges, line up the tops, space them evenly across or down.
The button beside it takes the **table list** away, for a reader who has drawn what they came for and
wants the room.

## Where it is kept, and taking it away

The picture is written to `localStorage` under the namespace it belongs to, beside the code that
reads it (`features/diagram/persist.ts`) — the same arrangement a table's column order and a
connection's database filter have, and for the same reason: it is about **one thing**, it outlives the
window, and no server would know what to do with it. It is written on a timer rather than on the
drag, since a box being moved changes it sixty times a second, and flushed when the window closes.

Nothing is read back trustingly: the entry was written by *a* version of this app, and a field that
does not read as what it should is dropped rather than drawn.

`Export as SVG…` writes the whole diagram — everything drawn, not just what is on screen — to a file
picked in the host's own save dialog, and `Copy as SVG` puts the same text on the clipboard. **What
is exported is the element itself**, with every colour, font and width read back off it and written
onto the copy: a file carries no stylesheet, and an export that drew the picture a second way would
be an export that could disagree with the screen. It comes out in the theme the app is wearing.

`Clear the Diagram` empties the canvas after confirming, and says in the confirmation that the tables
are not touched.
