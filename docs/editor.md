# Editor and completion

The editor is CodeMirror 6: line numbers, syntax highlighting off the same `--syn-*` tokens as the
rest of the app, bracket matching, undo history and soft wrapping. Each tab owns its editor, so the
caret and the undo stack do not follow a tab switch.

**The language follows the connection's engine**, through `queryLanguage`, and rides in the same
compartment as the schema so it reconfigures in place rather than rebuilding the editor. There are
three of them, because five of the seven engines write the same language and the other two write
nothing like it or like each other:

- **SQL** (`sqlLanguage`, `@codemirror/lang-sql`) for the five SQL engines, whose dialects differ only
  in a quoting rule and a keyword list — one `SQLDialect` each, so a bracket is an identifier on SQL
  Server and something else everywhere else. The **quoter** differs too, and only where an engine
  folds an unquoted name: PostgreSQL lowers one and Oracle raises one, so completion inserts the form
  that will run rather than the form the catalogue reported.
- **The shell** (`shellLanguage`, `@codemirror/lang-javascript`) for MongoDB. JavaScript to
  *highlight* — a command is `db.users.find({…})` and its argument is a document, so a brace is a
  brace and a string is a string — and nothing like JavaScript to *run*: the driver reads one command
  and its arguments, with no variables and no control flow. See
  [architecture.md](architecture.md#the-document-store).
- **Commands** (`commandLanguage`) for Redis. A command is **argv** — a verb and then bytes — so
  there is no grammar to parse and nothing to indent, and what colouring can honestly say is which
  word is the verb (and the second word of `CONFIG SET`, which is half of one), which run is quoted,
  which is a number, and where a `#` comment starts. Everything else is a key or a value the reader
  chose, and painting it would be inventing a meaning the server does not give it. Completion offers
  the commands with a line on what each does, and a container command's second word after its first.

`queryLanguage` is the whole of what the editor knows about which engine it is in front of, down to
the empty tab's own prompt — `SELECT * FROM ...` is not what a document store is waiting for, and
neither is what a key-value store is.

**A document store has no keywords, so what carries the colour is the shape of a command.** Most of
it comes off the tags lang-javascript already hands back, resolved against the same `--syn-*` tokens
SQL uses: `db` is the only bare name a command ever begins with, `find` and `ObjectId` are calls
whatever they are called on, a string is a string and a number is a number. **An operator is the one
thing a grammar cannot give** — to JavaScript `$gt` is a key like any other, and to the reader it is
the whole of what the filter says — so `shellLanguage.ts` marks a `$`-prefixed key as the keyword it
is. It reads the **parse tree** and never the text, which is what keeps `"$total"` a string and
`// $set` a comment, and is why it is not a second scanner.

**A selection narrows what runs.** Highlight one statement of a script and Execute sends only that;
with nothing selected it sends the whole buffer.

## Laying out and reviewing

Neither is done here. **Format** and **Review** are `QueryAPI` calls onto the driver's `Text`, for
the reason splitting always was: where a statement ends and what counts as a quote are lexical and
therefore the engine's. What this side does is dispatch the result — and it does not care which
grammar answered, because a hazard, a confirmation and a squiggle are the same three things whether
the statement was a `DELETE` or a `deleteMany`.

- **`Ctrl`+`Shift`+`F`** formats, bound on the *window* alongside `F5` — the editor does not own the
  keyboard when the reader is in the output pane, and a shortcut that only works with the caret in
  the text is a shortcut readers stop reaching for. What it does to a script, and the rules it
  follows, are in [features/script.md](features/script.md#laying-a-script-out).
- **`diagnostics.ts`** is the marking: one `StateField` holding a `DecorationSet`, updated by a
  `StateEffect` when a fresh review lands and **mapped through every change between** — a squiggle a
  character behind the typing is worse than none. It is a field rather than a compartment because
  marks change on nearly every keystroke, and reconfiguring the editor that often throws away work
  the other extensions are holding.
- **Offsets are translated before they are positions.** Go counts a string in bytes and CodeMirror
  counts it in UTF-16 units, so `position()` maps one to the other; a script that is entirely ASCII
  skips the scan, which is the common case. Without it every mark after the first non-ASCII
  character lands short of its text.
- The editor answers exactly one question from outside — `EditorHandle.reveal(offset)`, which puts
  the caret on a byte offset and scrolls to it. That is what makes the problems strip clickable, and
  it is the whole of the imperative surface.

## Completion

Completion fires as you type and on `Ctrl`+`Space`. On SQL it has four sources:

- **Tables** of the tab's namespace, with their column count as the hint, and the **sibling
  namespaces** beside it by name.
- **Qualified columns**, after `table.` or `alias.` — `FROM orders o … o.` resolves the alias. A
  sibling namespace is one more level of the same thing: `warehouse.` offers its tables,
  `warehouse.stock.` offers their columns, and `FROM warehouse.stock s … s.` resolves through both.
- **Bare columns** anywhere else in the statement — after `SELECT`, after `WHERE`, after
  `ORDER BY` — drawn from every table that statement names, each labelled with its table. A join
  offers both sides, whichever namespace each side is in. This is a source of ours, not lang-sql's,
  which only completes behind a dot.
- **Keywords** of the dialect, offered upper-case, and **statement snippets** (`select`, `insert`,
  `join`, `group by`, `with`, …). `Tab` walks the snippet's fields.

The last two are suppressed after a dot and inside strings and comments, where only the qualified
schema has anything useful to say.

`ConnectionAPI.LoadTableSchemas` reads a whole namespace's tables and columns in one catalogue scan —
`information_schema.columns` on MySQL, `pg_attribute` on PostgreSQL — cached per
connection-and-namespace until disconnect. Switching the toolbar's database or schema reconfigures
completion in place rather than rebuilding the editor.

### Joining across namespaces

A join whose two tables live apart is an ordinary statement — a qualified name is all it takes — so
completion holds more than one namespace at a time, and `useQuerySchema` is what decides which.

- A **sibling** is whatever varies below a connection: a *database* where a table is qualified by
  one (MySQL, Oracle, SQLite), a *schema* where it is qualified by that instead (PostgreSQL, SQL
  Server). A document store and a key space qualify nothing, so neither has siblings.
- Every sibling is **offered by name** as soon as the connection knows it is there, which is what
  makes the list a way of finding out what exists.
- A sibling is only **read** once the script names it: each read is a catalogue scan and a server
  can hold hundreds of databases, so reading all of them to open an empty tab is a cost nobody
  asked for. The trigger is the name appearing **anywhere in the buffer**, not a dot after it —
  which is what puts the tables in hand before the dot is typed.
- A **three-part** name is not completed. SQL Server's `other_db.dbo.Orders` runs, but the schemas
  of a database nothing is pointed at have never been listed, so there is nothing to offer under
  them.

`sqlLanguage` hands lang-sql **one** namespace whose keys are the sibling names, because lang-sql
nests a namespace by splitting a key on the dot and resolves an alias through however many levels
it finds — so nothing here walks a path or resolves an alias itself. A table of the tab's own
namespace still wins the bare name, since that is what an unqualified name means, and is reachable
qualified as well.

**Completion inserts the form that will actually run.** PostgreSQL folds an unquoted identifier to
lower case, so a table created as `AIFolders` reaches the server as `aifolders` and only resolves
when it is quoted. `quoterFor` in `sqlLanguage.ts` therefore gives a PostgreSQL completion an
`apply` of `"AIFolders"` while the label stays readable; a name that is already lower-case and
plain is offered bare. MySQL folds nothing, so its names go in as they are stored.

Typing such a name by hand still fails, and it is the server that says so — which is why a mapped
SQLSTATE keeps the server's own wording in the error's `detail`. `relation "aifolders" does not
exist` names the folded spelling, and that is the entire clue.

One upstream limitation is worth knowing: a table whose name is a reserved keyword — `user`,
`plugin` — is tokenised as a keyword, so `FROM user u` does not register the alias `u` and `u.`
falls back to keyword matches. Quoting it (`FROM \`user\` u`, or `FROM "user" u` on PostgreSQL)
restores column completion. Bare column completion is unaffected, because it matches words against
the schema rather than trusting the parser's token types, and it reads both quotings.

## Two colours that are load-bearing, not cosmetic

- `--selection` is applied through
  `&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground`. CodeMirror's own
  focused-selection rule is five classes deep, so the obvious `&.cm-focused .cm-selectionBackground`
  loses the cascade and the library's default lavender wins in both themes.
- `--line-highlight` (the active line) **must stay translucent**. The selection layer sits at
  `z-index: -2`, behind the lines, so an opaque active-line background paints over the selection.

**`editorTheme` has a second reader.** The value viewer's laid-out view is CodeMirror too —
`DocumentView`, see [features/value-viewer.md](features/value-viewer.md) — and wears this same theme, so
one editor and one laid-out document cannot drift apart. Its own token, `propertyName`, is the one
the shared syntax style deliberately leaves unset: the shell's documents keep their keys the colour
of the data they name, while a document being *read* is nothing but keys and values.

That view reads a document store's notation with **this same JavaScript parser under its
`SingleExpression` top rule**, and the difference is worth knowing. In the editor a document is
always inside a call — `find({ … })` — so its brace is an object in expression position. On its own
it is not: read as a script, a bare `{ a: 1, b: 2 }` is a *block*, `a` is a label and everything
after the first comma is a syntax error. `SingleExpression` is the parser's own top rule for exactly
this case.
