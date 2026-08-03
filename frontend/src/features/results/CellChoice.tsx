import { useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'

import { cn } from '@/lib/cn'

import type { Cell } from './resultSet'

const LIST_HEIGHT = 220
const LIST_MIN_WIDTH = 180

interface CellChoiceProps {
  value: Cell
  options: readonly string[]
  width: number
  onCommit: (value: Cell) => void
  onCancel: () => void
}

/**
 * The editor of a `choice` column: the cell's own box, still typable, over the list of values the
 * column accepts. Typing filters that list rather than replacing it, and `Enter` on a cell nothing
 * matched commits what was typed — a column's options are what it is usually given, not all a
 * server will take.
 *
 * The list is portalled and fixed, because the grid scrolls and would otherwise clip it; for the
 * same reason a scroll dismisses it, the box it was measured against having moved.
 */
export function CellChoice({ value, options, width, onCommit, onCancel }: CellChoiceProps) {
  const box = useRef<HTMLInputElement>(null)
  const [draft, setDraft] = useState(value ?? '')
  const [typed, setTyped] = useState(false)
  const [active, setActive] = useState(-1)
  const [anchor, setAnchor] = useState<DOMRect | null>(null)

  // The list is fixed, so it has to be told where its cell is — and told again whenever the grid
  // scrolls under it, which is what keeps the two together instead of dismissing one of them.
  useLayoutEffect(() => {
    const place = () => setAnchor(box.current?.getBoundingClientRect() ?? null)

    place()
    window.addEventListener('scroll', place, true)
    window.addEventListener('resize', place)
    return () => {
      window.removeEventListener('scroll', place, true)
      window.removeEventListener('resize', place)
    }
  }, [])

  const needle = draft.trim().toLowerCase()
  const matches = typed ? options.filter((one) => one.toLowerCase().includes(needle)) : options
  const highlighted = matches[active]

  const commit = (text: string) => onCommit(text === '' && value === null ? null : text)

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') setActive(Math.min(active + 1, matches.length - 1))
    else if (event.key === 'ArrowUp') setActive(Math.max(active - 1, -1))
    else if (event.key === 'Enter') commit(highlighted ?? draft)
    else if (event.key === 'Escape') onCancel()
    else return

    event.preventDefault()
    event.stopPropagation()
  }

  return (
    <>
      <input
        ref={box}
        autoFocus
        value={draft}
        spellCheck={false}
        onFocus={(event) => event.target.select()}
        onChange={(event) => {
          setDraft(event.target.value)
          setTyped(true)
          setActive(-1)
        }}
        onBlur={() => commit(draft)}
        onKeyDown={onKeyDown}
        style={{ width }}
        className="shrink-0 border-r border-line/60 bg-canvas px-2 font-mono text-sm text-fg shadow-[inset_0_0_0_1px_var(--accent)] outline-none"
      />

      {anchor && matches.length > 0
        ? createPortal(
            <div
              // The list belongs to the box below it: a press inside must not blur the box, or
              // the commit that blur runs would beat the option that was clicked.
              onMouseDown={(event) => event.preventDefault()}
              style={{
                left: anchor.left,
                minWidth: Math.max(anchor.width, LIST_MIN_WIDTH),
                maxHeight: LIST_HEIGHT,
                ...(anchor.bottom + LIST_HEIGHT > window.innerHeight && anchor.top > LIST_HEIGHT
                  ? { bottom: window.innerHeight - anchor.top }
                  : { top: anchor.bottom }),
              }}
              className="scroll-thin fixed z-50 overflow-auto overscroll-contain rounded-md border border-line-strong bg-elevated py-1 shadow-[var(--shadow-overlay)]"
            >
              {matches.map((option, at) => (
                <div
                  key={option}
                  onMouseEnter={() => setActive(at)}
                  onClick={() => commit(option)}
                  className={cn(
                    'cursor-default px-2 py-1 font-mono text-sm',
                    at === active ? 'bg-hover text-fg' : 'text-muted',
                    option === value && 'text-accent',
                  )}
                >
                  {option}
                </div>
              ))}
            </div>,
            document.body,
          )
        : null}
    </>
  )
}
