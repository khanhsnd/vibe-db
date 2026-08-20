# What a service keeps about one object

[← Features and UI behaviour](../features.md)

Picking a row of a [bucket](object-storage.md) opens the object below the grid. This is what
is in there, what each pane costs, and why two of the seven are not what a reader might expect.

There are **seven panes** because a service keeps seven different kinds of fact about an object and
each is a different question. Folding them into one column would be a wall nobody reads.

| Pane | What it is | What it costs |
|---|---|---|
| **Properties** | the facts that mean something in particular — size, type, class, when, and the uploader's own metadata | the head |
| **Header** | the **whole** of what the service answered, sorted by name — and the six of them that can be [changed](#changing-a-header-is-a-copy-of-the-object-onto-itself) | the same head |
| **Permissions** | the object's access-control policy: the owner, and who may do what | one request |
| **Tags** | the service's own key-value labels, which are **not** the uploader's metadata | one request |
| **Preview** | as much of it as is worth putting on a screen | one ranged read |
| **Versions** | the last twenty, newest first, delete markers included | one request |
| **Log** | what **this app** did about this object | nothing at all |

**A pane is read when it is opened and never before.** Picking a row costs *one* request — the head,
which Properties and Header are both drawn from — and the rest cost one each, the first time a reader
asks. A panel that read all seven on selection would make walking a folder of fifty objects three
hundred requests, which is the cost this whole feature is built to avoid.

**A preview is a byte range and never the object.** Opening a four-gigabyte log to read its first
lines costs half a megabyte, and the note under it says so: *this is a preview, not the object*. A
picture is drawn rather than read, and one too large to draw is not drawn **cut in half** — half a
JPEG is a grey box, and a reader would take it for a broken object.

Two numbers, and both were chosen by looking at what a bucket actually holds. Text is read to **half a
megabyte**, which is more than anybody reads of a log. A picture is read to **eight**, which is the
one that had to change: at two, the commonest picture in any bucket — a photograph off a phone, a
screenshot of a dashboard — was over the bound and the reader was told to download it instead, which
is a preview that does not preview the things people look at.

**A type the service never set falls back to the name.** Most objects in most buckets were uploaded by
something that said nothing about what they were, so a `.png` arrives as `application/octet-stream` —
and a panel that trusted the service alone drew it as text, which is how an image that would not draw
was found. Where the service says something, that is what is used; where it says one of the two
spellings of "some bytes", the extension is the only other fact there is. **Properties still shows
what the service said**, because that is the truth about the object and the thing a reader would go
and fix — with [Header](#changing-a-header-is-a-copy-of-the-object-onto-itself), now that they can.

**Permissions and Tags are read and never written.** Writing an ACL is a whole grammar of canned
names and grant lists, and a client that let a reader edit half of it would be a client that quietly
dropped the other half. Not every service has either — Cloudflare R2 has no ACLs at all — so what a
reader is shown is **the service's own refusal**, which is a different and truer sentence than an
empty table saying "nobody may do anything".

**One grant is drawn loudly**: a read granted to the group `AllUsers` is an object the whole internet
can fetch, and the pane says so at the top rather than leaving a reader to spot a URI in a table.

**Versions is empty on most buckets, and that is not an error**: a bucket only keeps a history where
versioning was turned on before the object was written. The request asks about a *prefix* — the
protocol has no way to ask about one object — so `report.csv.bak` is filtered out here rather than
shown as a version of `report.csv`.

## Changing a header is a copy of the object onto itself

**There is no request that sets a header.** What there is, is a copy of the object onto its own key
with `x-amz-metadata-directive: REPLACE` and the new set — and the whole shape of the Header pane
follows from that one fact.

**Six headers can be changed, and they are fixed by the protocol**: `Content-Type`,
`Content-Disposition`, `Content-Encoding`, `Content-Language`, `Cache-Control`, `Expires`. Anything
else the reader types becomes an `x-amz-meta-` pair, which is the other half of what a copy will
take. The list travels **from the Go side** with the object rather than being spelled in the pane, so
the two cannot come to disagree about what a service accepts.

**Everything else is drawn with a lock.** The tag, the length, the last-modified, the request id, the
server's own name — those belong to the service and no request writes them. Offering to edit them
would be offering something that does not exist, so the pane says so with a padlock and a second
heading rather than by silently ignoring an edit.

**A change is staged and applied in one press.** That is not a preference: the request *replaces the
set entire*, so a header left out of it is a header the object stops having. A per-row write would
mean one rewrite of the object per keystroke, and a row deleted between two of them would come back.
The bar under the rows is `Add`, `Edit`, `Delete`, `Discard` and `Apply`; a click on a value opens its
editor, a click on a name picks the row, and what is staged is washed in the accent until it goes.

**Apply rewrites the object**, and the confirmation says so plainly: a new ETag, a new modified time,
and a new version on a bucket that keeps them. The storage class is **carried through** — a copy that
named none would move the object to the service's default, which is a bill nobody asked for — and an
object past five gigabytes cannot be copied in one request at all, so this app refuses it by name
rather than letting the service refuse a copy the reader never asked for.

It is a `write` on [`domain.ObjectHazard`](object-storage.md#what-a-reader-can-do-to-a-bucket) — so a
production connection asks first and a read-only one does not draw the bar at all. What comes back
from an Apply is the object **read again**, because what a reader has to see afterwards is what the
service now says rather than what they typed.

## The Log pane is this app's own, not the service's

There is no per-object log in the protocol. A service's *access* log is a bucket setting that writes
log objects into another bucket, is usually not turned on, and would have to be found, listed,
downloaded and parsed — so a client showing one would either be showing nothing or showing somebody
else's gigabytes.

What a reader actually wants from a Log tab is *what did this program just ask for, and what came
back* — which only this side can answer. So that is what it is: every request this app made about the
object, newest first, with the code it came back with (`ok`, `FORBIDDEN`, `UNKNOWN_OBJECT`) and how
long it took. A missing permission shows up there as the request that was refused rather than as a
dialog somebody has already dismissed.

It is a **window and not a ledger**: the last five hundred requests, in memory, gone when the app
closes. An entry for a *folder* — a folder deleted, a folder downloaded — shows up under every object
that was inside it, because "the folder above this was deleted" is the answer a reader is looking for.

## The ETag is not always a checksum

The panel says which. An object the service took in **one** request has an ETag that is an MD5 of its
bytes, and a download of it is verified against that sum. An object written in **parts** has an ETag
that is a sum of sums with a part count after it, so it is not a checksum of anything a client can
compute — and a download of such an object reports having checked its **length** and nothing more.

Saying so is the point. A client that claimed to verify every download would be wrong about most of
the large ones.
