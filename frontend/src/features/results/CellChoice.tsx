import { useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'

import { cn } from '@/lib/cn'

import { overlayStyle, useCellAnchor } from './cellAnchor'
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
 * The list is portalled and fixed, because the grid scrolls and would otherwise clip it, so it
 * follows its own cell through `useCellAnchor`.
 */
export function CellChoice({ value, options, width, onCommit, onCancel }: CellChoiceProps) {
  const box = useRef<HTMLInputElement>(null)
  const [draft, setDraft] = useState(value ?? '')
  const [typed, setTyped] = useState(false)
  const [active, setActive] = useState(-1)
  const anchor = useCellAnchor(box)

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
              style={overlayStyle(anchor, LIST_HEIGHT, LIST_MIN_WIDTH)}
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
