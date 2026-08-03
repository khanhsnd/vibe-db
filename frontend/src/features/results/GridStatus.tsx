import { useEffect, useState } from 'react'
import { Check, Copy } from 'lucide-react'

import { copyText } from '@/bridge/clipboard'
import { IconButton } from '@/shared/ui/IconButton'

const numbers = new Intl.NumberFormat('en-US')

const MORE_ON_SERVER = 'The read stopped at its row limit, so the server has more rows than this.'

/** Long enough for the tick to be seen, short enough that the bar is not left saying it. */
const COPIED_MS = 1200

/** The age is read in whole seconds at finest, so a second is as often as it can change. */
const AGE_TICK_MS = 1000

/**
 * How many words of the tail are held back from the ellipsis. A read's bounds are the last thing in
 * the statement and the only part of it a page turn moves, so they are exactly what must not be
 * clipped — and `LIMIT 100 OFFSET 0` is four words on both engines. Clipping the column list
 * instead costs nothing, it being the same list on every page.
 */
const PINNED_WORDS = 4

interface GridStatusProps {
  /** The statement that produced the rows, exactly as it ran. */
  statement: string
  /** 1-based place of the row in front, among everything the read holds. */
  record: number
  total: number
  /** A floor rather than a count, the read having stopped at a cap. */
  partial?: boolean
  page: number
  pageCount: number
  /** When the rows landed, so how stale they are can be read off the bar. */
  loadedAt: number | null
}

/**
 * The line under the footer: what ran, where the reader is in what it returned, and how long ago
 * it returned. It is a *status* line rather than a strip of controls, so it carries
 * `--surface-panel` and not the footer's `--surface-bar` — the shade is what says the two are
 * different kinds of thing — and it holds exactly one control, the copy of the statement.
 */
export function GridStatus({
  statement,
  record,
  total,
  partial = false,
  page,
  pageCount,
  loadedAt,
}: GridStatusProps) {
  const age = useAge(loadedAt)
  const { head, tail } = splitStatement(statement)

  return (
    <div className="flex h-6 shrink-0 items-stretch border-t border-line bg-panel text-2xs text-muted">
      <div className="flex min-w-0 flex-1 items-center gap-1 px-2">
        {statement === '' ? (
          // A read this side never learned the statement of — one that was stopped — says so,
          // rather than leaving the last statement standing beside rows it did not produce.
          <span className="text-faint">No statement</span>
        ) : (
          // Two spans, so the ellipsis lands where it costs least — see PINNED_WORDS.
          <span className="flex min-w-0 overflow-hidden font-mono text-accent" title={statement}>
            <span className="min-w-0 truncate">{head}</span>
            {tail === '' ? null : <span className="ml-1 shrink-0">{tail}</span>}
          </span>
        )}
        <CopyStatement statement={statement} />
      </div>

      <span className="flex shrink-0 items-center border-l border-line px-2">
        Record {numbers.format(record)} of{' '}
        <span className="ml-1" title={partial ? MORE_ON_SERVER : undefined}>
          {numbers.format(total)}
          {partial ? '+' : ''}
        </span>
        <span className="ml-1">
          in page {numbers.format(page)} of {numbers.format(pageCount)}
        </span>
      </span>

      <span className="flex shrink-0 items-center border-l border-line px-2">
        Last Refresh: {ageText(age)}
      </span>
    </div>
  )
}

/** The statement as a head the bar may ellipsize and a tail it may not. */
function splitStatement(statement: string) {
  const words = statement.split(' ')
  if (words.length <= PINNED_WORDS) return { head: statement, tail: '' }

  const at = words.length - PINNED_WORDS
  return { head: words.slice(0, at).join(' '), tail: words.slice(at).join(' ') }
}

/** Copying is confirmed on the icon itself, there being no room here for a message. */
function CopyStatement({ statement }: { statement: string }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const clear = setTimeout(() => setCopied(false), COPIED_MS)
    return () => clearTimeout(clear)
  }, [copied])

  return (
    <IconButton
      label="Copy statement"
      icon={copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
      disabled={statement === ''}
      onClick={() => {
        void copyText(statement)
        setCopied(true)
      }}
    />
  )
}

/**
 * How long ago the rows landed, ticking. The clock is kept here rather than by the caller, so a
 * grid holding a million rows is not re-rendered once a second to move one word.
 */
function useAge(loadedAt: number | null) {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (loadedAt === null) return

    setNow(Date.now())
    const clock = setInterval(() => setNow(Date.now()), AGE_TICK_MS)
    return () => clearInterval(clock)
  }, [loadedAt])

  return loadedAt === null ? null : Math.max(0, now - loadedAt)
}

function ageText(age: number | null) {
  if (age === null) return '—'

  const seconds = Math.floor(age / 1000)
  if (seconds < 60) return `${seconds}s`

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`

  return `${Math.floor(minutes / 60)}h`
}
