import { useEffect, useMemo, useRef, useState } from 'react'

import { cn } from '@/lib/cn'

import { DocumentView, type Notation } from './DocumentView'
import { GeoShape } from './GeoShape'
import {
  decodeBinary,
  detect,
  hexDump,
  imageType,
  parseArray,
  prettyJSON,
  type GeoJSON,
} from './value'

/** How much of a blob is dumped. Beyond this a reader is not reading bytes, they are scrolling. */
const HEX_LIMIT = 4096

/**
 * JSON laid out, in the shape a layout arrives in. It is the platform's `JSON.stringify` where a
 * document store's own notation is the *engine's* `Format`, and both reach the same view.
 */
const layJSON = (value: string) => Promise.resolve(prettyJSON(value))

/**
 * One cell's value, shown as the thing it *is*. Which views a value offers is read off the value
 * rather than off its column — see `value.ts` — because every value crosses the wire as text and a
 * `TEXT` column holds JSON as often as a `JSON` one does.
 *
 * **A value is written back in whichever view it is read in**, where the surface can write at all:
 * `Raw`, the laid-out `JSON` and the laid-out `Document` are all typed into, because a document is
 * the thing a reader most often opens the viewer to change and sending them to `Raw` to change it
 * is sending them to the one view that does not lay it out. The rest — a hex dump, a drawing, a
 * list of items — are renderings of bytes and shapes rather than of text, and writing one back
 * would write the rendering; `Raw` is beside them for that.
 */
export function ValueViewer({
  value,
  columnType,
  /** Absent for a read-only surface, exactly as `ResultGrid.onEditCell` is. */
  onChange,
  formatValue,
  className,
}: {
  value: string | null
  columnType?: string | undefined
  onChange?: ((next: string) => void) | undefined
  /**
   * Lays a value out in the engine's own notation — the driver's `Format`, since **how text is laid
   * out is lexical and therefore the engine's** and a document store's documents are not JSON. A
   * surface that cannot reach a session leaves it out, and a document then offers `Raw` alone.
   *
   * It must be **referentially stable**: the document view calls it from an effect, and one rebuilt
   * every render would never settle.
   */
  formatValue?: ((value: string) => Promise<string>) | undefined
  className?: string | undefined
}) {
  const { arrived, reread, write } = useArrived(value, onChange)

  const found = useMemo(() => detect(arrived, columnType), [arrived, columnType])
  // The *presence* of a formatter decides the views, never its identity: an unstable one would
  // otherwise rebuild the list every render and knock the reader back to the first tab.
  const laidOut = formatValue !== undefined
  const views = useMemo(() => viewsFor(found.kind, laidOut), [found.kind, laidOut])

  const [view, setView] = useState(views[0] ?? 'raw')
  // A different cell is a different value, and the view it was last read in may not exist here.
  useEffect(() => {
    setView(views[0] ?? 'raw')
  }, [views])

  /**
   * Opening another view reads the value again as it now stands, so an edit typed into `Raw` is
   * what the Document view lays out — and a value edited into something else is read as that.
   */
  const show = (one: ViewKind) => {
    setView(one)
    reread()
  }

  return (
    <div className={cn('flex min-h-0 flex-col', className)}>
      <div className="flex h-7 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5">
        {views.map((one) => (
          <button
            key={one}
            type="button"
            aria-pressed={one === view}
            onClick={() => show(one)}
            className={cn(
              'h-5 rounded px-1.5 text-2xs font-medium transition-colors',
              one === view ? 'bg-accent-subtle text-accent' : 'text-muted hover:bg-hover',
            )}
          >
            {VIEW_LABEL[one]}
          </button>
        ))}

        <div className="min-w-2 flex-1" />
        <span className="shrink-0 font-mono text-2xs text-faint">{found.label}</span>
      </div>

      <View
        view={view}
        value={value}
        opened={arrived}
        onChange={write}
        formatValue={formatValue}
      />
    </div>
  )
}

/**
 * The value as it last **arrived** — the cell the viewer opened on, a Discard, a NULL ticked —
 * rather than as it is being typed. What a reader types comes straight back here as the same prop,
 * and reading a keystroke as a new value would take a half-typed document for plain text and pull
 * its own tab out from under the reader, and lay the line out again around the caret.
 *
 * `write` is what a view changes the value through, and `reread` is how opening another view takes
 * what is there now.
 */
function useArrived(value: string | null, onChange?: (next: string) => void) {
  const typed = useRef<string | null>(null)
  const [arrived, setArrived] = useState(value)

  if (value !== arrived && value !== typed.current) setArrived(value)

  return {
    arrived,
    reread: () => setArrived(value),
    write:
      onChange &&
      ((next: string) => {
        typed.current = next
        onChange(next)
      }),
  }
}

type ViewKind = 'raw' | 'json' | 'document' | 'map' | 'list' | 'hex' | 'image'

const VIEW_LABEL: Record<ViewKind, string> = {
  raw: 'Raw',
  json: 'JSON',
  document: 'Document',
  map: 'Shape',
  list: 'Items',
  hex: 'Hex',
  image: 'Image',
}

/**
 * Which views a value offers, most useful first — that first one is what opens. The order is the
 * point: a reader who opened a GeoJSON cell wants to see the shape, and one who opened a blob
 * wants to know whether it is a picture before they want its bytes.
 *
 * A **document** is the one view a value can be denied: laying it out is the driver's, so a surface
 * with no session to ask has nothing to lay it out with and offers what cannot be wrong instead.
 */
function viewsFor(kind: ReturnType<typeof detect>['kind'], laidOut: boolean): ViewKind[] {
  switch (kind) {
    case 'geojson':
      return ['map', 'json', 'raw']
    case 'json':
      return ['json', 'raw']
    case 'document':
      return laidOut ? ['document', 'raw'] : ['raw']
    case 'array':
      return ['list', 'raw']
    case 'binary':
      return ['image', 'hex', 'raw']
    default:
      return ['raw']
  }
}

function View({
  view,
  value,
  opened,
  onChange,
  formatValue,
}: {
  /** What the cell holds now, which is what `Raw` shows and what typing into it changes. */
  value: string | null
  /** That same value as this view opened on it — every laid-out view reads this one. */
  opened: string | null
  view: ViewKind
  onChange?: ((next: string) => void) | undefined
  formatValue?: ((value: string) => Promise<string>) | undefined
}) {
  if (value === null || opened === null) {
    return <Note>This cell holds NULL.</Note>
  }

  switch (view) {
    case 'json':
      return <Laid text={opened} notation="json" lay={layJSON} onChange={onChange} />

    case 'document':
      return <Laid text={opened} notation="shell" lay={formatValue} onChange={onChange} />

    case 'map':
      return <GeoShape shape={JSON.parse(opened) as GeoJSON} />

    case 'list':
      return <Items items={parseArray(opened) ?? []} />

    case 'hex':
      return <Binary value={opened} />

    case 'image':
      return <Picture value={opened} />

    default:
      return (
        <textarea
          value={value}
          readOnly={!onChange}
          spellCheck={false}
          onChange={(e) => onChange?.(e.target.value)}
          className="data-font scroll-thin block min-h-0 w-full flex-1 resize-none bg-canvas px-3 py-2 leading-relaxed text-fg outline-none read-only:text-muted"
        />
      )
  }
}

/**
 * A value laid out for reading and typed back in the notation it was laid out in. JSON is laid out
 * by the platform; a document store's own notation — bare keys, `ObjectId(…)`, `ISODate(…)` — by
 * `driver.Text.Format`, over the same tokens its editor and its `Review` read. There is no second
 * formatter on this side: a bare key and where a line breaks are that engine's grammar.
 *
 * It goes up **unformatted first** and is replaced when the layout arrives, so the pane is never
 * blank and a format that cannot run leaves the value readable rather than gone. What is typed into
 * it goes back as the notation it was shown in, which is the notation that engine's parser reads.
 */
function Laid({
  text,
  notation,
  lay,
  onChange,
}: {
  text: string
  notation: Notation
  lay?: ((value: string) => Promise<string>) | undefined
  onChange?: ((next: string) => void) | undefined
}) {
  const [laid, setLaid] = useState(text)

  useEffect(() => {
    setLaid(text)
    if (!lay) return

    let live = true
    void lay(text).then(
      (next) => {
        if (live) setLaid(next)
      },
      // A layout is a convenience, not the value: the text above already says what the cell holds.
      () => {},
    )
    return () => {
      live = false
    }
  }, [text, lay])

  return <DocumentView text={laid} notation={notation} onChange={onChange} />
}

function Monospace({ text }: { text: string }) {
  return (
    <pre className="data-font scroll-thin selectable min-h-0 flex-1 overflow-auto px-3 py-2 text-fg">
      {text}
    </pre>
  )
}

function Items({ items }: { items: string[] }) {
  if (items.length === 0) return <Note>This array is empty.</Note>

  return (
    <ol className="scroll-thin min-h-0 flex-1 overflow-auto py-1">
      {items.map((item, index) => (
        <li key={index} className="flex gap-2 px-3 py-0.5">
          <span className="w-8 shrink-0 text-right font-mono text-2xs text-faint">{index}</span>
          {/* An unquoted NULL inside a PostgreSQL array is the absence of a value, and is drawn
              in the same colour the grid draws one. */}
          <span
            className={cn('data-font selectable min-w-0 break-all', item === 'NULL' && 'text-null')}
          >
            {item}
          </span>
        </li>
      ))}
    </ol>
  )
}

function Binary({ value }: { value: string }) {
  const bytes = useMemo(() => decodeBinary(value), [value])
  const lines = useMemo(() => hexDump(bytes, HEX_LIMIT), [bytes])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <Monospace text={lines.join('\n')} />
      {bytes.length > HEX_LIMIT ? (
        <p className="shrink-0 border-t border-line px-3 py-1 text-2xs text-faint">
          The first {HEX_LIMIT.toLocaleString()} bytes of {bytes.length.toLocaleString()}.
        </p>
      ) : null}
    </div>
  )
}

/**
 * The picture a blob holds, where its first bytes say it is one. The bytes are already here, so
 * the data URI reaches nothing: nothing in a reader's database is ever sent anywhere to be drawn.
 */
function Picture({ value }: { value: string }) {
  const source = useMemo(() => {
    const bytes = decodeBinary(value)
    const media = imageType(bytes)
    return media === null ? null : `data:${media};base64,${base64(bytes)}`
  }, [value])

  if (source === null) {
    return <Note>These bytes do not begin like an image. The Hex view is beside this one.</Note>
  }

  return (
    <div className="scroll-thin grid min-h-0 flex-1 place-items-center overflow-auto p-3">
      <img src={source} alt="The image this cell holds" className="max-h-full max-w-full" />
    </div>
  )
}

function Note({ children }: { children: React.ReactNode }) {
  return <p className="min-h-0 flex-1 px-3 py-2 text-xs text-muted">{children}</p>
}

/** `btoa` takes a string of code points below 256, which is exactly one byte each. */
function base64(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}
