# Connections

[← Features and UI behaviour](../features.md)

## Picking an engine

**A new connection picks its type before it is filled in.** The dialog opens on a grid of engine
tiles — each engine's own mark at a size that reads by shape — and `Next` moves on to the form; a
double-click on a tile does both. `Back` in the form returns to the grid, which is also how an
existing connection changes engine: editing one opens straight on the form, since it already has a
type.

Everything the form defaults to comes from that choice: the port, the usual administrator's name,
and what the Database box is even *called*. The whole table of it is
[features/engines.md](engines.md#what-a-connection-asks-for). Going back and choosing another engine
rewrites only the fields that still hold the old engine's default, so a typed port survives the
switch. The editor loads that engine's own language for highlighting and completion — a SQL dialect
for five of them, the shell for MongoDB, and commands for Redis; see [editor.md](../editor.md).

**Every connection wears its engine's chip** — the dolphin on teal for MySQL, the elephant on blue
for PostgreSQL, the turned page for SQLite, the window pane for SQL Server, the ring for Oracle, the
leaf on green for MongoDB, the stack of discs on red for Redis — in the tree, in the dialog's header
and in the status bar. It replaces the engine badge the tree used to carry on the right of a
connection row, which is where the **connection status dot** now sits: what the connection *is* on
the left, how it is *doing* on the right.

Some engines differ rather than just defaulting:

- **SQLite has no server.** The General tab is one `Database file` box with a `Browse` beside it, and
  the host, port, user and password are not there at all — see
  [engines.md](engines.md#what-a-connection-asks-for) for why they are absent rather than disabled.
  An SSH tunnel is refused, and so is a file that is not already there.
- **PostgreSQL needs a database.** A session is bound to one for life, so that is the database the
  connection bootstraps from — the one its database list is read in. Reaching any other database dials
  it separately, which the adapter does on first use.
- **Oracle needs a service, not a database.** The box is labelled `Service name` because that is what
  it holds: an Oracle connection asks the listener for a service, and what the tree lists under it are
  *schemas*. Oracle also has no opportunistic TLS — a listener endpoint is TCP or TCPS and the client
  has to know which before it dials — so `Preferred` connects in the clear.
- **SQL Server needs none.** One pool reads every database on the server, its names carrying both the
  database and the schema, so a database named here is a *starting point*: it is where a statement
  carrying no namespace goes.
- **MongoDB needs none**, for the same reason and with the same meaning. TLS is on or it is not — the
  driver has no opportunistic mode — so `Preferred` connects in the clear, and the two cloud identity
  methods are refused outright rather than sent as a password no MongoDB authentication mechanism
  would take.
- **Redis takes a database *number*.** Its databases are not named but numbered — `0` to `15` on a
  default install — and the set is fixed when the server starts, so the box takes an index and blank
  means `0`. Anything that is not a number is refused before anything is dialled. The two cloud
  identity methods are refused here too, for the same reason they are on MongoDB.

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
  perfectly well from `ssh` or from another client; see [decisions.md](../decisions.md).
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

## What a right-click on a connection offers

The connection's own menu is the one place everything about a *server* is asked for, and it is
ordered by what a reader is there to do rather than by what the app can do:

| | |
|---|---|
| `Connect` / `Disconnect` | Whichever it is not already. |
| `Refresh` | Re-reads the database list, and forgets everything under a database the server no longer carries — schemas, tables, shapes. |
| `Console` | A [console](console.md#the-console) on this connection. |
| `Execute SQL File…` | [Runs a file](console.md#execute-sql-file) of statements against it. |
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
