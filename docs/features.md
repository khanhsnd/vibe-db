# Features and UI behaviour

Working: connection profiles (create / edit / delete) for **MySQL and PostgreSQL**, **over an SSH
tunnel, with TLS held to a certificate this machine trusts, and signing in by AWS or Google cloud
identity as well as by password**, Test Connection
with server version and latency, connect / disconnect, **an environment label that colours a
connection everywhere and stands between the reader and a statement they cannot take back**,
**laying a script out and warning about it before it runs**, **a value viewer that reads a cell as
the JSON, GeoJSON, array or blob it holds**, **creating and dropping a database**,
**tables, views and routines listed per namespace**, **a table list of what the catalogue keeps about
each table**, **a table's fields, indexes and foreign keys in
the tree and in a designer tab**, **schema changes from the tree's context menu** — new / rename /
empty / truncate / drop a table, add / edit / rename / drop a field, add or drop an index or a
foreign key, drop a view or a routine — table data browsing with pagination, cell editing plus row
insert and delete on any grid whose rows can be traced to one table, **exporting a grid to CSV, JSON
or SQL**, query tabs with completion that **run a whole script**, **a console that runs one statement
at a time and keeps the transcript**, **running a file of statements against a connection**,
**marking a connection with a colour that washes down the sidebar**, **a history of every statement
run, kept on disk and put back in the editor by a double-click**, passwords in Windows Credential
Manager, and the full layout shell with dark and light themes and a **data font the reader picks**.

Not there yet: a **materialised view** is not listed, because it is dropped by a statement of its
own and a list that cannot be acted on is worse than one that does not claim it. A routine can be
dropped but not read or written — there is no body editor. A column's **comment** is neither read
nor written, so the designer has no column for one rather than a column that quietly drops what is
typed into it. An export writes what the grid holds rather than replaying the statement for
everything behind it.

## Picking an engine

**A new connection picks its type before it is filled in.** The dialog opens on a grid of engine
tiles — each engine's own mark at a size that reads by shape — and `Next` moves on to the form; a
double-click on a tile does both. `Back` in the form returns to the grid, which is also how an
existing connection changes engine: editing one opens straight on the form, since it already has a
type.

Everything the form defaults to comes from that choice: port 3306 and user `root` for MySQL, 5432
and `postgres` for PostgreSQL, 27017 and **no user at all** for MongoDB — a server started without
access control takes no credentials, and a name filled in for it would be a name the reader had to
clear. Going back and choosing another engine rewrites only the fields that still hold the old
engine's default, so a typed port survives the switch. The editor loads that engine's own language
for highlighting and completion — SQL for two of them, the shell for MongoDB; see
[editor.md](editor.md).

**Every connection wears its engine's chip** — the dolphin on teal for MySQL, the elephant on blue
for PostgreSQL, the leaf on green for MongoDB — in the tree, in the dialog's header and in the status
bar. It replaces the engine badge the tree used to carry on the right of a connection row, which is
where the **connection status dot** now sits: what the connection *is* on the left, how it is *doing*
on the right.

Two things differ rather than just defaulting:

- **PostgreSQL needs a database.** A session is bound to one for life, so that is the database the
  connection bootstraps from — the one its database list is read in. Reaching any other database dials
  it separately, which the adapter does on first use.
- **MongoDB needs none.** One connection reads every database on the server, so a database named here
  is a *starting point* and nothing more: it is where a command carrying no namespace goes. TLS is on
  or it is not — the driver has no opportunistic mode — so `Preferred` connects in the clear, and the
  two cloud identity methods are refused outright rather than sent as a password no MongoDB
  authentication mechanism would take.

## Filling a connection in

The form is three tabs, because the three answer different questions and only the first is asked
every time: **General** is where the server is, **Security** is what it is held to, and **SSH** is
what carries the connection there. Every tab is the same height, so switching one never moves the
footer under it.

### Security

- **Authentication** is a password, **AWS IAM** or **Google Cloud IAM**. The two cloud methods have
  nothing to type and nothing to save: the password is a token minted *per connection* from an
  identity the machine already holds, and one good for a quarter of an hour is not a secret worth
  keeping — so the Password box and `Save password` are both switched off for them, and the
  connection never prompts. What each one needs is written under the picker rather than in a manual:
  | Method | What the machine has to hold |
  |---|---|
  | AWS IAM | `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` in the environment, plus `AWS_SESSION_TOKEN` for temporary credentials. A reader on SSO or a named profile exports them with `aws configure export-credentials --format env`. The **region** is read off an RDS endpoint — `name.hash.region.rds.amazonaws.com` — and only has to be typed for a host that is not one, such as a proxy or a CNAME. The token is a SigV4-signed request to connect, so it **must** go over TLS: a signature anybody can read is a signature anybody can replay |
  | Google Cloud IAM | The application default credentials — `gcloud auth application-default login`, or `GOOGLE_APPLICATION_CREDENTIALS` pointing at a service-account key file. Both routes end at Google's token endpoint, one by refreshing and one by signing an assertion, and the access token that comes back is the password |
- **TLS** has five modes, and the two above `Required` are the only two that hold the server to
  anything: `Disabled` is plain text, `Preferred` takes TLS where the server offers it,
  `Required` encrypts and checks nothing, `Verify CA` checks the certificate chain but not the host
  name, and `Verify Full` checks both. The middle one is not a weaker verification — it is
  encryption without any — and the labels say so.
- **The certificate files are files on this machine** and are only ever read here: a root
  certificate to check the chain against (blank uses the machine's own trust store), and a client
  certificate and key to present. Naming one under a mode that never opens it is **refused** rather
  than ignored, because a file the app quietly drops is a reader believing they have configured
  something. A **certificate name** holds the server to a name other than the one dialled, which is
  what a tunnel makes ordinary — the connection goes to a local port and the certificate is still
  the database server's.
- **Every path box has a `Browse` beside it and stays writable** — the two together, deliberately.
  A reader with the file in front of them picks it; a reader who already has the path on the
  clipboard — out of `gcloud`, out of a download, out of a colleague's message — pastes it, which is
  as common and would be worse served by a dialog. The picker offers certificates
  (`*.pem;*.crt;*.cer;*.der`) or keys (`*.pem;*.key`) first and All files behind them, and the two
  boxes that offer no pattern at all are the SSH private key and `known_hosts`, whose files usually
  carry no extension. It is `SystemAPI.ChooseFile`, the same API the font list comes from, because it
  asks the *machine* and not a server — these boxes are filled in before there is a connection to
  fill them for. **Nothing is read**: the path is saved on the profile and the file is opened at
  connect time, which is also where a wrong one is reported.

### SSH

A connection through a bastion dials the bastion first and the database *from* it. **The host and
port on the General tab are resolved on the far side**, so a server bound to loopback on the bastion
is reached by naming `127.0.0.1` — the loopback that matters is the bastion's. Nothing is listened
on locally: there is no forwarded port to collide with anything, and nothing else on this machine
can reach the tunnel.

- **Three authentications**: a private key with a pass phrase, a password, or the **SSH agent** —
  which is the only one of the three where this app never holds the key or the phrase, since the
  agent does the signing.
- **The host key is checked against `known_hosts` by default**, and a bastion that is not in it is
  refused rather than trusted. Turning that off is a deliberate act for a host that cannot be
  enrolled: a tunnel whose far end is not identified protects the credentials it carries from
  nobody, which is the one thing it exists to do.
- **Which key is asked for is decided by what the file already holds.** A server offers several —
  RSA, ECDSA, Ed25519 — and hands over whichever the client asks for first, so a client asking in
  its own order gets a key `known_hosts` has never seen and reports a mismatch for a host whose key
  has not changed. That is the `knownhosts: key mismatch` a reader sees on a host that connects
  perfectly well from `ssh` or from another client; see [decisions.md](decisions.md).
- The pass phrase or password lives in the credential store beside the connection's own, under the
  profile's id with a `:ssh` suffix — one profile, two entries, both forgotten when it is deleted.
  Neither ever comes back out: the dialog is told *that* one is stored, never what it is.
- A tunnelled connection is given the ordinary connect timeout **plus** a tunnel's own, because it
  has two hops to make and cutting the second off for the first taking its time would report a
  working server as unreachable.

## Which environment a connection is for

**A connection says what it is for, and every surface that names it says so too.** The environment
is picked in the dialog — none, Development, Staging or Production — and it is the one fact about a
profile that changes how the app *behaves* rather than how it connects.

- The colours are a traffic light on purpose: what green, amber and red already mean is the whole
  reason the label is worth drawing. They are `--env-*` tokens in `shared/theme/tokens.css` and
  nothing else draws an environment — `features/connections/EnvironmentDot.tsx` has all three
  shapes, so adding an environment is a row in `ENVIRONMENTS` and nothing else.
- **A stripe** down the leading edge of the connection's row in the tree and of every tab on that
  connection; **a badge** in the status bar and in the dialog that asks about a dangerous statement.
  A connection naming no environment draws no stripe and no badge, and the picker still shows the
  muted dot — a gap where the other rows have a mark reads as a fault rather than as an absence.
- **Only the connection row is striped**, not everything under it: a database, a table and a field
  all belong to that connection and inherit the warning by sitting beneath it.

### What gets confirmed before it runs

**Production is the only guarded environment.** A warning a reader meets everywhere is one they stop
reading, and that habit is exactly what this exists to prevent. Two things are therefore asked about,
for two different reasons:

| Confirmed | Where | Because |
|---|---|---|
| Anything that writes — `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `TRUNCATE`, `ALTER` | On a **Production** connection | The cost of being on the wrong connection is the whole point of labelling one |
| An `UPDATE` or a `DELETE` carrying **no `WHERE`** | Anywhere at all | `DELETE FROM users` is a mistake on a laptop as surely as it is in production |

- **The rule lives in `domain.Environment.NeedsConfirmation`**, and the *answer* crosses the wire as
  `confirm` on each statement. Nothing in the frontend re-derives what is dangerous: the guard
  dialog lists exactly the statements the Go side flagged, and the editor underlines the same ones.
- **What is asked about is reviewed at the moment Execute is pressed**, not read off the strip above
  the editor — the strip follows the whole buffer on a debounce, and what runs may be a selection
  made since.
- **Explain is guarded exactly as Execute is**, because Explain *runs* the statement and reads a plan
  about it — a `DELETE` explained is a `DELETE` done. The guard is what says so before it happens, and
  what is confirmed is the text that was reviewed, however the selection has moved since.
- A review that cannot be had is not a reason to refuse to run. The reader asked, and the server is
  still what decides; it is the guard that is missing, not the query.
- The confirming button is drawn `danger` rather than `primary`. The accent button is the one a
  reader presses without reading it.

## Browsing a schema

The tree has as many levels, and as many folders, as the engine does:

```
MySQL         connection → database →          Tables       → table      → Fields       → field
PostgreSQL    connection → database → schema →  Views        → view              Indexes      → index
                                                Functions    → function          Foreign Keys → foreign key

MongoDB       connection → database →          Collections  → collection → Fields  → field
                                                Views        → view              Indexes → index
```

**A folder for something the engine has not got is not drawn.** MongoDB has no stored routines —
server-side JavaScript was removed from the server — and no referential constraints of any kind, so
neither folder appears rather than appearing permanently empty. `EngineInfo.hasRoutines` and
`hasForeignKeys` are what say so, and a component never compares against an engine's name.

**What a thing is called is the engine's too.** A collection is not a table and a document is not a
row, so `engineWords` supplies the nouns for the tree's folder, the right-click menu, the object list
and every confirmation. Only the wording changes: a `NodeKind` is still `tables` and the bound method
is still `DropTable`, so one name means one thing on the wire whatever it says on the screen. The
warning on **Truncate** is why it matters — on a table store it warns about a foreign key, and a
document store cannot have one, which is also why MongoDB is offered `Empty` alone: there is no
storage to reset, so truncating and emptying would be one command under two names.

Each level loads on first expand and is cached until disconnect: a PostgreSQL database loads its
schemas, whichever node owns the object folders loads **its tables, views and routines in one
read**, and a table loads its shape — fields, indexes and foreign keys, also in one read. The reason
is the same both times: expanding any one folder needs every folder's count, so one read answers all
three — which is why that read happens on **the namespace's own expand** and not on its folders':
the three folders arrive with their counts already on them, and there is no second click that only
fills in numbers. Clicking the node that owns tables — a MySQL database, a PostgreSQL schema, a
MongoDB database — marks it the active namespace for that connection, which is what a new query tab
inherits, and the row is drawn in the success colour to say so.

**A MongoDB field is inferred, not declared.** A collection says nothing about what it holds, so the
Fields folder is the fields the *documents* were found to carry — a sample of the collection's first
few hundred, `_id` always among them. What that costs, and where it shows, is in
[grid.md](grid.md#where-the-columns-come-from).

A **view opens its rows in the same tab a table does**, and lands there read-only: no column of a
view is a primary key, so no single stored row can be named. A **function or a procedure** carries
its argument list as its badge, because on PostgreSQL two of them may share a name and be told apart
only by what they take — which is also why dropping one sends the whole routine and not its name.

A field carries its **bare** type as a badge and a key icon when it is part of the primary key; an
index carries its columns, and a foreign key the table it points at. The badge is the bare type and
not the declared one because six levels of indent leave no room for `character varying(255)` — the
whole of it is in the row's tooltip, and in the designer.

**One click selects, two clicks open** — see [frontend.md](frontend.md). Applied to the tree:

| Gesture | What happens |
|---|---|
| Click a row | Picks it **alone**, and names the active namespace if that row owns tables. Reads nothing — except on the **`Tables` folder**, which also opens [the table list](#the-table-list): what that folder *is* to a reader is the tables and their numbers, and the tree has room for the names alone. |
| **`Ctrl`/`Cmd`-click a row** | Picks it **beside** whatever is picked already, or takes it back out. It reads nothing and travels nowhere: picking a second table is not opening it. Two tables picked this way is the whole precondition for [Compare](#comparing-two-tables). |
| Click the caret | Picks the node **and** opens or closes it, loading that level on first open. |
| Double-click a row | The same open — except on a **table**, which opens a data tab. A table has folders under it now, but double-clicking one has always meant "show me the rows", and the caret is what expands. |
| Right-click a row | The menu, on a pick it left alone if it landed **inside** one of several — so a pair can be acted on whole — and on that row alone otherwise. It is the rule the grid's own cell menu already sets. |
| Open a closed connection | Dials it, whichever gesture asked. A disconnected connection node still carries an empty child list, so expanding it would otherwise spin the caret over nothing. The store expands the node when the session lands, and a failed connect leaves the caret shut. |

**Every picked row is washed, and the focused one is bold** — the two depths the grid's own selection
has, and for the same reason: everything that acts on one row acts on the last press, and Compare
acts on two. `Ctrl` **toggles** here where the grid's only ever adds, because a rectangle cannot be
split and a list of rows can — taking the wrong table back out of a pair is the commonest correction
there is.

### Filtering the tree

**Filter objects** narrows the tree to the rows whose name holds what was typed, case-insensitively,
and **opens every folder above a hit** so the hit is on the screen rather than behind a caret.
`searchTree` is the whole of it, and three things follow from where it sits:

- **A folder never matches.** `Tables` is under every namespace, so a filter that matched it would
  answer every query with the whole tree. Only objects match — a connection, a database, a schema, a
  table, a view, a routine, a field, an index, a foreign key.
- **A hit keeps its own children shut.** The reader asked for that row, not for everything under it.
  Their own expansion is untouched by the search and is what is left when the box is emptied —
  `Escape`, or the cross in the box, empties it.
- **Only what has been read is searched**, the tree being filled a level at a time. A namespace
  nobody has opened holds no names yet, so nothing in it can be found; the empty state says exactly
  that rather than claiming the table does not exist.

## What a right-click on a connection offers

The connection's own menu is the one place everything about a *server* is asked for, and it is
ordered by what a reader is there to do rather than by what the app can do:

| | |
|---|---|
| `Connect` / `Disconnect` | Whichever it is not already. |
| `Refresh` | Re-reads the database list, and forgets everything under a database the server no longer carries — schemas, tables, shapes. |
| `Console` | A [console](#the-console) on this connection. |
| `Execute SQL File…` | [Runs a file](#execute-sql-file) of statements against it. |
| `New Database…` | The same dialog the database level offers. |
| `Colour` ▸ | Marks the connection — see below. |
| `Edit Connection…` | The profile dialog, opened on the form rather than the engine grid. |
| `Delete Connection` | The profile and both of its secrets. |

**Everything needing a live server is left out while the connection is shut**, not greyed out: a
closed connection cannot be asked anything at all, and a column of dead items says only that. So a
shut connection offers four items and an open one eight.

The rows themselves are tighter than a form's — 12 px labels, one line of padding — because a menu
is read by scanning a column of short labels, and fourteen of them at a dialog's spacing runs off
the bottom of the window.

### Marking a connection with a colour

**`Colour` washes the connection and everything under it** in one of eight, and the whole subtree
takes it: a database of the blue connection is the blue connection's database. It is what tells two
connections apart when the names are alike and the environments are the same — a staging replica and
the staging primary, four copies of one schema on one host.

- It is a **submenu of swatches**, and the one in force wears a ring. `No Colour` is drawn as an
  empty ring rather than a grey dot, which would read as a ninth colour.
- **It means nothing.** That is the difference between it and the environment stripe beside it: an
  environment is a fact the app acts on — production guards what runs on it — and a colour is the
  reader's own filing. Nothing behaves differently because of one.
- The wash sits **behind the row's text and in front of its background**, so hovering and selecting
  still read through it. How strong it is comes from one token, `--conn-tint`, and it is lighter on
  the light theme, where the same wash over white reads as a stronger colour.
- It is saved on the profile like any other field — `colour` in `connections.json` — so it survives
  a restart, and it is the one field of a profile no server ever hears about.

## The table list

Clicking the `Tables` folder opens a tab of **every table of that namespace with what the
catalogue keeps about it** — one row per table, in a `ResultGrid` like everything else that shows
rows and columns. Over it sit `Open Table`, `Design Table`, `New Table` and `Delete Table`; under
it the same `GridFooter` every grid has, with `Refresh` and the table count.

- **The columns are the engine's own**, and only the first is promised: the table's name, which is
  what the toolbar acts on. MySQL reports `Auto Increment`, `Modified Date`, `Data Length`,
  `Engine`, `Rows` and `Comment`; PostgreSQL reports `Owner`, `Access Method`, `Data Length`,
  `Rows` and `Comment` — and **no modified date, because the server keeps none**; MongoDB reports
  `Kind`, `Documents`, `Data Size`, `Storage Size`, `Avg Document Size`, `Indexes` and `Index Size`.
  No side of the wire names or formats a value: a size arrives spelled the way that engine spells it
  (`1,552 KB` from MySQL's own arithmetic, `1928 kB` from `pg_size_pretty`, `1.5 MB` from MongoDB's
  own, whose `$collStats` reports bytes), because inventing a column an engine cannot fill would say
  nothing. A row count is the catalogue's **estimate** on the SQL engines, and `(Null)` in
  PostgreSQL's says nothing has analysed the table yet; MongoDB's is exact, and a **view** reports no
  numbers at all — it has no storage of its own, so its row carries its name and its kind and nulls
  where the numbers would be, rather than a row of zeroes claiming an empty collection.
- **The toolbar acts on the selected row**, and its four items are the same objects the tree's
  right-click builds from (`tableActions`), so a confirmation is worded once. `Delete Table` asks
  first, and the change re-reads both the tree's names and this list's numbers.
- **A double-click opens the row's table**, which is the tree's own gesture applied to a row: the
  grid is read-only here, so a click has nothing else to mean. `ResultGrid` reports it through
  `onActivateRow` — see [grid.md](grid.md).
- The list is **read per tab rather than cached** like the tree's names are: a size and a row count
  are what the reader opened it for, and a stale one is worse than a second query.

## Finding text in a schema

**Right-click a database, the node that owns tables, or its `Tables` folder: `Find in Schema…`.** It
opens a tab that reads **every table of that namespace** looking for one piece of text, and answers
with one row per object that holds it and how many times.

```
┌ Connection: ────────────┬ Find results for “khánh”: ──────────────────────┐
│ [localhost-postgres  ⌄] │ Name                    │ Number of Matched Re… │
│ Database:               │ ▦ CV_VerificationCandi… │                     2 │
│ [hirec-test          ⌄] │                                                │
│ Schema:                 │                                                │
│ [public              ⌄] │                                                │
│                         │                                                │
│ Find what:              │                                                │
│ [khánh              ]   │                                                │
│ Look In:   '\' as escape│                                                │
│ [Data          ⌄]       │                                                │
│ Search Mode:            │                                                │
│ [Contains      ⌄]       │                                                │
│ ☐ Case Insensitive      │                                                │
│                [ Find ] │                                                │
├─────────────────────────┴────────────────────────────────────────────────┤
│ Found 1 objects with matched records          40 objects searched         │
```

- **`Look In` is two different questions, not two settings of one.** `Data` counts the **rows** whose
  columns hold the text, which is a statement and therefore the driver's
  (`driver.Session.MatchingRows`) — every column is read as text, so a search for `36` finds the
  number as readily as the name. `Structure` counts the **names** that hold it — the table's own, then
  every field, index and constraint — and asks no server anything at all: those names are what the
  catalogue reads the tree already makes answer with. One rule matches both (`domain.TextMatch`), so
  `Contains` cannot mean two things.
- **`Search Mode` is four modes and the reader's text is literal in three of them.** `Contains` and
  `Prefix` are *patterns*: `%` stands for any run of characters, `_` for one, and `\` makes either of
  them itself — which is what the note beside `Look In` says, and it is shown only while one of those
  two is picked. `Whole Word` is the text between two word edges, taken literally, so `a.b` finds
  `a.b` and not `axb`. `Regular Expression` is the reader's own, in their engine's own flavour of one,
  and an expression the server refuses is reported as the server's own rejection.
- **`Case Insensitive` decides, rather than the collation deciding for it.** A caseless match folds
  both sides; a case-*sensitive* one on MySQL compares the pattern as bytes and asks `REGEXP_LIKE` for
  `c`, because its usual collations would otherwise ignore case whatever the box said.
- **The connection is one of the pickers**, so a search can be pointed at another server or another
  database without going back to the tree — and the tab moves with it, its strip naming where it is
  now looking rather than where it was opened.
- **A count per object, and not the matches.** A table with four thousand matching rows is not a list
  anybody scrolls, so what the grid holds is where to look next.
- **A double-click on a data hit opens a query tab holding the read behind the count** —
  `SELECT * FROM "public"."CV_VerificationCandidates" WHERE (CAST("Name" AS TEXT) LIKE '%khánh%' OR …)`,
  or `db.users.find({ $or: [ … ] })` on a document store. It is **the engine's own statement, arriving
  with the hit**, spelled from the *same* condition the count was made through — so the tab cannot
  show a different set of rows from the number that sent the reader to it, and nothing in the frontend
  has spelled a statement. It is **not run for them**: a statement in an editor is the reader's to
  send, which is the rule everywhere else in the app. A **structure** hit has no such read — matching
  names is not one — so it opens the designer, where a field, an index and a constraint are read.
- **An object the search could not read is named, and the search carries on.** A schema nearly always
  holds a table this account may not read, and stopping at the first one would make the feature
  unusable there; a warning under the results says which and what the server said. A **Stop** replaces
  `Find` while it runs, which is the honest bound on a read as long as the schema is.
- Views and routines are **not** searched, and neither is a value's *type* or a column's default: what
  is read is the tables of the namespace, their rows and their names. On MongoDB the fields looked in
  are the **sampled** ones — the only field list a collection has — and a `$regex` matches a field
  holding a string, so a number in a document is not text to search. See [grid.md](grid.md).

## Comparing two tables, and making them agree

**Pick one table, `Ctrl`-click a second, right-click either: `Compare A ⇄ B ▸ Structure Only…` or
`Data Only…`.** The two need not be in one database, on one server, or even on one engine — a
staging table against the production one is the case the feature exists for, and each side carries
its own session.

The item is on a table's and a view's menu, and it is **there even when only one thing is picked**,
drawn dead and reading `Compare — Ctrl-click a second table`. That is the exception to the rule that
an item with nothing to do is left out: the gesture is the whole feature, and a gesture nobody can
see is a gesture nobody finds.

### The tab

```
[Structure|Data] ⌄⌃ ⌃⌄  Apply to [users_b|users_a]  ☐ Differences only  Refresh   Take all  Take none  [Apply to users_b]
├─ Origin: shop.users_a ─────────────────────╥─────┬─ Target: shop.users_b ───────────────────────┤
│ Name              │ Declaration            ║Take │ Name              │ Declaration  │Difference │
│ ▾ ▦ shop.users_a  │                        ║ ☐   │ ▾ ▦ shop.users_b  │              │3 of 11 …  │
│   ▾ 📁 Fields     │                        ║ ☐   │   ▾ 📁 Fields     │              │3 of 7 …   │
│     🔑 id         │ int NOT NULL PK        ║ ☐   │     🔑 id         │ int NOT N..  │           │
│     ▯ name        │ varchar(50) NULL       ║ ☑   │     ▯ name        │ varchar(100) │type, null │
│     ▯ created_at  │ timestamp NOT NULL     ║ ☑   │                   │              │only in u… │
│   ▾ 📁 Indexes    │                        ║ ☐   │   ▾ 📁 Indexes    │              │1 of 3 …   │
│     ⇅ by_email    │ (email)                ║ ☑   │     ⇅ by_email    │ UNIQUE (em..)│unique     │
│   ▸ 📁 Foreign K… │                        ║ ☐   │   ▸ 📁 Foreign K… │              │all 1 ide… │
├─ dragged ────────────────────────────────────────────────────────────────────────────────┤
│  Script (8) │ DDL Diff                                                                    │
```

`║` is the splitter, and the ticks sit on the far side of it — a strip of their own between the two
trees. The data half is the same three panes with rows in them, and it alone carries `[⇄|⇅]` and
`Rows`.

Three rules hold the whole surface up, and everything else follows from them.

**1. The same tree, drawn twice.** The shape half is **two panes**, each holding one side's tree:
the table is the root, `Fields`, `Indexes` and `Foreign Keys` fold under it, and each row under one
names a thing and carries what *that side* declares it as. Both are built from one list, so a caret
belongs to the **row** and not to a pane — folding either folds the pair, and the two can never
drift out of step. Every row wears the sidebar tree's own glyph and colour, so a field looks the
same in both places. Three tab strips would make one question into three; a reader comparing two
tables is asking what is different *about them*, and this is that question with both answers laid
against each other. **Expand all** / **Collapse all** are on the toolbar and only in this half —
they are the same one press applied to every branch, and a tree with nothing to open has no use for
them.

The data half is the same panes with rows in them instead of a tree. **Only it can be stacked**
(`[⇄|⇅]`): two trees one above the other cannot be read across, and the ticks between them would
have nowhere to be.

**2. Row *n* of one pane is row *n* of the other.** The pairing is worked out on the Go side
(`internal/compare`), the two panes are built from that one list of pairs, a **blank** stands where
a side has not got the row, and the two **scroll as one**. The divider between them starts at half
and **moves the boundary**, so neither side is ever left holding a strip too narrow to read.

**3. A tick is what is taken, and it is a strip of its own between the two sides.** Not a column of
either pane: a tick is a fact about the *comparison* and not about either table, and a column inside
a pane slides out of sight the moment that pane is scrolled sideways — which is the one control a
reader must never have to chase. The strip moves only up and down, whatever either side is doing.
A **section's own tick takes everything under it** and the **table's takes the lot** — the same
control as a row's and not a fourth kind of button. `Take all` / `Take none` set the lot from the
toolbar. What is ticked is the whole of what `Apply` touches, and the direction control says which
of the two tables changes — the *other* one is never written to. Changing the direction, the half
being read or the row count clears the ticks, because a pick is a position in *this* comparison and
the answer under it has moved.

Stacked, there is no between, so each pane carries the tick as its own first column — and it is
still the same tick: a row is taken or it is not, and which pane the reader reached for is not a
second question.

| Colour | Means |
|---|---|
| Amber row, with the differing **cells** deeper | On both sides and not alike |
| Green row | On the **right** and not on the left |
| Red row | On the **left** and not on the right |
| Flat grey row | The blank opposite one of the two above — a fourth thing, and neither a value nor an empty one |
| No wash | The two sides agree |

The colours are `--grid-diff-*` tokens, a family of their own rather than the success and danger
colours reused: nothing here has gone wrong or gone right, and `removed` only ever means "the other
side has not got it".

**Every grid here is dense** — 22 px rows, a one-line header, a narrow gutter — because the pane
holds two grids and a script, and every pixel a row does not take is another row on the screen.
Nothing is dropped to get it: the header keeps the name and the type-kind glyph, and the whole type
is in its tooltip. See [grid.md](grid.md).

### Script

Under the panes, **exactly what Apply will run**, numbered, copyable, and in the engine's own
spelling. It is not a description and it is not built by the frontend: the ticked rows become a list
of changes, and *that same list* is sent to be spelled and to be run
(`PreviewTableChanges` / `ApplyTableChanges`, `PreviewRowWrite` / `ApplyRowWrite`) — so the script
the reader copied is the script that ran. The data half's statements come from `RowStatements`,
which is the one way a row is written anywhere in the app, so a migration's rows and an import's are
the same statements.

The order is the designer's own: **what is going goes first**, so a constraint never stands in the
way of the field change under it; the fields move next; and what is arriving is made **last**, so an
index or a key can name a column this same run has only just added. An index and a foreign key that
merely differ are dropped and remade rather than altered, because neither engine can change which
columns one covers.

`Apply` **stops at the first statement the server rejects**, and says which. That is the honest
outcome — the order is what makes the run work, so every step after a failure is meaningless — and
there is no transaction around it, MySQL committing implicitly at every DDL statement. The preview
is what protects the reader, which is why it is there and why it is exact.

### DDL Diff

The other pane under the panes: both tables' **own declarations**, side by side and line for line.
It is the server's answer rather than a rendering of the comparison above it — MySQL is asked
`SHOW CREATE TABLE` — so it carries the storage engine, the character set and the auto-increment,
none of which a catalogue comparison can see. The comparison says what *this app* can tell apart;
this says what the two tables **are**. Lines are matched by their own content and never by position,
which is the same rule the field comparison follows.

### Structure

Fields, indexes and foreign keys, as three foldable sections under the table itself. The table and
each section say how many of the things below them differ (`3 of 7 differ`, `all 3 identical`,
`neither side has any`) so a shut one still answers the question, and the section for a constraint
neither engine has is not drawn at all. Folding the table shuts the whole comparison down to one
line, which is what makes two of them comparable at a glance.

**Everything is matched by name, never by position.** A column inserted in the middle would
otherwise report every column below it as changed — instead it is drawn *in the middle*, in the
place the right has it, with a blank opposite. Two engines spell one type two ways, so the
comparison folds case (`VARCHAR(50)` is `varchar(50)`) but never length (`varchar(50)` is not
`varchar(100)`), and **no default is not a default of the empty string**.

### Data

Rows matched on a key, with the cells that differ deepened. What the comparison was *able* to do is
under the panes, and none of it is optional:

- **Which columns it matched on.** The primary key both sides declare, or — where they share none —
  every column they share, which the line says. On that fallback a row is matched on the whole of
  itself, so nothing can come back changed, only present on one side; a warning says so rather than
  letting "no changed rows" read as a finding.
- **Which columns it left out.** Only the columns *both* sides hold are compared; the rest are
  named. A value with nothing opposite it is a difference between two shapes, and Structure is where
  that is read.
- **How far it got.** A comparison is **bounded on purpose** — both sides are held in memory at once
  and cross the bridge as one answer, so there is no `All` here the way there is on a table tab:
  that read is [streamed](#an-unbounded-read-on-a-table-tab-is-streamed-watched-and-stoppable), and
  a diff cannot be, needing both sides together. Both reads are ordered by the key before they are
  cut, so the two windows line up, and a side the read did not see the whole of says so.

Applying the data half is an **upsert** of everything ticked that the source holds — changing the
rows the key names and adding the ones it does not — and a **delete** of everything ticked that only
the table being written holds. Two writes and not one, because they are two intentions and
`RowStatements` takes one mode.

A count of each outcome sits under the panes in the colours it drew them in — it is the legend and
the tally at once. Nothing on it is clickable: **Differences only** is what narrows the panes, and a
count that also filtered would be two controls wearing one label.

## Changing a schema

Every schema change is raised from a right-click in the tree, and every one of them is confirmed or
filled in before it runs. `useTreeMenu` is the single place the items are built, per node kind:

| Right-click | Items |
|---|---|
| A connection | `Connect` / `Disconnect`, `New Database…` — only once there is a session to make one on — `Edit Connection…`, `Delete Connection` |
| A database | `New Database…`, `Delete Database`, then what its level owns: `New Table`, `Find in Schema…`, `Dump SQL File ▸` and `Refresh` on MySQL and MongoDB, `Refresh` alone on PostgreSQL, which holds schemas rather than tables |
| The node owning tables, or its `Tables` folder | `New Table`, `Find in Schema…`, `Dump SQL File ▸`, `Refresh` |
| The `Views` or `Functions` folder | `Refresh` — one read fills every folder, so it means the same wherever it is asked for |
| A table | `Open Table`, `Design Table`, `Compare … ▸`, `New Table`, `Rename Table…`, `Duplicate Table ▸`, `Dump SQL File ▸`, `Import Wizard…`, `Export Wizard…`, `Empty Table`, `Truncate Table`, `Delete Table`, `Refresh` |
| A view | `Open View`, `Compare … ▸`, `Delete View`, `Refresh` |
| A function or procedure | `Delete Function` / `Delete Procedure`, `Refresh` |
| The `Fields` folder | `Add Field`, `Refresh` |
| A field | `Edit Field`, `Rename Field…`, `Add Field`, `Delete Field`, `Refresh` |
| An index | `Delete Index`, `Refresh`. Disabled on the index behind a primary key: no engine will drop the key as an index, so the item says so rather than failing at the server |
| A foreign key | `Delete Foreign Key`, `Refresh` |

**Every word above is the engine's** (`engineWords`): a MongoDB connection reads `Open Collection`,
`Delete Collection`, and `Delete every document of orders?` in the confirmation. And the *items*
follow the engine too — MongoDB is offered no `Truncate Collection`, no routine rows and no foreign
key rows, because it has none of the three.

- **Every item carries a glyph**, from `shared/ui/menuIcons.tsx` — one action, one icon, wherever it
  is offered: `Design Table` and `Edit Field` share theirs because both open the designer, and
  anything that destroys carries the bin in the danger colour.
- **`Empty Table` deletes every row; `Truncate Table` drops them all at once.** Truncate is the
  faster of the two and the one a table under a foreign key will refuse, which is why both are there.
  **MongoDB is offered `Empty` alone**: a collection has no storage to reset, so the only way to take
  every document out of one and leave its indexes standing is to delete them — and two menu items
  running one command under two names would be a lie about what the second does.
- **`Duplicate Table` is one action asked in two ways**, so it is the one item with a submenu:
  `Structure Only` and `Structure and Data` are the same bound method and differ by a flag. Both ask
  for the copy's name — nothing here makes an object without being told what to call it — and the
  copy is then the table whose shape is re-read. What travels is the table's own: its columns, their
  defaults and its indexes, and **never a foreign key**, which names two tables and is neither
  engine's to copy. **A name already taken is refused** — on the SQL engines by the server, and on
  MongoDB by the adapter, because that server accepts `create` for a collection it already has and
  `$out` writes straight over one.
- **A change re-reads what it invalidated, and nothing else**: the namespace's table list, then the
  table's own shape. Which shapes are still worth holding is read off the fresh table list, so a
  dropped or renamed table needs no telling.
- **A rejected statement stays where it was asked for** with the server's own wording, so a name
  already taken costs a retype and not the work.

### Dumping a table or a schema to a file

`Dump SQL File ▸` writes **the statements that would make these objects again**, to a file picked in
the system's own save dialog. It is the one item in the tree's menus that changes nothing on the
server, and it sits beside `Duplicate` for that reason: both copy the table, and the only difference
is whether the copy lands on this server or in a file.

- **It is the same two-way choice `Duplicate` is** — `Structure Only` and `Structure and Data` — and
  the same submenu, because it is one action asked in two ways.
- **Where it is offered says what it covers.** On a table, that table. On the node owning tables, or
  its `Tables` folder, **every table of that namespace** in the order the tree lists them. A view is
  never among them — it holds no rows of its own and its declaration is not a table's — and neither is
  a routine.
- **Nothing is confirmed first**, unlike every other item in these menus: nothing is being changed, so
  the save dialog *is* the question, and dismissing it writes nothing at all. The file is named after
  the table, or after the schema or database whose tables are going into it.
- **What kind of file it is belongs to the engine** (`engineWords.scriptFile`): a MongoDB connection
  reads `Dump Script File` and writes `.js`, because its statements are commands and not SQL — and
  calling that file `.sql` would be a lie about its contents.
- **The declaration is the engine's own answer wherever the engine has one.** MySQL's dump is
  `SHOW CREATE TABLE` verbatim — storage engine, character set, `AUTO_INCREMENT`, every index and
  every constraint — with the database written into the name so the file's declarations and its rows
  cannot land in two different places. PostgreSQL has no such statement, so its shape is described
  from the same catalogue read the tree draws: one `CREATE TABLE`, then a `CREATE INDEX` per index
  (never the primary key's, which arrives with the table), then an `ALTER TABLE … ADD CONSTRAINT` per
  foreign key. A `serial` column is declared as `serial` rather than as the `integer` and
  `nextval(…)` default the catalogue reports it as, or the file would name a sequence it never
  created; an index over an **expression** is the one thing a description cannot carry, and it is
  written as though the expression were a column name. MongoDB writes `db.createCollection(…)` and a
  `createIndex` per index, `_id_` excepted since the server builds that one itself — and **nothing
  about the fields**, because a collection declares none: they arrive with the documents.
- **The rows are streamed, not paged.** A dump reads each row once in one pass and writes each batch
  as it arrives, so a table of a million rows costs one batch of memory and not a table's worth. It is
  given two hours rather than a query's fifteen seconds, for the reason
  [Execute SQL File](#execute-sql-file) is: a long job going fine is not a job to cut off.
- **A `NULL` and a value are written exactly as the SQL export writes them** — the same
  `RowStatements` the grid's `Export as SQL` calls, so a dump and an export of one table's rows
  agree to the character. See [Exporting a grid](#exporting-a-grid).

### The Import and Export wizards

`Import Wizard…` and `Export Wizard…` sit beside `Dump` on a table, on the `Tables` folder and on the
node that owns it — the same three places, covering the same subject: **one table where the reader
right-clicked one, and every table of the namespace where they right-clicked the folder.**

**They are not `Dump`, and the three do not overlap.** A dump writes the *statements* that would make
these objects again, for putting a table back on a server. The wizards carry the **rows**, in the
formats something other than a database reads. That is why SQL is not in the wizard's format list and
no wizard format is in the dump menu.

| | Reads | Writes |
|---|---|---|
| CSV (`.csv`), Text (`.txt`) | ✓ | ✓ |
| JSON (`.json`) | ✓ | ✓ |
| XML (`.xml`) | ✓ | ✓ |
| Excel (`.xlsx`) | ✓ | ✓ |
| HTML (`.html`), Markdown (`.md`) | | ✓ |

HTML and Markdown are written and never read: both are laid out to be *looked at*, and reading one
back would mean guessing which of a document's tables was the data — which is exactly the guess that
goes wrong on a document a person laid out. The formats live in
[`internal/transfer`](architecture.md#carrying-a-table-to-a-file), which knows no databases at all.

**One question to a screen**, in the order the answers depend on each other: what the file *is*, then
where it is, then how its bytes are cut into fields, then which field goes into which column, and only
then what the rows are to **do** when they get there. `>>` runs to the last step from wherever the
reader is, so a second import of the same shape of file is Next, Next, `>>`.

- **A step that has nothing to ask is not shown.** A JSON file has no delimiter to be wrong about, so
  a JSON import is six steps and a CSV import is seven. A step of greyed-out boxes says only that the
  wizard was written for something else.
- **Nothing reaches a server until `Start`.** Every step before it reads the file and nothing else,
  which is why the preview needs no session — and why pointing the wizard at the wrong file costs
  nothing.
- **The preview is how the answers are checked.** The head of the file is drawn in a `ResultGrid`
  under the delimiter and rows steps, read exactly as the import will read it: a delimiter that is
  wrong shows as one column holding the whole line, and a header row that is not one shows as columns
  named after the first record.
- **The mapping is proposed, not typed.** A file whose fields are named after the columns — which is
  nearly every file exported from anything — arrives already mapped, and the key is the table's own
  primary key. A column the file has nothing for is **left out of the write entirely**, so the
  server's own default applies: that is not the same as writing `NULL` into it.
- **A workbook is asked which sheet.** Excel stores a date as a count of days and says so only in its
  style table, so that table is read and a date column imports as a date rather than as `45678`.

#### What an import mode does

| Mode | |
|---|---|
| `Append` | Adds every record. A row already there is a failure the *server* reports — only it knows what a key collision is. |
| `Update` | Changes the records the key matches, and adds none. |
| `Append / Update` | Changes the records the key matches and adds the rest. |
| `Append without update` | Adds what the key does not match and leaves the rest alone. |
| `Delete` | Removes the records the key matches. |
| `Copy` | **Empties the destination** and then appends. |

The four that find a stored row first are **left disabled until a key column is ticked** on the
mapping step, because without one they would reach every row in the table. If one slips through
anyway — a key mapped to a field the file turns out not to carry — the statement matches *nothing*
rather than everything: `WHERE 1 = 0` on a SQL engine, and a filter that no document satisfies on
MongoDB.

**Whether the run is answered for first is a `domain` rule, not the wizard's.** `Copy` is
destructive, a keyed mode with no key is unfiltered, and everything else is an ordinary write —
`domain.ImportHazard`, read against the connection's environment by the same
`Environment.NeedsConfirmation` a typed statement goes through. The wizard *asks* the Go side and
draws the confirmation it is told to; it never decides. See
[What gets confirmed before it runs](#what-gets-confirmed-before-it-runs).

`Records per statement` is how many rows one statement carries — more is fewer round trips, and too
many is a statement the server refuses as oversized (`max_allowed_packet`). `Continue after a record
the server rejects` is the same switch [Execute SQL File](#execute-sql-file) offers and means the
same thing. **The reader's own `Stop` always ends the run**, whichever way that switch is set.

#### What an export writes

**One file per table**, named after the table, in the folder the reader picks — even for Excel, which
gets one workbook of one sheet each. A wizard that also asked for a file *name* would be asking a
question with no answer for the forty-table case. A character a file name may not hold is replaced
rather than dropped, so two tables differing only in punctuation do not write over each other.

- **The rows are streamed, not paged**, exactly as a dump's are: each batch is written to the file as
  it arrives, so a table of a million rows costs one batch of memory. Both wizards are given two
  hours for the reason a dump is.
- **A table that fails does not stop the rest.** An export changes nothing on a server, so there is
  nothing for a half-finished one to have left inconsistent — and a reader exporting forty tables
  would rather have thirty-nine and a message than a stop at the seventh.
- **A format that cannot say `NULL` asks what to write instead.** CSV, text, HTML and Markdown have
  nothing to say it with; JSON has `null`, XML says so in an attribute, and a spreadsheet's empty cell
  is not a cell holding no characters, so those three are not asked.
- **A UTF-8 file is written with a byte order mark**, which is what makes Excel open a UTF-8 CSV as
  UTF-8 instead of as the machine's own code page.

The last step of both wizards is a **transcript**, not a grid — for the reason
[Execute SQL File](#execute-sql-file)'s is: a server's rejection is a paragraph it wrote and a path is
a hundred characters wide, and in cells those are the two columns clipped to nothing.

### Where a schema change is made

**Anything with more than one answer in it happens in a tab, not a dialog** — `New Table`,
`Design Table`, `Add Field` and `Edit Field` all open (or reveal) the designer, which has the room
to work in and leaves the tree beside it readable. Three requests stay modal: `Rename …`, which is
one box; a confirmation, which has to be answered before anything else happens; and `New Database…`,
which has **no tab of its own to open** — a database is not a thing you sit and work in, and its
answers are one fixed screen rather than a list.

### New Database

The dialog is a tab strip over a form, and how much of the form there is belongs to the engine.
**PostgreSQL's database is an object in its own right**, so `General` asks for a Database Name, an
Owner, a Template, an Encoding, a Collation Order, a Character Classification, a Tablespace and a
Connection Limit, plus `Allow Connection` and `Is Template`; `Extensions` is a filterable list of
what the server has to offer; and `Comment` is free text. **MySQL's database is a namespace with a
character set**, so `General` asks for a name, a Character Set and a Collation — and the other two
tabs are not drawn, because MySQL has neither.

**MongoDB has no `CREATE DATABASE` at all.** A database exists exactly as long as it holds a
collection, and no command makes an empty one — so `General` asks for a Database Name and a **First
Collection**, and creating the database *is* creating that collection. A blank answer takes `data`
rather than failing: a reader who only wanted the database is better served by a collection they can
rename than by an error about a field they did not know was required. The preview pane is labelled
`Command Preview` there, because what it shows is not SQL.

- **Every list is the server's own**, read in one call when the dialog opens: the roles, the
  templates, the encodings, the tablespaces, the available extensions. A blank entry reads
  `Server default` and leaves that clause out of the statement altogether, rather than this side
  guessing at what the default is. MongoDB answers with no lists at all — a database there is a
  namespace and nothing more — so the dialog draws none of them.
- **MySQL's collations narrow to the chosen character set** without a second read, because a MySQL
  collation is named after the set it belongs to — `utf8mb4_general_ci`.
- **`SQL Preview` asks the driver what it would run.** It is the same list `Create` executes, so the
  two cannot drift; a database with no name yet previews nothing. An extension is shown as its own
  statement because it *is* one, run inside the database that has just been made.
- `Connection Limit` keeps `-1`, which is PostgreSQL's own "no limit", for anything typed into it
  that is not a number — a blank box meaning `0` would be a ban rather than an absence.

`Add Field` from the tree opens the designer with a blank field already appended, which is the one
thing a tab cannot say by being opened. It travels as a slot in the schema store rather than a field
of the tab, because the tab may already be open and a revealed tab has to hear the second ask too.

### The designer

`Design Table` opens a tab laid out the way a table designer is: a toolbar of `Save`, `Add …`,
`Delete …`, `Primary Key` and `Refresh`, over a strip of `Fields`, `Indexes`, `Foreign Keys` and
`Changes`. **`Save` takes the accent the moment there is anything to save** and drops back to a
plain toolbar button when there is not, and `Delete …` carries the danger tone, so the two
buttons that matter are readable before the labels are. `New Table` is the same tab with no table behind it yet and a name box in the toolbar;
its `Save` creates the table, and the tab then goes on designing it rather than offering to create
it twice — its indexes and foreign keys are made straight after, since `CREATE TABLE` carries the
fields and the primary key and nothing else.

- **The toolbar acts on the pane in front.** `Add` and `Delete` read `Field`, `Index` or
  `Foreign Key` depending on the tab, and `Primary Key` is live only over `Fields`. That is why the
  tab strip's selection is held by the designer rather than inside the strip. Changing tab clears
  the selection, because a cell reference is a position in the pane that reported it.
- **Every list is a `ResultGrid`**, because they are rows and columns — see [grid.md](grid.md), and
  **all three are edited in place**: one click on a cell opens it, `Escape` closes it again, and
  the arrows walk the rows a cell at a time.
- Its columns are the parts a field is *declared* from, not the string the server reports back:
  `Name`, `Type`, `Length`, `Decimal`, `Not null`, `Key`, `Default`. Length and decimals are parts
  of the type and travel as one string — `varchar(255)`, `numeric(10,2)` — and read as `0` when the
  type declares none. A `Default` is an **expression**, which is what a server-side default is:
  `now()`, `'IN_PROGRESS'`, `0`.
- **`Type` is a list, and the list is the engine's.** Clicking the cell offers MySQL's types,
  PostgreSQL's, or MongoDB's BSON types — `dataTypes.ts`, read off the connection's own profile — and
  typing filters them. It still takes **anything typed into it**, because a server also has domains,
  enums and types that list has never heard of; the options are what a column is usually given.
- **On MongoDB a field is not a column, and the designer says so by what it does.** A collection holds
  no declaration to change, so a change to a field is a change to the *documents* that carry it:
  adding one writes it onto the documents that lack it and leaves the rest alone, renaming one is a
  `$rename`, dropping one is an `$unset`, and altering one converts its values in place — a value the
  server cannot convert is removed rather than left wrong, which is the only outcome in which the
  collection afterwards holds what was declared. A **new** collection is the one case with nothing to
  change: `New Collection` creates it and the fields declared alongside have no documents to be
  written onto, so the Fields grid comes back holding `_id` alone. That is the truth about a new
  collection rather than a shortcoming — its fields arrive with its first document. See
  [architecture.md](architecture.md#the-document-store) and
  [decisions.md](decisions.md) for why the alternative, a validator built from the declared fields,
  is worse.
- **`Not null` is a checkbox and `Key` is the lock**, both flipped by one click. A key field shows
  a gold lock and **which column of the key it is** — the first is `1`, a second field clicked is
  `2` — counted down the fields, so a composite key reads in order. `Primary Key` in the toolbar is
  the same fact for a whole selection at once.
- `Primary Key` moves the key onto the selected fields, or off them if they all already hold it.
  Both SQL engines are made to replace a key rather than add a second one, and dropping the key of a
  table that has none is not an error. **MongoDB answers `UNSUPPORTED`** for anything but `_id`: the
  server mints it, indexes it and will not let it be changed, so a request naming it is already true
  and one naming anything else cannot be granted. A reader who wants some other field to be unique
  wants a unique index, which the Indexes grid makes.
- **The Indexes grid** is `Name`, `Fields`, `Unique` and `Method`, and **the Foreign Keys grid** is
  `Name`, `Fields`, `Referenced Schema`, `Referenced Table`, `Referenced Fields`, `On Update` and
  `On Delete`. A list of columns is typed as one cell, comma separated, because that is how it
  reads. `Method` offers the engine's access methods and the two action cells offer the five
  referential actions — suggestions, like the type list, since the cell still takes what is typed.
  A blank `Referenced Schema` means the same namespace the table is in. **There is no Foreign Keys
  pane on MongoDB**, which has no referential constraints of any kind: a pane whose `Save` could only
  fail is a pane worth leaving out. Its `Method` list is the values a MongoDB index *key* takes —
  blank for the ordinary ascending index, then `hashed`, `text`, `2dsphere`, `2d` — because that is
  where MongoDB keeps what SQL puts in a `USING` clause.
- **An index behind the primary key is not in the Indexes grid.** It is the key, and the Fields grid
  is what moves a key.
- **Nothing is sent until `Save`.** A pending cell carries the grid's warning ring; a field the
  server has not seen is ringed whole, and so is *any* pending index or foreign key — neither engine
  can alter which columns one covers, so an edited one is dropped and made again rather than changed
  a cell at a time. `Changes` lists what `Save` will do, in the order it will do it: **what is going
  goes first** — keys, then indexes — so nothing stands in the way of the field changes that follow;
  then the fields, a rename before the alter so the alter can still name the column; then **what is
  arriving**, so an index can name a column the same `Save` has just added. It reads as plain
  changes and not as SQL, because how a statement is spelled belongs to the driver and this side
  has never seen one.
- `Refresh` re-reads the table and drops whatever was staged. Nothing in the Indexes or Foreign Keys
  grids is ever `(Null)`, and neither is a missing length: a type with no length declared none, and
  `(Null)` would claim a column defaults *to* null.

The query toolbar mirrors the same shape: one `Database` picker on MySQL, and a `Schema` picker
beside it on PostgreSQL. Changing the database clears the schema, since a schema of one database
names nothing in another.

**Double-clicking** a table opens a data tab; a single click only selects it, and reads nothing from
the server. **Until the first page lands the pane says so** — `Loading <table>…` under a spinner,
from `shared/ui/Loading` — because a grid with no columns yet is a blank pane and a table of any
size can take seconds to answer. Only the *first* read gets it: once rows are on screen a re-read
leaves them there and the pager alone says it is working, which is what keeps a page turn from
blinking. A tab loads 100 rows per page by default, ordered by primary key so paging is stable —
until a header's `⋮` menu, or the Filter & Sort panel, gives it keys of its own; those travel into the
read's `ORDER BY` and therefore order the whole table rather than the page on screen.
**Paging lives on the right of the bar under the grid** — first, back, the page typed into a box,
forward, last — with a **gear** beside it that opens `Limit records` and the count that sizes a page,
and can lift the bound entirely; see [the row limit](#the-row-limit). **Under that bar is the status
line**: the read that produced the rows with a copy beside it, where the reader is in what it
returned, and how long ago it landed. The left of the control bar is where rows are added, deleted,
applied and discarded: it is the same `GridFooter` every grid has, and
[grid.md](grid.md) specifies both it and the line under it. Above the grid is one toolbar: **Export** against its right edge,
**Filter & Sort** on the left of it — see [filtering and sorting](#filtering-and-sorting) — and before
that two independent toggles, since they occupy different edges:

**Both panels are dragged, and the size is remembered.** The handle between a panel and the grid is
the divider itself and lights up `--accent` under the pointer; the bounds stop a drag leaving either
panel too small to read or the grid with nothing left. A panel's width or height is kept per
*combination of open panels*, so the list the reader widened is that width the next time they open
it — see [frontend.md](frontend.md) on `panelIds`.

- **`Cell Editor`** opens a panel under the grid for the selected cell. Discard reverts the draft and
  Apply stages the value on the cell, exactly as opening it in the grid does — the footer's
  ✓ is what writes `UPDATE … WHERE <primary key>` and reloads. A table with no primary key is
  read-only, because no single row can be identified. MySQL adds a `LIMIT 1` belt to the braces;
  PostgreSQL has no such clause and does not need one, since the key already names one row.
  **A query result has the same panel**, from the button on its own head strip beside Export: it is
  one `CellEditor` on both surfaces, and the line under it says why a value cannot be written back
  where it cannot — a table with no primary key here, a result no `UPDATE` can name there.
- **`Column`** opens a checkbox list to the left of the grid that picks which columns show.
  Unchecking the last visible column is blocked, so the grid can never go blank. Selection is
  cleared on a change, because the cell coordinates the grid reports are positions among the
  *visible* columns and the view maps them back to real column indexes. **Right-clicking a column
  header hides or restores columns too**, alongside the copy and paste items every grid's header menu
  has — one hidden set, either way in. [grid.md](grid.md) specifies that menu.
- **A column is dragged into place by its header**, and where a table's columns were put is
  **remembered against that table** — which connection, which namespace, which table — so opening it
  again next week, or after the app has been closed, opens it arranged. A query result is arranged
  the same way but remembers nothing: there is no stored table under it to remember one against, and
  its order goes with its statement, exactly as its sort does. The order is kept by column *name*,
  so a column added to the table since, or one dropped from it, moves nothing else.
  [grid.md](grid.md) specifies the drag.

## The menu bar

The strip above the command bar carries `File`, `Edit`, `View`, `Favorites`, `Tools`, `Window` and
`Help`. **Only `Window` does anything yet** — the other six are names in the place a reader of a
desktop client looks for them, and nothing behind them. Items are `MenuItem`s drawn by `MenuButton`,
the same object the tree's right-click menu is built from, so an action offered both ways stays one
object, and a menu given no items draws its name alone.

- **`Window` lists every open tab**, in the order the strip draws them, and picking one is exactly
  what clicking it in the strip is. It is the one place that names all of them at once: the strip
  truncates each title and scrolls the moment it holds more tabs than it can draw. Each row wears
  its tab's kind glyph in the tree's colour for that kind, and says **where** the tab is —
  `users — shop.public · Local MySQL` — because a strip has room for the title alone and two tabs
  called `users` on two servers read there as one tab. **The tab in front wears a tick** in place of
  its glyph, which is the one fact the strip stops telling the reader once it has scrolled. With
  nothing open the menu says `No open tabs` rather than opening empty.

## Every divider in the window is dragged

There are five, and they behave alike: **the sidebar against the work area**, **the editor against
the output panel** in a query tab, in a table tab **the column list against the grid**, and **the
grid against the cell editor** wherever there is one — a table tab and a query result both. Each is
the one-pixel line between the two panels, and it goes
`--accent` while the pointer is over it or holding it. The pointer is caught within ten pixels of the
line, so the target is a good deal wider than the line looks.

- **Double-clicking a divider puts it back where it opened**, which is the way out of a drag that
  went too far without hunting for the size that was there before.
- **A divider takes the keyboard**: `Tab` reaches it, the arrow keys across its orientation move it,
  and `Home` / `End` take it to either end.
- **Dragging the sidebar's divider past its minimum closes the sidebar**, which is the same thing the
  command bar's toggle does — and the toggle is told, so the next press opens it rather than being the
  press that appears to do nothing.

**A layout is remembered per window and per combination of panels**, in `localStorage`, and only when
a *reader* moved it: a panel opening or the window being resized is not a new preference. Every
divider is bounded so a drag cannot leave a panel too small to read what is in it — the cell editor
stops at its own chrome plus a line of value, the column list at 420 px, and the grid keeps three
quarters of the height whatever the editor under it is given.

Column widths inside a grid are dragged too, and are the grid's own business —
see [grid.md](grid.md).

## Tabs

**Every tab says in full what it is showing when the pointer rests on it**: what kind of thing and
its name, then `Database:`, `Connection:` and `Server:` as `user@host:port`. The strip truncates to a
name at 220 px, and two tabs called `users` on two connections are otherwise the same tab — the
tooltip is where that is settled. It is lines rather than a sentence because a reader is looking for
one of those facts, not reading prose, and it is the platform's own tooltip rather than a component,
which is what makes it appear over a tab the pointer merely passes.

**Only the tab in front is mounted**, so anything a tab is holding in component state is taken down
with it the moment the reader looks at another one. What each kind of tab keeps across that is
therefore a deliberate list, kept in a store under the tab's id and dropped when the tab closes:

- A **table tab** keeps *what was asked of it* (`stores/tableViews.ts`): the panels open — Cell
  Editor, Column, Filter & Sort — the columns hidden, the page and its size, the order and the
  conditions. It does **not** keep the answer: the rows are read again on the way back, so a table
  something else has changed under it reads fresh rather than stale, and the status line says when.
  The column *order* is remembered against the table itself and outlives the tab entirely.
- A **query tab** is the other way round and keeps its rows (`stores/queries.ts`), because a
  statement cannot be re-run behind the reader's back: it may write, and it may be expensive.
- A **compare tab** keeps only *which half* it was showing — its two tables and its mode are the
  tab itself. The diff is read again on the way back, for the reason a table's rows are: a
  comparison that had gone stale would be worse than the second read that replaces it.
- A **Find in Schema tab** keeps its hits (`stores/searches.ts`), for the reason a query tab keeps its
  rows: a search of a whole schema is far too long a read to lose by looking at another tab. What was
  *asked* — the text, `Look In`, `Search Mode`, the box — is the tab itself, exactly as a query tab's
  SQL is, and so is where it is pointed.

**A tab is named by what it shows, and a comparison by both sides**: `users ⇄ users_v2`, and the
tooltip carries `Against:` as well, because two tabs comparing `users` against two different servers
are otherwise the same tab. Asking for a pair that is already open **reveals that tab** — the mode
is state inside it, not part of what names it, so `Compare ▸ Data Only` on a comparison showing
structure switches that one rather than opening a second on the same pair.

**The wheel closes a tab**, as it does in every strip of tabs there is, and the `✕` that appears
under the pointer is the other way. The press itself is refused: Chromium's middle-click autoscroll
starts there and swallows the click that would follow, and the strip scrolls the moment it holds
more tabs than it can draw — so without that the gesture worked on a short strip and quietly stopped
working on a full one.

## Query tabs

**New Query** sits in the command bar beside **New Connection**, and as the `+` at the end of the tab
strip; both open the same tab, which inherits the selected connection and namespace, falling back to
whichever connection is open.

A query tab carries its own toolbar, editor buffer, database and row limit, so several tabs can sit
on the same connection without treading on each other. The toolbar picks the database, then
**Execute** (`F5` or `Ctrl`+`Enter`) or **Explain** runs the statement; **Format**
(`Ctrl`+`Shift`+`F`) lays it out; **Stop** cancels it.

- The namespace is applied on a connection pinned for that one statement — `USE` on MySQL,
  `SET search_path` on PostgreSQL — so the pool's other connections keep whatever they had. On
  PostgreSQL the database also decides *which pool* the connection comes from. On MongoDB it is
  simply the database the command is addressed to; one connection reads them all.
- A statement that returns rows fills the grid; one that does not reports its affected-row count.
  Which path a statement takes is decided by its leading keyword, read past any leading comments,
  plus a `RETURNING` clause on PostgreSQL, where that is what makes a write answer with rows. On
  MongoDB it is decided by the *method*: `find` and `aggregate` answer with documents, `insertOne`
  and `deleteMany` with a count.

### What Explain shows

**Explain runs the statement and asks about it as well.** `Result` therefore holds the statement's own
rows — the same rows Execute would have put there, writable on the same terms — and two more panes
appear beside it, `Summary` and `Explain`, holding what the server said it was going to do. It is
guarded like Execute for exactly that reason; see
[What gets confirmed before it runs](#what-gets-confirmed-before-it-runs).

- **The plan is read before the statement runs**, and that order is the whole of what makes it worth
  reading: asking afterwards would ask about a server the statement has already changed, and what
  `DELETE FROM t` was going to do is not what the planner says once the rows are gone. On MongoDB it
  also means a command with no plan answers `UNSUPPORTED` while it has still changed nothing.
- **The plan is read by the driver, never by the frontend.** Each engine asks in two forms — the
  `EXPLAIN` a reader would have typed, and the JSON form that carries a number per step — and both
  land in `driver.Plan`. **Neither is ever the `ANALYZE` form**: the statement runs once, on the path
  every statement takes, and the plan alongside it is an estimate.
- **Either form may fail on its own and the other is still shown.** A plan that cannot be had at all
  is no plan rather than a failure: the statement has run, its rows are in hand, and the two extra
  panes simply do not appear.
- `Summary` is what the plan says about itself, in the engine's own wording: a total cost and a
  startup cost on PostgreSQL, a query cost and a table count on MySQL, the winning stage and how many
  plans lost on MongoDB. It is a list of pairs rather than a fixed set of fields, because a field for
  a fact an engine has not got would only be filled with a zero that read as an answer.
- **`Explain` holds three tabs**, and they are three because they answer different questions:
  - **`Grid`** — the engine's own EXPLAIN, exactly as it answered: MySQL's grid of access types,
    PostgreSQL's plan text down one column, MongoDB's one document. Nothing is normalised, reordered
    or renamed — a reader who reaches for Explain knows these words, and rewriting them would put this
    client between them and their server.
  - **`JSON`** — the plan document as the server wrote it, laid out, with numbered lines and folds.
    It is where a step's cost and the shape of the tree actually live, and it is the answer to "but
    what did the server *say*", which no reading of it can be.
  - **`Statistics`** — one row per kind of step: how many there are, what they cost between them, and
    what share of the plan that is, dearest first. It counts a step's **own** cost, meaning what it
    adds beyond what it reads, so the shares add up to the plan rather than to several times it.
- **A step the engine did not price is left blank, not zeroed.** MySQL prices reading a table and
  evaluating its rows; it does not price the sort or the grouping above them, and a `0.00` there would
  read as free rather than as unpriced. A document store prices nothing at all, so its statistics carry
  no cost columns — `Costed` says so, and the grid is not left to infer it from a column of zeroes.

### What a MongoDB tab runs

The editor takes the shell's own commands, and the driver reads them whole — the grammar is small
enough to. `db.<collection>.<method>(…)` for a collection, `db.<method>(…)` for the database, plus the
cursor methods a read is bounded by:

```javascript
db.users.find({ tier: "gold" }).sort({ joined: -1 }).limit(20)
db.users.findOne({ _id: ObjectId("507f1f77bcf86cd799439011") })
db.orders.aggregate([
    { $match: { total: { $gt: 100 } } },
    { $group: { _id: "$customer", spend: { $sum: "$total" } } }
])
db.users.updateMany({ tier: "gold" }, { $set: { note: "reviewed" } })
db.getCollection("audit-log").countDocuments({})
db.runCommand({ collStats: "users" })
```

- **The arguments are documents, not text.** A field name and a value reach the server as *values*, so
  there is nothing to quote and nothing a reader typed can be read as anything else.
- **The shell's constructors are read as themselves**: `ObjectId`, `ISODate` / `Date`, `NumberLong`,
  `NumberInt`, `NumberDecimal`, `UUID`, `BinData`, `Timestamp`, `RegExp`, `MinKey`, `MaxKey`, a
  `/pattern/flags` literal, and `new` in front of any of them. A filter that spelled an ObjectId as a
  string would match nothing and say nothing about why, which is the whole reason this is a parser and
  not `JSON.parse`.
- **It is not JavaScript.** There are no variables, no arithmetic and no control flow, because a client
  that half-implemented them would be a client whose answers could not be trusted. **Anything beyond
  the methods this side knows is `db.runCommand({…})`**, which reaches the server verbatim — and the
  error for an unknown method says so.
- **A `find` on one collection is writable**, its documents mapping one to one onto stored ones with
  `_id` addressing them. A projection that leaves `_id` out is read-only, for the same reason an
  aliased key column makes a SQL result read-only: the row it names can no longer be found.
- **Explain** works on `find` and `aggregate`, those being the two with a plan; anything else answers
  `UNSUPPORTED` rather than pretending — and it answers **before the command runs**, so explaining an
  `insertOne` inserts nothing. `Explain` → `Grid` holds the plan as one row whose cells are the plan's
  own nested documents, **View Value** being where one is read, and `Statistics` holds the same answer
  as its stages: `FETCH`, `IXSCAN`, or the `$cursor` that feeds a pipeline and the plan under *it*.
  There is no cost and no row estimate, a document store pricing nothing, so those columns are not
  drawn; see [What Explain shows](#what-explain-shows). A server on the slot-based engine reports its
  plan twice — as stages and as the slots it compiled them into — and it is the stages that are read.
- **`Ctrl`+`Shift`+`F` lays a command out** the way it lays a statement out: a brace holding more than
  a line comfortably carries goes down the page, and one that does not stays across it. Nothing a
  server would notice changes.

### One Execute runs the whole script

**Execute runs whatever the editor holds**, whether that is one statement or twenty. The text is cut
into the statements it carries and they run in order on the same connection, stopping at the first
the server rejects.

- **Where a statement ends is the engine's**, so the splitter is the driver's: `sqlkit.Split` reads
  the semicolons that are *not* inside a string, a quoted identifier, a line or block comment, or —
  on PostgreSQL — a `$tag$ … $tag$` body, which is how a function carries semicolons of its own. A
  backslash escapes a quote on MySQL and does not on PostgreSQL, so `'C:\'` is one string there and
  a runaway one here. A chunk with no statement in it — trailing whitespace, the comment a script
  ends on — is dropped rather than sent. MongoDB reads the same semicolons through the same lexer,
  with `//` opening a comment and both quotes opening a string.
- **The grid shows the last result set the script produced**, so a script ending in a `SELECT` shows
  it and one ending in an `UPDATE` keeps the last set it did return.
- **A `Messages` pane appears beside `Result` for a script**, listing every statement with its row
  count, its affected count, its time and either `OK` or the server's own wording. It is a
  `ResultGrid`, because it is rows and columns.
- **A rejection is a message, not a failure — for a script.** One statement on its own still fails
  as a failure, so the output panel shows the server's wording where the reader is looking; but the
  statements above a rejected one *did run*, and throwing would hide how far the script got. A
  script that stops opens the `Messages` pane by itself, and its `Messages (n)` tab says how many
  statements there were to account for.
- **Rows are paged under the grid by the same pager the table view has.** What a read returned is a
  *prefix* of the result, and the pager cuts pages out of it in memory — turning to a page it
  already holds reads nothing. A total shown as `1,000+` on the status line says the prefix stopped
  at the toolbar's `Limit`, and that is also what keeps `Next` alive on the last page: asking for a
  page past the prefix, or clearing the gear's `Limit records`, **replays the statement for as many
  rows as that needs**. So lifting the bound really does load all, and paging is never stuck at the
  limit. Two things follow from the replay: a statement
  with no `ORDER BY` may reshuffle what lands on a page, and the reader keeps their page number
  across a read of the same statement — only a different statement sends them back to page one.
- **A header's `⋮` menu sorts the result in memory**, since there is no statement to re-order: it
  reaches the rows fetched so far and no further, so on a truncated read the prefix is sorted rather
  than the table. Fetching more re-sorts what arrives. A different statement drops the sort. **A
  query result is not filtered** — narrowing belongs to a tab whose rows come from a statement this
  side writes, which is the table tab.
- **Errors surface in the output panel below the editor**, with the code, the server's own wording
  and the engine's error code, never as a dialog. A cancel is reported as `QUERY_CANCELLED`, not as
  a failure.
- Progress shows in two places: `Running…` beside the toolbar buttons, and a spinner on the pager
  under the grid. Previous rows stay on screen until the new ones land.
- **How many rows came back, and how long it took, are on the grid's own status line** — not on a
  strip of their own under the panel. There was one, and it said `325 rows · 5,077 ms` directly
  beneath a line already reading `Record 1 of 325`: two accounts of one read, a row apart. The
  elapsed time went with it, `ElapsedMs` still crossing the wire for whatever wants it next.

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
columns — see [grid.md](grid.md) — and this simply is not rows and columns, exactly as
[the console's transcript](#the-console) is not.

- **A statement keeps its own line breaks.** Every line hangs from a six-character tag, measured in
  `ch` so it holds at whatever size the reader set the data font to, which is what keeps a wrapped
  line — and a statement's own second line — under the statement rather than under the tag.
- **`[ERR]` carries the server's wording alone**, without the wire code beside it. The code is what
  the *app* switches on ([errors.md](errors.md)); next to the wording it is noise, and where the
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

## The row limit

There are two row-count controls, and they are shaped by what they size rather than by each other.
The **query toolbar's** is `shared/ui/RowLimit`: a dropdown of presets plus `All` and `Custom…`,
where `Custom…` reveals a number input beside it for any count. Picking `Custom…` seeds that input
with the count already in force and applies it at once, so the number on screen is always the one
that will run; typing commits per keystroke, and clearing the box keeps the last valid count rather
than fighting the typist.

The **grid pager's** is the gear at the end of the pager, and it is a *checkbox* plus a count —
`Limit records`, then how many `records per page`. Lifting the bound is a different kind of read
rather than a bigger one, and a checkbox says that where one more entry in a list of sizes would
have hidden it. The count survives the bound being lifted, so turning it back on restores what it
was; it commits per keystroke for the same reason the dropdown's does.

| Control | Sizes | Default | What it sizes |
|---|---|---|---|
| Query toolbar `Limit` | 100, 1 000, 10 000, or typed | 1 000 | The rows one **Execute** may return — the first prefix, not a ceiling, since the pager can replay for more. Hitting it is reported as a `+` on the status line's total rather than passed off as the whole answer. It sizes the *next* Execute, so a pager that grew past it leaves it alone. |
| Grid pager gear | any typed count | 100 | One page of the grid. Clearing `Limit records` collapses the walk to a single page, so the pager sizes itself by the row total and the arrows go dead — and on a query result it first fetches every row, because otherwise it would only mean all of the prefix. On a **table** tab that read is streamed and watched, below. |

Lifting the bound is deliberately the *only* way to fetch unbounded, so the bounded defaults of
[decisions.md](decisions.md) cannot be lost by accident. It travels the wire as the sentinel
`app.AllRows` (`-1`) from both controls, and `driverRows` translates it into the driver's own
convention of a non-positive limit meaning "read everything". A missing, zero or otherwise
nonsensical count still falls back to `app.DefaultQueryRows` / `app.DefaultPageSize` — the Go zero
value can never mean unbounded.

Two consequences of that convention are worth knowing:

- `buildSelect` in either adapter **drops the `LIMIT … OFFSET …` clause** for an unbounded page,
  which makes its offset moot. `MaxRows` needs no equivalent, since `sqlkit.ScanRows` already reads
  to the end on a non-positive cap. It **binds nothing at all**: a limit and an offset are numbers
  this side counts rather than anything a reader typed, so they are written into the statement, and
  one string is then both what runs and what the status line shows — the only arrangement in which
  the two cannot drift apart.
- There is no ceiling on a typed count in either control. One would be theatre next to an unbounded
  read, and silently answering a different question than the one asked is worse than a slow grid.

### An unbounded read on a table tab is streamed, watched and stoppable

A table of a million and a half rows broke both halves of the first attempt at this: the read outran
the 15-second deadline every other read is given, and the answer would have had to cross the bridge
as one JavaScript string, a bound method's result being delivered to the WebView as exactly that. So
a table read that asked for every row and minted a **run id** behaves differently from every other
read:

- **It has no deadline.** How long every row of a table takes is not knowable ahead of time, and a
  clock that cuts it off is the very failure an unbounded read exists to avoid. `StopTableRows`
  bounds it instead.
- **Its rows arrive in batches.** `LoadTableRows` answers with the columns and the total alone and
  emits the rows as `app.EventRows` — `rows:batch`, `sqlkit.BatchRows` (5 000) rows at a time.
  `useTableData` stitches them back together and shows the grid once the call resolves. It is still
  **one statement and one scan**: the batches are how the answer is delivered, not a walk of
  `LIMIT … OFFSET …` pages, so nothing is counted twice and no row is skipped by a write landing
  mid-read.
- **Its count arrives separately, on a clock.** `app.EventRowsCounted` — `rows:counted`, every
  `sqlkit.CountEvery` (50 ms) — and that separation is the whole reason there are two events. A batch
  fills on a round number, so a count handed over with one would step in thousands however fast the
  read ran; on its own clock it is whatever the read has *truly* reached, so it climbs the way the
  number in any other client does. The last count sent is the exact one, whatever the clock had said
  a moment before.
- **A panel over the grid reports it while it runs** — `shared/ui/FetchProgress`: how long it has been
  going, how many rows have arrived, and `Stop`. It sits *over* the rows rather than replacing them,
  because whatever the grid already held is still true; `Loading` stays out of its way. The clock is
  the panel's own, so counting seconds costs the grid nothing.
- **Stopping keeps every row the read counted** — not only the batches that happened to fill.
  `ScanRows` hands the part-filled batch over on its way out of *any* ending, a stopped read
  included, because the alternative is a count of 60 031 against a grid of 60 000: the read visibly
  losing 31 rows it had already claimed. A stop is a choice and not a failure, so those rows become
  the grid's and the total becomes the number actually held. The columns are the ones already on
  screen, so a *first* read stopped before it answered shows nothing — it had named no columns yet.
  Leaving the tab or changing the limit stops the read too, rather than leaving a scan of a million
  rows emitting batches at a listener that has gone.
- **The count where a stop lands is ahead of the last one shown**, and honestly so: `Stop` has to
  reach the server and the scan only notices between rows, so a read pulling 300 000 rows a second
  gets tens of thousands further in that time. What the grid shows is what was truly read, not what
  the panel last had room to say.

Two limits are worth knowing before pointing this at a very large table. The rows are held **twice**
while they arrive — once as the batches queue for the WebView, once as JavaScript — so memory is the
real bound, which is the trade `All` is asking for. And a **query** result's `All` is unchanged: it
still replays the statement under the ordinary statement timeout, since a script's outcome is one
answer rather than a stream.

### Laying a script out

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

### What the editor says before anything runs

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

### Every statement run is kept

**Execute writes what it ran into a log on disk**, and the `History` pane is that log — see
[the history pane](#the-history-pane). It is kept whatever became of the statement, a rejected one
being among the likeliest to be wanted back, and it is written after the run rather than before, so
what is remembered is what actually happened. A log that cannot be written is dropped rather than
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

## The output panel

The panel under the editor is a tab strip of **`Result`**, **`History`** and, for a script,
**`Messages`**; a further pane — a query plan — is one more entry in the list `QueryOutput` hands
`ResultTabs`. Nothing sits below that strip: what the run did is the pane's own to report, on the
status line under its grid, so `QueryOutput` is the tabs and nothing else.

**An Execute brings its own outcome forward** — `Result`, or `Messages` for a script that stopped —
wherever the reader had got to, and otherwise the strip stays where they left it. `Messages` is
there only for a script, so a single statement run while it was in front puts the reader back on
`Result` rather than on a pane that has gone.

**A failure can be taken away with the reader.** The app selects nothing by dragging — it is a
desktop client, not a document — but *every surface that reports a failure is an exception*: the
error panel, the one-line strip under a grid that could not save, an empty pane whose hint is the
server's wording, the console's transcript, the note under a box a dialog refused, and the status
bar. The error panel also carries a **copy button**, which takes the code, the wording and the
engine's own detail together, since dragging across three lines in a pane forty pixels tall is not
how anybody wants to quote an error.

### The history pane

`History` is every statement **this connection** has run, newest first, in a `ResultGrid` like
everything else that shows rows and columns: `When`, `Statement`, `Database`, `Runs`, `Rows`,
`Time (ms)` and `Message`, which reads `OK` or the server's own wording exactly as `Messages` does.

- **A double-click loads a row's statement into the editor**, which is what the log is for; the grid
  is read-only, so a click has nothing else to mean. It brings **the namespace it ran in** with it,
  because a statement names tables of one database and one run against another would fail on names
  that are not there — the toolbar's pickers move with it, visibly. The editor's own undo is what
  puts back whatever was being typed.
- **It is ordered by a header's own `⋮` menu**, in memory, there being no statement of its own to
  re-order; the footer says how many statements the log holds. There is no filtering here. `When` is
  spelled `2026-08-04 14:23:05` — local time, in the one spelling that also sorts as text.
- **The pane is read on mount rather than held in a store.** It is drawn only while the reader is
  looking at it, so every open is a fresh log and an Execute never has to tell it anything.
- **The log is every connection's and the pane shows one.** A statement of another server names
  tables that are not here, so it would be noise at best and unrunnable at worst.
- **`Clear History` forgets every statement**, behind the same confirmation a drop is behind, and it
  says that nothing on any server changes. It sits over the rows rather than in the footer, because
  it acts on the whole log where the footer acts on a row. There is no way to forget one entry: a log
  is what it is, and a reader who wants it gone wants it gone.

Under the pane sits the same `GridFooter` the table view has, with the source of the rows named on
its right — the qualified table, or why the result is read-only:

- **The editing icons show only for a writable result.** For one that no `UPDATE` can name they are
  not there at all; `Refresh` and `Stop` remain.
- **One click on a cell opens it** (or `Enter` on the focused one). `Esc` abandons the input and
  gives the keyboard back to the grid. Edits are held locally and marked in the grid, so a mistyped
  value costs nothing.
- ✗ drops every pending change, ✓ writes them — edits with the same `UPDATE … WHERE <primary key>`
  the table view uses, then inserts, then deletions — and reloads. ↻ reloads alone.
- **`Refresh` and `Apply` replay the statement that produced the rows**, not whatever the editor
  holds now, so editing the SQL after a run cannot make Apply execute something else.

## Filtering and sorting

**Filtering is the table tab's**, since it is the one surface whose rows come from a statement this
side writes and can therefore narrow. A query result and the history log hold their rows already and
are only ever *sorted* — by a header's `⋮` menu, in memory. What follows is the table tab.

There are **three ways in**, and they hold the same conditions:

- **The Filter & Sort button** in the table view's toolbar, beside `Cell Editor` and `Column`. It
  opens `FilterSortPanel`, which is where a
  condition is *built*: a tick box, a column, an operator and a value, each of the three parts
  opening the list of what it may be. `+` adds a condition beside the row and `()+` adds a **group**,
  which is parentheses joining its children by `and` or by `or` — the word between the rows is a
  button that flips the whole group, so `a AND (b OR c)` is sayable and not merely wishable.
  **Sort By** under it is a list of columns and directions, the first deciding and the rest breaking
  its ties. Nothing narrows anything until **Apply Filter & Sort**; the panel says **Criteria
  Edited** while the draft differs from what is in force. The whole of it is in
  [grid.md](grid.md#the-filter--sort-panel).
- **A header's `⋮` menu**, where the reader already knows which field they mean: **Add Filter** opens
  the panel and puts a condition on *that* field in it, at the end of the root. A header knows the
  field and not the value, so what it adds is unspelled — it lands in the **draft**, narrowing
  nothing until the value is picked and **Apply Filter & Sort** is pressed. It is the shortest way
  from a column the reader is looking at to a condition on it.
- **Right-clicking a cell**, where a condition is *picked* rather than built. The menu is one object —
  [grid.md](grid.md) lists every item — and **Filter ▸** offers every operator against the value under
  the pointer, so a condition read off a cell is never typed twice; it applies **at once**, having
  been picked whole. **Show/Hide Filter & Sort** opens the panel, and **Remove All Filters & Sorts**
  clears the lot, the sort included. Hiding the panel does not widen the grid — the conditions hold
  either way, which is what the count on the button is for.

**Twenty operators**: `=`, `≠`, `<`, `≤`, `>`, `≥`, `contains`, `does not contain`, `begins with`,
`does not begin with`, `ends with`, `does not end with`, `is NULL`, `is not NULL`, `is empty`,
`is not empty`, `is between`, `is not between`, `is in list`, `is not in list`. How many values one
takes is its own business — none, one, two, or any number — and a condition short of what its
operator needs narrows nothing until it is filled in. `is empty` asks about the empty string and
`is NULL` about the absence of a value; they are different questions.

**Each part opens a small panel under itself, never a dialog over the grid** — the table stays in
sight while it is being narrowed. The **column** panel and the **value** panel are both searched
rather than scrolled, since a table has as many columns as it likes and a column as many values as
its rows.

**A value is picked from the column's own values.** Clicking the value part lists what the column
actually holds — `LoadColumnValues`, a `SELECT DISTINCT … ORDER BY 1 LIMIT 200`, or on MongoDB a
`$group` / `$sort` / `$limit` pipeline, which is the only spelling whose cap is the server's: `distinct`
answers with every value there is and would hand back a megabyte to fill a list of two hundred — with
a box beside it for a value the list has not got. NULL is not offered there: `is NULL` is the way to it. There is
no OK on the panel: what it changes is the draft, and **Apply Filter & Sort** is the one commit.

**Ticking more than one value makes it `is in list`.** The rows are check boxes, and the operator
changes as the second one is ticked — `Name = Java developer` becoming
`Name is in list (Java developer, JavaScript)` — so what is on screen is always what will run;
a negative operator becomes `is not in list`, and unticking back to one gives the old operator back
while the panel is still open. The four comparisons that name a **bound** (`<`, `≤`, `>`, `≥`) are
single-pick instead: `> a` and `> b` are one comparison, not both.

**The filtering happens on the server**: the tree travels in `TablePageRequest`, becomes the read's
`WHERE`, and the **total is counted through it too**, so the pager walks the pages that are actually
there and the status line's statement shows the clause that ran. A draft row is never filtered away;
it is a row still being filled in.

The operator crosses the wire as a **token** (`gt`, `contains`, …) and is spelled by the driver, so
nothing a reader typed arrives as an operator; a conjunction crosses as `and` or `or` and nothing
else. On the SQL engines the *values* are written into the statement as string literals rather than
bound, for the same reason the `LIMIT` is: one string is then both what ran and what the status line
shows and copies — a statement carrying `?` is not one a reader could paste into an editor.
`sqlkit.Dialect.QuoteString` is what stands between the text and the statement, and both engines read
a string literal back into a column of any type. Two smaller consequences: the six text matches
compare the column **as text** (`CAST(… AS CHAR)` / `AS TEXT`), so they work on a number or a date as
well as a name, and a `%` the reader typed is their own wildcard. In memory those matches ignore case;
on the server the collation decides, which is MySQL ignoring it and PostgreSQL not.

**On MongoDB there is nothing to quote.** The tree becomes a `bson.D` and travels as a *document*, so
a field name and a value reach the server as values and can only ever be values. Three things follow.
The reader's text is read into whatever type the sampled field holds, which is what stands where a
SQL engine's cast stands — a value that will not read as that type stays the text it is, matching
nothing rather than quietly becoming something else. The six text matches are a `$regex` with the
value **escaped** and matched case-insensitively: a regular expression has a dozen metacharacters
where `LIKE` has two, so a `contains` of `a.b` finds `a.b` and not `axb`, and a reader who wants a
pattern writes one in the query tab. And `is NULL` is `{ field: null }`, which finds a document
*missing* the field as well as one holding null, because in a document store those are one question.
The statement shown under the grid is the command that ran, spelled from the same documents that were
sent: `db.users.find({ age: { $gt: 30 } }).sort({ _id: 1 }).skip(0).limit(100)`.

## Reading a value whole

A cell is a line in a grid, and plenty of what a database holds is not one line: a JSON document, a
GeoJSON shape, a PostgreSQL array, a blob. **View Value** in every grid's cell menu opens
`ValueDialog` on the cell under the pointer, read-only; the **Cell Editor** panel — a table tab's and
a query result's alike — shows the same `ValueViewer` on the cell it is editing, and writes.

**Which views a value offers is read off the value, not off its column.** The type is not enough and
never was: a `TEXT` column holds JSON as often as a `JSON` one does, a `JSON` column holds GeoJSON,
a MongoDB field holds whatever each document put there, and every value arrives as a string whatever
it started as. `features/results/value.ts` is where one
cell is read, and it reads nothing else.

| The value | Views, in the order they open |
|---|---|
| JSON — an object or an array that parses | **JSON** laid out, then **Raw**. Only a leading `{` or `[` is tried, or every number in the grid would be handed to `JSON.parse`, which happily accepts `42` and calls it a document |
| A **document store's document or array** — a field the engine types `object` or `array` | **Document**, then **Raw**. This is checked *before* JSON, so an array of scalars that happens to parse still reads in the notation the grid drew it in and the parser reads back |
| **The laid-out view itself** | `DocumentView` — numbered lines, a **fold arrow beside every object and array**, and the editor's own syntax colours with the keys apart from the values they name. It is CodeMirror, the SQL editor's own engine, for two reasons: a document worth opening whole is often far larger than a screen and CodeMirror lays out only the lines in view, where a `<pre>` of a megabyte freezes the window; and the grammar is the library's, so nothing here is a second scanner. JSON is read by `@codemirror/lang-json`; a document store's notation by the JavaScript parser under its **`SingleExpression`** top rule, because read as a *script* a bare `{ … }` is a block, its first field a label and every field after it a syntax error. It is **editable wherever the surface can write**, with the editor's own history under it, and read-only where it cannot |
| **GeoJSON** — a `type` from the GeoJSON set *and* the member that type is defined by | **Shape**, then **JSON**, then **Raw**. A `{"type": "Point"}` with no coordinates is malformed, and drawing nothing says more than drawing the origin |
| A **PostgreSQL array** — `{a,"b,c",NULL}` | **Items**, then **Raw**. Read as an array only where the column says it is one, since `{a,b}` is also perfectly good text. An unquoted `NULL` inside one is the absence of a value and is drawn in the grid's own NULL colour; a nested `{…}` is kept whole, because a two-dimensional array is a list of lists |
| **Bytes** — a `\x…` value | **Image**, then **Hex**, then **Raw**. A reader who opened a blob wants to know whether it is a picture before they want its bytes |
| Anything else | **Raw** alone |

- **`Raw` is always there and is always exactly what the cell holds**, so however clever the
  detection is there is one view that cannot be wrong.
- **A value is written back in whichever view it is read in**, wherever the surface can write at
  all: `Raw`, the laid-out `JSON` and the laid-out `Document` are all typed straight into, because a
  document is the thing a reader most often opens the viewer to *change* and sending them to `Raw`
  to change it is sending them to the one view that does not lay it out. What is typed goes back in
  the notation it was shown in, which is the notation that engine's own parser reads. The rest —
  **Shape**, **Items**, **Hex**, **Image** — are renderings of a shape or of bytes rather than of
  text, and writing one back would write the rendering; `Raw` is beside them for that.
- **What a value *is* is read when it arrives, not while it is typed.** The tab strip follows the
  cell the viewer opened on, a Discard or a NULL ticked — never a keystroke, or half a typed
  document would count as plain text and take its own tab away mid-word. Opening another view reads
  the value again as it now stands, so an edit made in `Raw` is what the Document view lays out.
- **Every view is selectable, laid-out ones included.** `user-select: none` is global — see
  [frontend.md](frontend.md) — so a rendering that did not opt back in was a document nobody could
  copy out of.
- **The viewer is where a long value is read, and the grid is not.** A cell draws a clipped preview
  of it — see [grid.md](grid.md) — because a cell is one line a couple of hundred pixels wide. This
  is the whole of what **View Value** is for.
- **A document is laid out by the engine that wrote it, never here.** JSON is laid out by
  `JSON.stringify`, which is the platform's; a document store's own notation — bare keys,
  `ObjectId(…)`, `ISODate(…)` — is laid out by **`driver.Text.Format`**, over the same tokens that
  engine's editor and its `Review` read, because how text is laid out is lexical and therefore the
  engine's. It reaches the viewer as `ValueViewer.formatValue`, which `ResultGrid` and the Cell
  Editor panel forward from a surface that has a session. **A surface with no session offers `Raw`
  alone** rather than a layout it cannot vouch for, and the value goes up unformatted first so the
  pane is never blank and a format that cannot run leaves the document readable.
- A document therefore breaks by **the engine's own rule** — `mongo.breakItems`, the same rule the
  editor's formatter uses, so a brace a line comfortably carries stays on it. Formatting the value
  twice is formatting it once.
- **A shape is a plain SVG and deliberately not a map.** A basemap means tiles, and this app has no
  business sending the coordinates in a reader's database to a tile server to find out where they
  are. What the shape *is* — a ring, a route, a scatter — is readable without one, and the numbers
  are in the JSON tab beside it. Coordinates are fitted to the box rather than projected; at the
  size of one geometry the difference is a slight vertical stretch, and a projection would imply a
  map that is not there. A shape with no extent in one direction is centred on that axis.
- **An image is drawn from the bytes already in hand**, as a `data:` URI, so nothing in a reader's
  database is ever sent anywhere to be rendered. Which image it is comes from the **first bytes** —
  PNG, JPEG, GIF, BMP, WebP — because a blob column carries no type and its magic number is the only
  honest evidence there is. Bytes that begin like nothing say so, with the Hex view beside them.
- The hex dump is offset, hex and printable characters, sixteen bytes to a line, capped at 4 096
  bytes — beyond that a reader is not reading bytes, they are scrolling — and the cap says how much
  of how many it showed.

### Why bytes arrive as `\x…`

**A value that is not valid UTF-8 crosses the wire as hex**, written the way PostgreSQL already
spells a `bytea` so that one form means one thing on both engines. `sqlkit.TextOrNil` is the single
place that decides it.

It is not a preference. A bound method's result reaches the WebView as one JSON string, and a byte
sequence that is not text is replaced by the replacement character on the way — a blob arriving as
a row of diamonds — and what has been replaced can never be recovered. Hex is lossless, and it is
what the viewer decodes to show the bytes, the image or the dump.

**Bytes that *are* text stay text.** A `BLOB` holding JSON is far commoner than one holding a PNG,
and a reader who stored text wants to read it rather than decode it. Nothing consults the column's
type to decide this: the bytes themselves are the evidence, which is also why it cannot be wrong
about a `TEXT` column an engine happens to report as a `BLOB`.

## Exporting a grid

An **Export** button sits **in the strip over every grid, against its right edge**, with the
`Cell Editor` toggle beside it — its own head strip on a query result, and the table view's toolbar
where `Cell Editor` and `Column` already are, one strip
over a table rather than two — and opens **CSV**, **JSON**
and **SQL**; each writes to a file picked in the system's own save dialog — so the file lands where the
reader put it, named after the table, and nothing is written if the dialog is dismissed. It is above
the rows rather than in the footer below them because it acts on the table as a whole, where the
footer acts on a row.

**What is written is what the grid holds, in the order it draws it.** A query result exports its
whole result rather than the page in front, since a reader who paged still means the result; a table
tab exports the page, because a table's rows are paged by the *server* and the page is all the grid
has. A hidden column is not exported: it is one nobody asked for. **A reader who means the whole
table rather than what is on screen wants
[`Dump SQL File`](#dumping-a-table-or-a-schema-to-a-file)** in the tree instead, which reads every row
out of the server rather than off the grid.

**The cell menu's Save Data As ▸ is the same three formats**, so a reader who is already in the menu
need not travel to the button; **Copy As ▸** beside it puts the selection on the clipboard in the
clipboard's own shapes — tab-separated with field names, CSV, JSON — and treats a `NULL` the way the
file of that format does.

| Format | What it does with a `NULL` |
|---|---|
| CSV | An empty field, which is the only thing a CSV has to say "nothing" with |
| JSON | `null`, which is why a reader who needs to tell it from an empty string wants this one |
| SQL | The `NULL` keyword. Every other value is written as a literal, which both SQL engines read back into a column of any type. The statements are asked of the driver, so the quoting is the engine's; a result no single table can be traced to is named `exported_rows`, which the reader has to correct — the honest outcome, rather than a file that looks right. **On MongoDB the format writes commands, not SQL**: one `db.<collection>.insertOne({…})` per document, with an absent value *left out* rather than written as null — a field nobody wrote is not a field holding nothing, and that is the difference a document store keeps. Each value is read back out of the notation the grid drew it in, so a number exports as a number and an `ObjectId` as one |

### When a result is writable

A result is writable only when the driver can trace its rows to one stored row each.
`sqlkit.Dialect.ParseSource` accepts a plain `SELECT … FROM <table>` with at most a `WHERE`,
`ORDER BY`, `LIMIT` or `OFFSET` after it, and rejects joins, unions, `GROUP BY`, `DISTINCT`, derived
tables and `EXPLAIN`. The named table is then looked up in the catalogue, and the result stays
read-only unless it carries every primary-key column exactly once — an aliased or repeated key
column names no row. The footer says which case applies. This is needed because `database/sql`
reports a column's type but not the table it came from, so the source cannot be read off the wire.

One `Dialect` field decides how a quoted name is read, so `` `shop`.`users` `` is a table on MySQL
and a string on PostgreSQL, where `"shop"."users"` is the table instead. The qualifier the statement
carries is read as whatever the engine's inner level is: a database on MySQL, a schema on
PostgreSQL, whose statements cannot name another database at all.

## Options

The gear at the right of the command bar opens **Options** (`features/settings/SettingsDialog`): a
rail of sections on the left, the one in force on the right, which is the shape a reader already
knows a settings dialog by. Sections are added to that rail — never a second dialog.

| Section | Holds |
|---|---|
| General | **Theme** — dark or light, the same switch the command bar's sun/moon is. |
| Editor | The **data font** — family, size, weight, with a preview line — and **word wrap** for the SQL editor, which reconfigures a live editor through a CodeMirror compartment rather than rebuilding it. |
| Records | What a **new tab** starts with: records per page (a table read and a query result alike) and the row limit a query tab caps its read at. Both are defaults, read once when a tab mounts, so changing one never moves a result the reader has already paged through. |

**A pick applies as it is made**, not behind an OK: the app behind the dialog is the preview, and a
dialog that could be dismissed half-set would need a draft of every preference to do it. The footer
therefore offers **Restore Defaults** and **Done**, and nothing that cancels. Everything is kept in
`localStorage` under one key, `vibe-db.settings`, by `stores/settings.ts` — the one place a
preference is saved and put in force.

### The font list is the machine's

The family list is **every font installed on this machine**, not a list the app ships: `SystemAPI.
ListFonts` reads the Windows font registry — the machine-wide hive *and* the reader's own, since a
font installed for one user is registered only there — strips the `(TrueType)` tag, folds a family's
`Regular`, `Italic`, `Bold` and `Oblique` variants back into it, and answers sorted. `Segoe UI Light`
and `Arial Black` survive whole, because those are families a stylesheet can name; `Arial Bold` is
Arial asked for in bold and is folded away. A few hundred families is normal, so the list is
**searched rather than scrolled**, and every row is drawn in its own family — the only honest preview
of a pick. Outside the desktop shell (a browser dev run) the host answers with nothing and the picker
offers the six families in `FALLBACK_FAMILIES`.

The default is **Cascadia Mono light 13 px**: a screenful of rows reads heavier than a paragraph of
the same weight, so data is set a weight below regular. A weight only draws as itself where the
family has it — Cascadia is variable and does, Consolas and Lucida Console ship 400 and 700 alone, and
Windows then rounds to the nearest they have. Sizes run 10–16 px, which is what a 28 px row can hold.

The pick is written onto the root element as `--font-data`, `--font-data-size` and
`--font-data-weight`, ahead of `--font-data-fallback`, which is why nothing rebuilds, why a family the
machine turns out not to have still lands on something monospaced, and why a column fit measures the
font the reader actually chose.

## Grid behaviour

Every table in the app is one component, `ResultGrid`, and it looks and behaves the same wherever it
appears: a two-line header, drag-selected cell ranges, in-place editing, resizable columns.
**[grid.md](grid.md) is the specification** — read it before changing any table, and add to it rather
than to a call site.
