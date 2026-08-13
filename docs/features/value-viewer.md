# Reading a value whole

[← Features and UI behaviour](../features.md)

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
| **HTML** — a value whose **first tag names an HTML element** | **Page**, then **Raw**. A leading `<` is not enough and never was: an XML document begins with one, so does a `<?xml` declaration, and so does a `<foo>` of somebody's own — so what decides is the tag, which is the same bargain JSON's leading `{` strikes. A fragment opening with prose — `Hello <b>world</b>` — reads as text, exactly as a `42` that happens to parse reads as a number |
| **Bytes** — a `\x…` value | **Image**, then **Hex**, then **Raw**. A reader who opened a blob wants to know whether it is a picture before they want its bytes |
| Anything else | **Raw** alone |

- **`Raw` is always there and is always exactly what the cell holds**, so however clever the
  detection is there is one view that cannot be wrong.
- **A value is written back in whichever view it is read in**, wherever the surface can write at
  all: `Raw`, the laid-out `JSON` and the laid-out `Document` are all typed straight into, because a
  document is the thing a reader most often opens the viewer to *change* and sending them to `Raw`
  to change it is sending them to the one view that does not lay it out. What is typed goes back in
  the notation it was shown in, which is the notation that engine's own parser reads. The rest —
  **Shape**, **Items**, **Hex**, **Image**, **Page** — are renderings of a shape, of bytes or of
  markup rather than of text, and writing one back would write the rendering; `Raw` is beside them
  for that. Which is why a writable cell editor **opens on `Raw`** where the value is a PostgreSQL
  array or a page: the reader would otherwise have to leave the first tab before they could type.
- **What a value *is* is read when it arrives, not while it is typed.** The tab strip follows the
  cell the viewer opened on, a Discard or a NULL ticked — never a keystroke, or half a typed
  document would count as plain text and take its own tab away mid-word. Opening another view reads
  the value again as it now stands, so an edit made in `Raw` is what the Document view lays out.
- **Every view is selectable, laid-out ones included.** `user-select: none` is global — see
  [frontend.md](../frontend.md) — so a rendering that did not opt back in was a document nobody could
  copy out of.
- **The viewer is where a long value is read, and the grid is not.** A cell draws a clipped preview
  of it — see [grid.md](../grid.md) — because a cell is one line a couple of hundred pixels wide. This
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
- **A page is drawn in a frame that may do nothing at all.** A cell's markup is somebody else's text
  and this window holds the app's own bridge to their database, so it goes into an `<iframe>` with an
  opaque origin, no scripts, no forms and no way to navigate anything — `sandbox` with nothing
  allowed back — and a `Content-Security-Policy` inside the frame as well: `default-src 'none'`, an
  image only from `data:`, a style only inline. So nothing in a reader's database is ever sent
  anywhere to be drawn, which is the same rule a shape is drawn without a tile server under and an
  image is drawn from the bytes already in hand. A second policy cannot loosen the first, so markup
  carrying a `<meta>` of its own is no way round it.
- **The page is drawn on the app's own ground, not on a colour of its own.** A frame inherits no
  custom property, so the tokens the app is themed with are copied into it — the canvas, the
  foreground, the UI font, a link's accent — and each is a rule the value's own stylesheet overrides.
  They are copied again when the reader switches theme under an open preview.
- The hex dump is offset, hex and printable characters, sixteen bytes to a line, capped at 4 096
  bytes — beyond that a reader is not reading bytes, they are scrolling — and the cap says how much
  of how many it showed.

## Why bytes arrive as `\x…`

**A value that is not valid UTF-8 crosses the wire as hex**, written the way PostgreSQL already
spells a `bytea` so that one form means one thing on every engine. `sqlkit.TextOrNil` is the single
place that decides it.

It is not a preference. A bound method's result reaches the WebView as one JSON string, and a byte
sequence that is not text is replaced by the replacement character on the way — a blob arriving as
a row of diamonds — and what has been replaced can never be recovered. Hex is lossless, and it is
what the viewer decodes to show the bytes, the image or the dump.

**Bytes that *are* text stay text.** A `BLOB` holding JSON is far commoner than one holding a PNG,
and a reader who stored text wants to read it rather than decode it. Nothing consults the column's
type to decide this: the bytes themselves are the evidence, which is also why it cannot be wrong
about a `TEXT` column an engine happens to report as a `BLOB`.
