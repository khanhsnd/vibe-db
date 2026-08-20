# Browsing an object store

[← Features and UI behaviour](../features.md)

**An object store is a second kind of connection, not an eighth engine.** It sits in the same
sidebar, wears the same colour and the same environment stripe, is carried over the same proxy and
the same bastion — and shares nothing else with a database, because there is nothing else to share:
no catalogue, no namespace, no statement, and nothing that is a row.

What it is instead is two levels deep and stops there.

| Level | What it is |
|---|---|
| the connection | one service and one key pair — see [the dialog](#the-dialog) |
| a **bucket** | what the credentials can list, or the one the profile names |
| below that | **not in the tree.** A folder inside a bucket is drawn in the grid, a page at a time — see [why](#why-a-folder-is-not-a-node) |

## Where it is asked for

**Three places, because they are the three a reader reaches for**, and all three build from one list
(`app/layout/newMenu.ts`) so they cannot come to disagree:

| Surface | How the two are offered |
|---|---|
| `New Connection` on the **command bar** | a menu of the two. It is the button a reader actually reaches for, so it is where both kinds have to be |
| `+` in the **sidebar** header | the same menu |
| the **empty state** of a first run | two **buttons**, which is the one place they are not a menu: a reader with nothing on the screen is choosing what kind of thing they have, and a menu would hide half the answer behind a press |

An object store is deliberately **not** in the engine list of `New Connection`'s own dialog. It is not
an eighth engine — there is no catalogue, no statement and nothing that is a row — and putting it
there would promise a query tab, a designer and a table list that a bucket cannot have.

## The dialog

What the store's dialog asks for is an address, a key pair and — optionally — one bucket.

**Picking a service fills three boxes in**, and that is the whole reason the list of them exists.
Every service here speaks the same protocol, so what differs between them is three facts a reader
would otherwise have to know by heart, and each of them is a 404 with nothing in it when it is wrong:

| Service | Endpoint | Bucket named in | Region |
|---|---|---|---|
| Amazon S3 | worked out from the region | the host | the reader's |
| MinIO | theirs, and usually `http://` | the **path** | `us-east-1`, which it ignores |
| Cloudflare R2 | `<account>.r2.cloudflarestorage.com` | the host | `auto`, and only `auto` |
| DigitalOcean Spaces | `fra1.digitaloceanspaces.com` | the host | the endpoint's first label |
| Wasabi | `s3.<region>.wasabisys.com` | the host | the reader's |
| Google Cloud Storage | `storage.googleapis.com` | the **path** | `us-east-1` |
| Other S3-compatible | theirs | the **path** | `us-east-1` |

Every one of those is a **default and not a rule**: the profile carries its own flag for each, so a
reader with a wildcard DNS entry in front of their own MinIO can have the other form.

**A missing scheme is `https`.** That is the one default in this dialog that is a security decision
rather than a convenience — a bucket reached in the clear is a bucket whose keys are on the wire — so
a reader who means plain text says `http://` out loud. The certificate is **verified** by default,
which is the one place in this app a TLS mode is picked rather than left at the engine's own.

**Signing in** is a key pair or the machine's own AWS identity. The second reads
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN` **at the moment of connecting**
and stores nothing, which is what makes an SSO login or an assumed role work here — the arrangement
[an RDS IAM token](connections.md) has, applied to a credential that may be temporary.

**A named bucket is an answer, not a filter.** A key that may read one bucket and not list them is
the ordinary arrangement in an organisation, and a tree that could only start from a listing would
show such a reader nothing at all. Named, the tree holds that bucket alone; empty, it holds every
bucket the key can list.

**Test Connection asks for something.** Opening a store dials nothing — every call to the service is
a request of its own — so a profile that could not reach anything would otherwise pass. What it asks
for is the buckets, or the named bucket's first page.

## Why a folder is not a node

A prefix inside a bucket is drawn in the **grid** and never in the tree, which is the same decision
[a key space](key-space.md#the-keys-are-drawn-as-the-tree-they-spell) makes about its colons and for
a related reason.

There, the tree is a *reading* of a page the app already holds. Here it is stronger than that: the
folders come **from the service**. A listing asked for with a delimiter answers with the prefixes it
found and the objects directly inside, so a folder is one request — and browsing a bucket of forty
million objects costs exactly what browsing one of forty costs.

A tree of prefixes would be a listing per branch and a bucket has no bottom. This is the load-bearing
decision of the whole feature, and it is why there is **no index anywhere in it**.

## The grid

Opening a bucket opens a [`ResultGrid`](../grid.md), six columns wide. They are not read from
anywhere and not inferred from anything: they are the six facts an S3 listing carries for every key,
on every service there is — which is what a bucket has instead of a catalogue.

| Column | What it is |
|---|---|
| `Name` | the tail inside the folder it was listed in, and the **whole key** in a search's answer. It carries the row's **glyph** — see below |
| `Size` | bytes, and **empty** on a folder row: a prefix has no size, and a zero would be claiming one |
| `Kind` | `Folder`, or the extension — the only thing a *listing* knows about what is inside an object |
| `Modified` | when it was last written |
| `Storage Class` | the service's own word for it |
| `ETag` | the service's tag for the bytes, which is [not always a checksum](object-panel.md#the-etag-is-not-always-a-checksum) |

**Folders come first**, which is what every file manager does and what a reader walking a path is
looking for. The path above the grid is a row of buttons, so walking back out of six folders is one
press.

**Every row carries a glyph, and it is drawn by shape rather than by colour.** A folder is a folder;
an object is what its name says it is — a picture, a spreadsheet, a page of prose, something packed,
something sealed, a dump. At thirteen pixels a logo *is* a shape, which is the same reasoning the
[provider's chip](#the-dialog) is drawn by, and it is what lets a reader pick the one archive out of
a folder of two hundred logs before reading a word.

There are **two colours and not fifteen**: a folder's and an object's. `--kind-*` is one token per
kind of thing the app holds, so a token per file extension would be inventing a fifteenth kind of
thing — and a listing knows the *name* and nothing else about what is inside, so anything
unrecognised is a plain sheet rather than a guess.

**One click tells you about the row; a double-click acts on it.** A press puts the object in
[the panel below](object-panel.md) — a bucket's rows are things rather than values, so
"tell me about this one" is what a press means here, which is the arrangement a [key
space](key-space.md) has. A double-click walks into a folder or opens an object. A **folder shows
nothing** in the panel: it is a prefix the reader is walking through, and there is no object there to
describe.

**A right-click offers what the toolbar does**, built from the same action set — so a button and a
menu item cannot come to mean different things, which matters most for the one that deletes. It is a
short menu on purpose: the built-in one is written for a table's rows, and there is no NULL to set on
an object, no UUID worth minting into one, no condition to add and no sort worth offering over a
page cut from a service's own order. What is left is what a reader does — open it, bring it down, put
one up, copy a link, rename it, delete it — and a delete of a **folder** says so in the item itself,
because a folder stands for everything under it.

**A page is forward only.** A continuation token is what a service gives instead of an offset, so the
footer has a Back and a Next and a number that is only how many pages have been read — never "page 3
of 40", which would mean knowing how many objects there are, which is a read of the whole bucket.
The footer's totals are **this page's** and say so, for the same reason.

**Flatten** asks the service for no delimiter: every key under the folder, however deep, and no
folders at all. The keys are then grouped on the slash into an
[outlined column](../grid.md) — which is `keyRows`, the same flattener a key space's colons go
through, because a bucket and a key space are both one flat namespace whose names are paths.

## The panel under the grid

Picking a row opens the object below the grid, as seven panes — see
[what a service keeps about one object](object-panel.md), which is its own file because seven panes
over four requests is a feature rather than a paragraph.

## What a reader can do to a bucket

Every one of these is answered for by rules that arrive **with the session** — what it is called, how
dangerous it is, whether it has to be confirmed, and whether it will be sent at all. They are
`domain.ObjectHazard` and `domain.Access` read once, so nothing in the UI re-derives them and a
read-only store simply does not draw the buttons.

| Action | Hazard | Answered for |
|---|---|---|
| Upload, New Folder, New Bucket | write | on a guarded connection |
| Copy | write | on a guarded connection |
| **Change Headers** | write | on a guarded connection — and it [rewrites the object](object-panel.md#changing-a-header-is-a-copy-of-the-object-onto-itself) |
| Delete (the rows that were ticked) | destructive | on a guarded connection |
| Move | destructive | on a guarded connection |
| **Delete Folder** | **unfiltered** | **always** |
| **Empty Bucket** | **unfiltered** | **always** |
| **Delete Bucket** | **unfiltered** | **always** |

The three at the bottom name **nothing** — they stand for everything under them, however much that
turns out to be — which is exactly what makes them the class this rule exists for: `DELETE FROM
users` with no `WHERE` is a mistake on a laptop as surely as in production, and so is emptying a
bucket. What the dialog says is the honest sentence: *however many objects that turns out to be. A
bucket does not say how many until they have been read.*

**New Folder writes a zero-length object ending in `/`**, which is what every console writes and the
only way to have an empty folder at all — a bucket has no folders, a prefix being a reading of the
keys in it. The app says that rather than pretending a folder was created.

**Copy Link** is a signed URL that reads one object without a credential, good for fifteen minutes.
**Nothing is sent**: the signature goes in the query, so the link is made on this machine. It is the
one action here a read-only connection still offers, because reading is what such a connection is
for.

## What a store has not got

A bucket has no columns, so there is no Filter & Sort panel: it is [searched
instead](object-search.md). It has no statement, so there is no query tab, no console and no history.
It has no rows, so nothing here is edited in a cell — an object is replaced by uploading over it.
There is no versioning, no lifecycle rule, no bucket policy and no ACL: each of those is a document
in a grammar of its own, and a client that showed one it could not fully read would be worse than one
that showed none.

## Where it lives

| Part | Where |
|---|---|
| the profile, its hazards, its provider presets | `internal/domain/objects.go` |
| the port, the walk, the search, the download, the upload, the queue | `internal/objectstore` |
| the one adapter — REST and SigV4, no SDK | `internal/objectstore/s3` |
| the profiles on disk (`object-stores.json`) | `internal/profile/jsonfile/stores.go` |
| the open connections, and the read-only guard | `internal/session/stores.go` |
| the bound methods and their wire types | `internal/app/store_api.go`, `store_dto.go` |
| the bridge | `frontend/src/bridge/objects.ts` |
| the tree, the grid, the panel, the dialogs | `frontend/src/features/storage` |
