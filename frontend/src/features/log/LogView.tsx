import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { Pause, Play, Search, TriangleAlert, WrapText } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ICON } from '@/shared/ui/menuIcons'
import { useLogStore } from '@/stores/log'

import { logLines, matches, type LineTone, type LogLine } from './logLines'

/** How often the viewer asks what has run since. Faster than an eye reads a new line. */
const POLL_MS = 1000

/** How near the foot the reader has to be for a new line to carry them down with it. */
const NEAR_BOTTOM = 24

/**
 * Everything this app has run since it started, oldest first — the statement as it was written,
 * what it ran against, and how long it took. It is the one place that says what the window has been
 * doing across *every* connection, which is what makes it the thing to open when something ran that
 * the reader did not ask for.
 *
 * It is a **log and not a grid**: a statement is lines rather than a cell, and the lines are
 * numbered down the side because what a reader does here is point at one. Its sibling is the
 * History pane in a query tab, which answers the other question — one entry per statement, kept on
 * disk, ready to be run again. See [docs/features/log.md](../../../../docs/features/log.md).
 */
export function LogView() {
  const entries = useLogStore((s) => s.entries)
  const paused = useLogStore((s) => s.paused)
  const wordWrap = useLogStore((s) => s.wordWrap)
  const errorsOnly = useLogStore((s) => s.errorsOnly)
  const find = useLogStore((s) => s.find)
  const poll = useLogStore((s) => s.poll)

  const shown = useMemo(
    () => entries.filter((entry) => (!errorsOnly || entry.error !== '') && matches(entry, find)),
    [entries, errorsOnly, find],
  )
  const lines = useMemo(() => logLines(shown), [shown])

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <LogToolbar filtered={shown.length === entries.length ? null : shown.length} />
      <LogLines
        lines={lines}
        wordWrap={wordWrap}
        paused={paused}
        poll={poll}
        empty={entries.length === 0}
      />
    </div>
  )
}

function LogToolbar({ filtered }: { filtered: number | null }) {
  const paused = useLogStore((s) => s.paused)
  const wordWrap = useLogStore((s) => s.wordWrap)
  const errorsOnly = useLogStore((s) => s.errorsOnly)
  const find = useLogStore((s) => s.find)
  const clear = useLogStore((s) => s.clear)
  const setPaused = useLogStore((s) => s.setPaused)
  const setWordWrap = useLogStore((s) => s.setWordWrap)
  const setErrorsOnly = useLogStore((s) => s.setErrorsOnly)
  const setFind = useLogStore((s) => s.setFind)

  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
    >
      {/* Clearing asks nothing first, unlike Clear History beside it: this log is memory, it holds
          nothing that was not already reported where it happened, and nothing on any server has
          ever been in it. */}
      <Button
        variant="subtle"
        icon={ICON.delete}
        onClick={() => void clear()}
        className="text-danger hover:bg-danger-subtle hover:text-danger"
      >
        Clear
      </Button>

      <Toggle on={wordWrap} icon={<WrapText size={13} />} onClick={() => setWordWrap(!wordWrap)}>
        Word Wrap
      </Toggle>

      <Toggle
        on={errorsOnly}
        icon={<TriangleAlert size={13} />}
        onClick={() => setErrorsOnly(!errorsOnly)}
      >
        Show Errors Only
      </Toggle>

      {/* Pause stops the *reading* and not the running: what runs while it is held goes on being
          kept, and letting go asks for all of it at once. */}
      <Toggle
        on={paused}
        icon={paused ? <Play size={13} /> : <Pause size={13} />}
        onClick={() => setPaused(!paused)}
      >
        {paused ? 'Resume' : 'Pause'}
      </Toggle>

      <div className="ml-auto flex items-center gap-2">
        {filtered === null ? null : <span className="text-2xs text-faint">{filtered} shown</span>}

        <div className="flex h-6 items-center gap-1.5 rounded-md border border-line-strong bg-elevated px-2">
          <Search size={12} className="shrink-0 text-faint" />
          <input
            value={find}
            onChange={(event) => setFind(event.target.value)}
            placeholder="Find"
            aria-label="Find in the log"
            className="w-40 bg-transparent text-xs text-fg outline-none placeholder:text-faint"
          />
        </div>
      </div>
    </div>
  )
}

/** A toolbar button that is either held down or not, and says which by wearing the hover shade. */
function Toggle({
  on,
  icon,
  onClick,
  children,
}: {
  on: boolean
  icon: ReactNode
  onClick: () => void
  children: ReactNode
}) {
  return (
    <Button
      variant="subtle"
      icon={icon}
      aria-pressed={on}
      onClick={onClick}
      className={cn(on && 'bg-hover text-fg')}
    >
      {children}
    </Button>
  )
}

const TONE: Record<LineTone, string> = {
  head: 'text-accent',
  sql: 'text-fg',
  note: 'text-muted',
  error: 'text-danger',
}

interface LogLinesProps {
  lines: LogLine[]
  wordWrap: boolean
  paused: boolean
  poll: () => Promise<void>
  /** Whether the log itself is empty, as against emptied by the toolbar's own filters. */
  empty: boolean
}

function LogLines({ lines, wordWrap, paused, poll, empty }: LogLinesProps) {
  const box = useRef<HTMLDivElement>(null)
  // Whether the reader is at the foot, which is what decides if a new line carries them with it.
  // A reader who has scrolled up is reading something, and a log that yanked them back down every
  // second would be unreadable.
  const following = useRef(true)

  useEffect(() => {
    if (paused) return

    void poll()
    const timer = setInterval(() => void poll(), POLL_MS)
    return () => clearInterval(timer)
  }, [paused, poll])

  useEffect(() => {
    if (!following.current) return
    const element = box.current
    if (element) element.scrollTop = element.scrollHeight
  }, [lines.length])

  const onScroll = () => {
    const element = box.current
    if (!element) return
    following.current =
      element.scrollHeight - element.scrollTop - element.clientHeight <= NEAR_BOTTOM
  }

  if (lines.length === 0) {
    return (
      <div className="min-h-0 flex-1">
        <Empty
          title={empty ? 'Nothing has run yet' : 'No statement matches'}
          hint={
            empty
              ? 'Every statement you execute in a query tab or a console is kept here, oldest first.'
              : 'Clear the box, or turn Show Errors Only off.'
          }
        />
      </div>
    )
  }

  return (
    <div
      ref={box}
      onScroll={onScroll}
      className="scroll-thin selectable min-h-0 flex-1 overflow-auto py-1 font-mono text-xs leading-5"
    >
      <div className={cn(wordWrap ? 'w-full' : 'w-max min-w-full')}>
        {lines.map((line, at) => (
          <div key={at} className="flex">
            {/* The number rides the horizontal scroll rather than leaving with it: unwrapped, a
                log is far wider than the window, and a line the reader has scrolled across to
                read is one they still have to be able to name. */}
            <span className="sticky left-0 w-14 shrink-0 bg-canvas pr-3 text-right tabular-nums text-faint select-none">
              {at + 1}
            </span>
            <span
              className={cn(
                'min-w-0',
                wordWrap ? 'whitespace-pre-wrap' : 'whitespace-pre',
                TONE[line.tone],
              )}
            >
              {line.text}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
