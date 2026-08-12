# Snippets: the statements you named

[← Features and UI behaviour](../features.md)

**A snippet is a statement the reader named and kept.** It sits beside the
[History pane](results.md#the-history-pane) because the two are one kind of thing looked at from
opposite ends: history is every statement that ran, kept by the app, and the library is the handful
somebody decided to come back to.

So a snippet carries **no connection, no namespace and no outcome**. A statement worth naming is one
worth running in more than one place, and pinning it to the database it was first typed against would
make a library of one connection's statements rather than of the reader's. The one narrowing there is
is the **engine**, and that is because the library is offered by the editor's own completion: a
`SELECT … LIMIT 10` offered in front of a Redis tab is noise, and a document store's commands are not
SQL at all. A snippet tagged with no engine is offered in every editor.

## Where it lives, and why not in the browser

**The library is a file**, `snippets.json`, in the same folder as the connection profiles and the
query history — `internal/snippet/jsonfile`, over the same `jsonstore.File` those two use, so it is
read whole, written whole, and put in place by a rename. A crash mid-write leaves the library that was
there rather than half of a new one.

That is the difference between it and a [restored query tab](query-tab.md#the-tabs-are-still-there-next-time),
which is `localStorage`'s. A query tab is *this window's* unfinished work; a library is something
built up over months, and a reader is entitled to back it up, copy it to another machine, read it,
and diff it. Every field in the file is a string, so all four of those work in any text editor.

- **A snippet is identified by its name, folded.** Saving under a name the library already holds
  replaces that entry rather than standing a second beside it, which is what makes editing one and
  saving it a single gesture. `Recent Orders` and `recent orders` are one snippet, because completion
  reaches a name that way.
- **A rename lets go of the old entry in the same write.** The name it was known by travels in with
  it (`previous`), so there is no second call that could fail on its own and leave the library holding
  both.
- **A save that is refused writes nothing.** The name is required and the body is required — see
  [Failing](#failing) — and a rejection leaves the file exactly as the reader last left it, with their
  text still in the boxes.
- The library is sorted by name on the way out as well as on the way in, so a file somebody has typed
  into by hand still reads as a library rather than as the order things were added.
- **Every write answers with the library it leaves behind.** The list is held in one store shared by
  every editor and by the Snippets pane, so there is no window in which a snippet has been saved and
  the completion beside it has not heard.

## `${…}` is a field, not a variable

**The body is put in the editor verbatim**, and `${…}` in it marks a field the caret jumps between
with `Tab`. It is the notation the dozen [built-in snippets](../editor.md#what-completion-offers) have
always used, and the reader's own bodies are read exactly the same way — so turning a working
statement into a form is a matter of wrapping the parts that change:

```sql
SELECT ${columns}
FROM orders
WHERE placed > ${date}
  AND status = '${status}'
```

- **It is not a parameter and nothing is bound.** A field is text the reader finishes typing before
  they run anything; the statement that reaches the server is whatever the editor holds by then, on
  the path every statement takes. Nothing about a snippet is sent anywhere.
- **Nothing reads the body.** It is not split, not laid out, not checked and not lexed on its way in
  or out — what a statement *is* is the engine's answer, and a snippet is not in front of one.
  `Format` and the [problems strip](script.md) do their usual work once it is in the editor, because
  by then it is just text in a buffer like any other.

## Reaching one

There are two ways, and they are deliberately different.

**By typing its name**, in completion, wherever a statement is written — a query tab and a
[definition editor](definitions.md) alike. A saved snippet is offered at the same word as everything
else and **sorts above** it: a name somebody chose is a name they meant, and having to scroll past
`SELECT` to reach it would be the library not being worth keeping. The description stands as the
detail line where there is one, and the body's first line where there is not.

**Out of the `Snippets` pane**, beside `Result` and `History` in a query tab. A double-click — or
`Insert` — puts the body in **at the caret**, replacing whatever was selected, with its fields live.
That is the whole difference between it and a history entry: a history entry is a statement that ran
and is put back **whole**, over the buffer and with the namespace it ran in; a snippet is a piece
meant to be built with.

- Both go through the same `snippet` the completion list applies, so a body typed by name and one
  picked out of the pane behave identically — fields and all.
- The pane lists the **whole** library rather than only this engine's. Completion is narrowed, because
  a name offered where it cannot run is noise; but a reader looking at their library is looking for
  everything in it, and one they cannot see is one they cannot edit.
- The pane is a [`ResultGrid`](../grid.md), because it is rows and columns — `Name`, `Engine`,
  `Description` and the statement as one line — and it is read-only: a body is edited in the dialog,
  which has the room for a script. `F2` on a row opens that dialog, as it does everywhere a row has
  something to rename.
- **Nothing is read when the pane opens.** The library is read once when the window opens, because
  every editor's completion needs it whether or not the pane has ever been looked at. `Refresh` in the
  footer is for a file somebody has edited underneath the app.

## Making one

**`Save Snippet`** in a query tab's toolbar opens the dialog on whatever is selected, falling back to
the whole buffer — the same text `Execute` would run — so naming the statement in front of you is one
press and no copying. It needs **no connection**: a snippet is text this machine keeps, and one is
worth saving whether or not the server it was written against is still open, so the button is alive on
a tab with no session and dead only on an empty buffer.

`New Snippet` in the pane opens the same dialog empty, tagged with the engine the reader is in front
of. **New and Edit are one dialog**, told apart by whether it carries a name it was known by: they ask
the same four questions, and a save under a name the library already holds replaces that entry either
way, so a second form with a different verb on the button would be the same form.

**There is no `Save as Snippet` in the History pane**, and deliberately not: a double-click there
already puts the statement in the editor, and `Save Snippet` is then the next button along. A second
route to one dialog would be a second thing to keep in step for one press saved.

## Failing

| Shown | When |
|---|---|
| `INVALID_NAME` under the Name box | The name came through blank. A name is what identifies a snippet and what is typed to reach one, so it is the one thing required |
| `EMPTY_SNIPPET` under the Statement box | The body came through blank. It is [its own code](../errors.md) rather than `EMPTY_STATEMENT` because nothing was being run — a library entry with no body is a name the reader would reach for and get nothing back from |
| `INVALID_PROFILE` in the banner | The engine is not one this app knows, which is only reachable by hand-editing the file |
| A strip under the grid | The library could not be read. Whatever was drawn stays drawn, exactly as the [table list](schema-tree.md) does with the same failure |

A **delete is confirmed** — `Forget “name”?` — and says in as many words that nothing on any server
changes, because that is the question a reader asks of a dialog with a red button on it. A name the
library has not got is not a failure: what was asked for is already true, so two windows deleting the
same snippet do not race into an error.
