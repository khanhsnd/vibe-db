# Editor and completion

The editor is CodeMirror 6 (`@codemirror/lang-sql`): line numbers, syntax highlighting off the same
`--syn-*` tokens as the rest of the app, bracket matching, undo history and soft wrapping. Each tab
owns its editor, so the caret and the undo stack do not follow a tab switch.

The dialect follows the connection's engine — `MySQL` or `PostgreSQL` — and rides in the same
compartment as the schema, so it reconfigures in place rather than rebuilding the editor.

**A selection narrows what runs.** Highlight one statement of a script and Execute sends only that;
with nothing selected it sends the whole buffer.

## Laying out and reviewing

Neither is done here. **Format** and **Review** are `QueryAPI` calls onto `sqlkit.Dialect`, for the
reason splitting always was: where a statement ends and what counts as a quote are lexical and
therefore the engine's. What this side does is dispatch the result.

- **`Ctrl`+`Shift`+`F`** formats, bound on the *window* alongside `F5` — the editor does not own the
  keyboard when the reader is in the output pane, and a shortcut that only works with the caret in
  the text is a shortcut readers stop reaching for. What it does to a script, and the rules it
  follows, are in [features.md](features.md#laying-a-script-out).
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

Completion fires as you type and on `Ctrl`+`Space`, over four sources:

- **Tables** of the tab's namespace, with their column count as the hint.
- **Qualified columns**, after `table.` or `alias.` — `FROM orders o … o.` resolves the alias.
- **Bare columns** anywhere else in the statement — after `SELECT`, after `WHERE`, after
  `ORDER BY` — drawn from every table that statement names, each labelled with its table. A join
  offers both sides. This is a source of ours, not lang-sql's, which only completes behind a dot.
- **Keywords** of the dialect, offered upper-case, and **statement snippets** (`select`, `insert`,
  `join`, `group by`, `with`, …). `Tab` walks the snippet's fields.

The last two are suppressed after a dot and inside strings and comments, where only the qualified
schema has anything useful to say.

`ConnectionAPI.LoadTableSchemas` reads a whole namespace's tables and columns in one catalogue scan —
`information_schema.columns` on MySQL, `pg_attribute` on PostgreSQL — cached per
connection-and-namespace until disconnect. Switching the toolbar's database or schema reconfigures
completion in place rather than rebuilding the editor.

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
