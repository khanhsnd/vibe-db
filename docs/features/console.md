# The console, and running a file of statements

[← Features and UI behaviour](../features.md)

## The console

**`Console` on a connection opens a tab that is driven one statement at a time.** It is the prompt a
reader reaches for when they know what they want to type: a transcript of everything already sent, a
prompt under it, and — below both, behind a divider — the rows of the last statement that returned
any, in the same `ResultPane` a query tab shows.

- **Enter sends. Shift+Enter takes a new line.** That single difference is the whole reason the
  console is not the SQL editor, where `Enter` is a newline and `F5` runs.
- **The caret is already in the prompt** when the console opens, and goes back there the moment a
  statement finishes — the box is disabled while one is in flight, and a reader who has to click
  back into it between lines is not at a console. It opens **four lines tall and grows to twelve**
  with what is typed, then scrolls rather than eating the transcript above it.
- **The arrows walk back through what has been sent**, from the ends of the prompt only, so they
  still move the caret through a line written over several. The transcript *is* that history —
  nothing is kept twice — and a recalled line that has been edited stays edited.
- **The transcript is one line per statement**, not a grid: how many rows it returned, or how many
  it changed, and how long it took — or the server's own refusal, in the danger colour. The rows
  themselves are in the grid below, which is the one thing in this app that draws rows and columns.
- **It runs through the same store a query tab does**, so a console's rows are paged, sorted,
  exported, edited and stopped exactly as any other result is, and `Stop` is the same button. A line
  reads as many rows as a fresh query tab would — the Options row limit — because a prompt has no
  toolbar to put a Limit control on.
- The **database picker on its strip** is the query tab's, so a console can be pointed elsewhere
  without being reopened; picking a database drops the schema with it, as everywhere else.
- It has **no editor**: no completion, no formatting, no review of a whole script before it runs. A
  reader with a script to think about wants a query tab, and the two are one keystroke apart.
- `Clear` empties the transcript. It is one console per connection *and namespace*, like every other
  tab, so asking for one twice reveals the one already open.

## Execute SQL File

**`Execute SQL File…` runs a file of statements against the connection and says what each of them
did.** A dump of a hundred thousand statements is not something to open in a buffer, review and
format — it is something to point at a database and watch — so this is a dialog rather than a tab,
and **no rows come back at all**. On MongoDB it is `Execute Script File…`, a document store holding
no SQL: the noun is `engineWords.scriptFile`, like every other word the engine owns.

The dialog asks four things and reports one:

- **Database** (and schema, where the engine has them), which the namespace the tree has selected on
  that connection is what it opens on.
- **File**, picked in the host's own dialog through `Browse`, or typed.
- **Encoding**, which is how the file's bytes are read as text: UTF-8, UTF-16 in either byte order,
  Windows-1252, Windows-1258 and ISO-8859-1. UTF-8 **strips a byte order mark**, which nearly every
  file written by a Windows tool carries and which would otherwise become part of the first
  statement's first keyword. A file that is not the encoding it was read as fails as
  `FILE_UNREADABLE` rather than running mangled statements.
- **Continue on error**, on by default: the rest of the file still runs after a statement the server
  refuses. Turned off, the run stops where it stopped, which is what a file whose statements depend
  on each other wants. Either way **the reader's own `Stop` ends it** — that is about the run and not
  about a statement, so `Continue on error` does not override it.

The **log is a transcript, not a table**, and reads the way a run reads — the statement, then what it
came to, tagged down the left:

```
[SQL] Started application_activities.sql
[SQL] CREATE TABLE "public"."application_activities" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "applicationId" uuid NOT NULL,
        PRIMARY KEY ("id")
      )
[ERR] ERROR:  relation "application_activities" already exists
[SQL] CREATE INDEX "IDX_application_activities_applicationId" ON …
[OK]  1 ms
[SQL] Finished with 1 error · 2 statements run · 9 ms
```

**Cells were the wrong shape for it.** A statement is laid out over as many lines as its author wrote
it over and an error is a paragraph the server wrote; as two columns of four those are the two widest,
both clipped to a couple of hundred pixels, and the one thing a reader opened the dialog to read is
the one thing they cannot see. `ResultGrid` is still the only thing in the app that draws rows and
columns — see [grid.md](../grid.md) — and this simply is not rows and columns, exactly as
[the console's transcript](#the-console) is not.

- **A statement keeps its own line breaks.** Every line hangs from a six-character tag, measured in
  `ch` so it holds at whatever size the reader set the data font to, which is what keeps a wrapped
  line — and a statement's own second line — under the statement rather than under the tag.
- **`[ERR]` carries the server's wording alone**, without the wire code beside it. The code is what
  the *app* switches on ([errors.md](../errors.md)); next to the wording it is noise, and where the
  server's own answer already opens `ERROR:` it is noise that reads as a stutter.
- **`[OK]` on a statement that changed nothing reports its time alone** — the tag already said it
  succeeded. There is never a row *count*: a file run keeps no rows, so a number there would be a fact
  about the cap rather than about the statement.
- **The whole log is selectable and copies as it reads.** The tag is part of each line's own text
  rather than a span beside it, and the air between statements is padding rather than blank lines, so
  a log lifted into a message arrives laid out the way it was shown and no longer than it looked.
- **The dialog opens at a form's width and widens when the log arrives** — 660 px, then 1 120 px,
  which gives the transcript about 140 characters to a line so a `CREATE TABLE` reads as it was
  written and only the longest single-line `ALTER TABLE` folds. Holding the wide one from the start
  would leave most of the dialog standing empty for the whole time a reader spends filling it in — a
  cost paid every time, against a single widening paid seconds or minutes after `Start`, when the
  reader is looking at the log that has just appeared rather than at the button. The form keeps its own
  620 px inside the wide one: a Database picker stretched to 900 px is harder to read, not easier.
  Nothing is ever *hidden* by a narrower window either way, since the log wraps and never clips.
- **The log is the one scroller on this surface.** Its height is set so the form above it and the log
  together clear an ordinary window without the dialog's own body scrolling as well: two nested
  scrollers is what made the old table unreachable.

Two things Navicat's dialog has are deliberately not here. **Batching several statements per
round-trip** is not, because where a statement ends is the driver's answer and the app runs them one
at a time by that answer; and **`SET AUTOCOMMIT=0`** is not, because nothing below this layer opens a
transaction yet — a checkbox that did nothing would be worse than its absence. Nothing a file runs
reaches the history log either: a log holds statements a reader can put back in an editor, and a file
of ten thousand of them is a file, which is where it already is.
