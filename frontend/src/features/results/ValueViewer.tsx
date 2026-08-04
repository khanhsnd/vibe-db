import { useEffect, useMemo, useState } from 'react'

import { cn } from '@/lib/cn'

import { DocumentView } from './DocumentView'
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
 * One cell's value, shown as the thing it *is*. Which views a value offers is read off the value
 * rather than off its column — see `value.ts` — because every value crosses the wire as text and a
 * `TEXT` column holds JSON as often as a `JSON` one does.
 *
 * `Raw` is always among them and is always exactly what the cell holds, so however clever the
 * detection is there is one view that cannot be wrong. It is also the only one that is editable:
 * a laid-out document, a hex dump and a drawing are all *renderings*, and writing one back would
 * write the rendering.
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
  const found = useMemo(() => detect(value, columnType), [value, columnType])
  // The *presence* of a formatter decides the views, never its identity: an unstable one would
  // otherwise rebuild the list every render and knock the reader back to the first tab.
  const laidOut = formatValue !== undefined
  const views = useMemo(() => viewsFor(found.kind, laidOut), [found.kind, laidOut])

  const [view, setView] = useState(views[0] ?? 'raw')
  // A different cell is a different value, and the view it was last read in may not exist here.
  useEffect(() => {
    setView(views[0] ?? 'raw')
  }, [views])

  return (
    <div className={cn('flex min-h-0 flex-col', className)}>
      <div className="flex h-7 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5">
        {views.map((one) => (
          <button
            key={one}
            type="button"
            aria-pressed={one === view}
            onClick={() => setView(one)}
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

      <View view={view} value={value} onChange={onChange} formatValue={formatValue} />
    </div>
  )
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
  onChange,
  formatValue,
}: {
  view: ViewKind
  value: string | null
  onChange?: ((next: string) => void) | undefined
  formatValue?: ((value: string) => Promise<string>) | undefined
}) {
  if (value === null) {
    return <Note>This cell holds NULL.</Note>
  }

  switch (view) {
    case 'json':
      return <DocumentView text={prettyJSON(value)} notation="json" />

    case 'document':
      return <Document text={value} formatValue={formatValue} />

    case 'map':
      return <GeoShape shape={JSON.parse(value) as GeoJSON} />

    case 'list':
      return <Items items={parseArray(value) ?? []} />

    case 'hex':
      return <Binary value={value} />

    case 'image':
      return <Picture value={value} />

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
 * A document store's own document, laid out by the engine that wrote it — `driver.Text.Format`, over
 * the same tokens its editor and its `Review` read. There is no second formatter on this side: a
 * bare key, an `ObjectId(…)` and where a line breaks are all that engine's grammar.
 *
 * It goes up **unformatted first** and is replaced when the layout arrives, so the pane is never
 * blank and a format that cannot run leaves the document readable rather than gone.
 */
function Document({
  text,
  formatValue,
}: {
  text: string
  formatValue?: ((value: string) => Promise<string>) | undefined
}) {
  const [laid, setLaid] = useState(text)

  useEffect(() => {
    setLaid(text)
    if (!formatValue) return

    let live = true
    void formatValue(text).then(
      (next) => {
        if (live) setLaid(next)
      },
      // A layout is a convenience, not the value: the text above already says what the cell holds.
      () => {},
    )
    return () => {
      live = false
    }
  }, [text, formatValue])

  return <DocumentView text={laid} notation="shell" />
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
