# Laying a script out, and what is said about it

[← Features and UI behaviour](../features.md)

## Laying a script out

**Format** rewrites the buffer the way the script reads: one clause to a line, its items indented
under it, keywords in upper case, and one blank line between statements. It is
`QueryAPI.Format` → `sqlkit.Dialect.Format`, and it is the **driver's** for the reason splitting
always was — a semicolon inside a `$body$` ends nothing on PostgreSQL and a backslash escapes a
quote only on MySQL. A tab with nothing dialled cannot format, exactly as it cannot execute.

- **Nothing a server would notice changes.** A string, a quoted identifier and a comment are single
  tokens and are written back byte for byte, so only the whitespace *between* tokens and the case of
  *unquoted* keywords ever move. A test compares the token stream before and against after to keep
  that true.
- **A script the lexer cannot make sense of comes back untouched** — one ending inside a quote or a
  block comment. The one thing worse than an unformatted script is a formatted one that no longer
  runs, and the strip above the editor is already saying what is wrong with it.
- **It is the whole buffer, never the selection**: a statement pulled out of its script,
  re-indented and put back would be indented against nothing.
- **Formatting twice is the same as formatting once.** Every rule that decides a line break counts
  *items* rather than characters, because formatting changes how wide a list is written and a width
  test would break on the second pass what it left alone on the first.
- A parenthesis is punctuation or a block, and three things make it a block: it holds a statement,
  it is the field list of a `CREATE` or an `ALTER`, or it holds more than a line's worth of items.
  **A function call is none of them** — and what tells `COUNT(*)` from `INSERT INTO users (id, name)`
  is the reader's own spacing, which is also what decides the space in front of it. No rule this
  side can tell a call from a field list better than the person who wrote it.

## What the editor says before anything runs

`QueryAPI.Review` reads what the script is going to do, and what is already wrong with it, without
sending any of it anywhere. One call answers both, so what is underlined in the text and what the
guard dialog lists cannot disagree. It runs on a **250 ms debounce** after a keystroke — the call is
local, so that is pacing the render and not the network.

**It is a lexer and not a parser, deliberately.** Everything it reports it can prove: a quote, a
comment or a `$tag$` body the script ends inside of, and a bracket with no partner. Whether a column
exists, whether a clause is in the right order, whether a type will take that value — those are the
server's answers to give, and a client that guessed at them would be wrong often enough that a
reader learned to ignore it, which costs more than the warning saved.

- **On MongoDB it is a parser as well**, that grammar being small enough to read whole: a command this
  side cannot make sense of will not run either, so it is reported. **A command that is merely
  unfinished is not.** Half of `db.users.f` is what every command looks like on the way to being
  written, and a squiggle that sits under the text the whole time it is being typed is one the reader
  stops seeing. Nothing is lost by keeping quiet, because a command truncated for real is a bracket or
  a quote with no partner and that is already reported.
- **The text itself is where a fault is located**: a wavy `--danger` underline under what is wrong,
  and a flat `--warn` one under the **first line** of a statement that will have to be confirmed. A
  twenty-line `DELETE` underlined end to end is a wall of colour that says nothing more and makes
  the statement harder to read at the moment the reader is checking it.
- **The strip between the toolbar and the editor is one line each and never grows**: the first
  fault, and a count of the statements that change data. Clicking either puts the caret on it. A
  list of twenty repeated messages would push the editor off the screen exactly when the reader
  wants to look at it.
- The offsets a review carries are **byte** offsets, because Go counts bytes; the editor translates
  them before they are positions in the document. Without that every mark after the first
  non-ASCII character — a Vietnamese column name, a comment, a string — would land short of its text.

## Every statement run is kept

**Execute writes what it ran into a log on disk**, and the `History` pane is that log — see
[the history pane](results.md#the-history-pane). It is kept whatever became of the statement, a
rejected one being among the likeliest to be wanted back, and it is written after the run rather
than before, so what is remembered is what actually happened. A log that cannot be written is dropped rather than
reported: it is not a failure the reader can act on, and certainly not one the query they just ran
should be reported as.

- **The log is one entry per statement, not per Execute.** A statement run again is *moved* to the
  head with its `Runs` counted up, rather than added beside itself — five hundred copies of one
  `SELECT` is not a list anybody reads, and what a reader wants back is the statement. Two entries
  are one when the text, the connection and the namespace all match: the same text against another
  database is another statement, that being where the tables it names live.
- **It holds 500 statements**, oldest dropped first, and lives in `history.json` beside
  `connections.json` in the reader's own configuration directory. The rule for what a log holds
  after one more statement is `domain.Remember`'s and is written down once.
- **The whole of what the reader executed is one entry**, script or single statement. What they
  pressed Execute on is what they want back, and cutting a twenty-statement script into twenty
  entries would bury the nineteen runs before it.
- The outcome kept is the **last** run's: the rows the grid ended up with, how long the whole
  Execute took, and the wording of the statement it stopped at.
