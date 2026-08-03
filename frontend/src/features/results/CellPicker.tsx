import { useEffect, useRef, type KeyboardEvent, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { Clock3 } from 'lucide-react'

import { Button } from '@/shared/ui/Button'

import { overlayStyle, useCellAnchor } from './cellAnchor'
import type { TemporalPicker } from './columnType'

const PANEL_HEIGHT = 84
const PANEL_MIN_WIDTH = 208

/**
 * What each picker can read out of a cell, the rest of the text being none of its business — a
 * fraction of a second and a time zone are both past where a picker can go, so picking drops them.
 */
const READS: Record<TemporalPicker, RegExp> = {
  date: /^\d{4}-\d{2}-\d{2}/,
  time: /^\d{2}:\d{2}(:\d{2})?/,
  'datetime-local': /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}(:\d{2})?/,
}

/** What the picker takes out of the cell's own text; a value it cannot read opens it empty. */
export function toPicker(picker: TemporalPicker, text: string): string {
  return READS[picker].exec(text.trim())?.[0].replace(' ', 'T') ?? ''
}

/** What the cell holds after a pick: the same moment with the separator SQL writes it with. */
export function fromPicker(picked: string): string {
  return picked.replace('T', ' ')
}

/** This moment in the reader's own zone, to the granularity the column keeps. */
export function nowText(picker: TemporalPicker): string {
  const now = new Date()
  const pad = (part: number) => String(part).padStart(2, '0')
  const date = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
  const time = `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`

  if (picker === 'date') return date
  if (picker === 'time') return time
  return `${date} ${time}`
}

interface CellPickerProps {
  picker: TemporalPicker
  /** The cell's text, which is what the picker opens on. */
  value: string
  anchor: RefObject<HTMLElement | null>
  onChange: (value: string) => void
  onCommit: () => void
  onCancel: () => void
}

/**
 * The picker behind the `⋮` beside a temporal cell's box: the platform's own calendar and clock
 * over the value that cell holds, with **Now** for the moment it is opened. It writes back through
 * the box rather than around it, so what was picked is still text the reader can go on typing
 * into, and the footer still commits.
 */
export function CellPicker({
  picker,
  value,
  anchor,
  onChange,
  onCommit,
  onCancel,
}: CellPickerProps) {
  const box = useRef<HTMLInputElement>(null)
  const rect = useCellAnchor(anchor)

  // The click on `⋮` is the activation a browser wants before it opens the calendar itself, so
  // one click reaches it. Where a browser refuses, the control's own glyph is still there.
  useEffect(() => {
    try {
      box.current?.showPicker()
    } catch {
      box.current?.focus()
    }
  }, [])

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') onCommit()
    else if (event.key === 'Escape') onCancel()
    else return

    event.preventDefault()
    event.stopPropagation()
  }

  if (!rect) return null

  return createPortal(
    <div
      style={overlayStyle(rect, PANEL_HEIGHT, PANEL_MIN_WIDTH)}
      // The accent edge is the cell's own ring carried onto the panel: the two are one editor,
      // and a panel that only had a shadow was lost against a light surface under it.
      className="fixed z-50 flex flex-col gap-1.5 rounded-lg border border-accent bg-elevated p-2 shadow-[var(--shadow-overlay)]"
    >
      <input
        ref={box}
        type={picker}
        step={1}
        autoFocus
        value={toPicker(picker, value)}
        onChange={(event) => onChange(fromPicker(event.target.value))}
        // Leaving the panel is as good as pressing Done: the value is already in the box.
        onBlur={onCommit}
        onKeyDown={onKeyDown}
        className="h-7 w-full rounded-md border border-line bg-canvas px-2 font-mono text-sm text-fg outline-none focus:border-accent"
      />

      {/* A press on either button must not blur the field, or the commit that blur runs would
          beat the click. */}
      <div onMouseDown={(event) => event.preventDefault()} className="flex items-center gap-1">
        <Button
          variant="subtle"
          icon={<Clock3 size={13} />}
          onClick={() => onChange(nowText(picker))}
        >
          Now
        </Button>

        <div className="min-w-2 flex-1" />

        <Button variant="primary" onClick={onCommit}>
          Done
        </Button>
      </div>
    </div>,
    document.body,
  )
}
