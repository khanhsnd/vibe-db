# Editor and completion

The editor is CodeMirror 6 with the MySQL dialect (`@codemirror/lang-sql`): line numbers, syntax
highlighting off the same `--syn-*` tokens as the rest of the app, bracket matching, undo history
and soft wrapping. Each tab owns its editor, so the caret and the undo stack do not follow a tab
switch.

**A selection narrows what runs.** Highlight one statement of a script and Execute sends only that;
with nothing selected it sends the whole buffer.

## Completion

Completion fires as you type and on `Ctrl`+`Space`, over four sources:

- **Tables** of the tab's database, with their column count as the hint.
- **Qualified columns**, after `table.` or `alias.` — `FROM orders o … o.` resolves the alias.
- **Bare columns** anywhere else in the statement — after `SELECT`, after `WHERE`, after
  `ORDER BY` — drawn from every table that statement names, each labelled with its table. A join
  offers both sides. This is a source of ours, not lang-sql's, which only completes behind a dot.
- **Keywords** of the dialect, offered upper-case, and **statement snippets** (`select`, `insert`,
  `join`, `group by`, `with`, …). `Tab` walks the snippet's fields.

The last two are suppressed after a dot and inside strings and comments, where only the qualified
schema has anything useful to say.

`ConnectionAPI.LoadSchema` reads a whole database's tables and columns in one
`information_schema.columns` scan, cached per connection-and-database until disconnect. Switching
the toolbar's database reconfigures completion in place rather than rebuilding the editor.

One upstream limitation is worth knowing: a table whose name is a reserved MySQL keyword — `user`,
`plugin` — is tokenised as a keyword, so `FROM user u` does not register the alias `u` and `u.`
falls back to keyword matches. Quoting it (`FROM \`user\` u`) restores column completion. Bare
column completion is unaffected, because it matches words against the schema rather than trusting
the parser's token types.

## Two colours that are load-bearing, not cosmetic

- `--selection` is applied through
  `&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground`. CodeMirror's own
  focused-selection rule is five classes deep, so the obvious `&.cm-focused .cm-selectionBackground`
  loses the cascade and the library's default lavender wins in both themes.
- `--line-highlight` (the active line) **must stay translucent**. The selection layer sits at
  `z-index: -2`, behind the lines, so an opaque active-line background paints over the selection.
