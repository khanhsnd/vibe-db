import { useState } from 'react'

import type { Cell } from '@/bridge/tables'
import { ValueViewer } from '@/features/results/ValueViewer'
import { Button } from '@/shared/ui/Button'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { cn } from '@/lib/cn'

import { KEY_TYPES } from './keySpace'
import { refusedReason } from './keyValue'

/**
 * What the panel opens at, and how far it may be dragged. It is taller than the cell editor's own
 * bounds because it holds four things and not one — a name, a type, a value and a clock — and the
 * floor is what leaves a line or two of the value readable under them.
 */
export const KEY_EDITOR_HEIGHT = 300
export const KEY_EDITOR_MIN_HEIGHT = 180

/** The width the four labels share, so their controls line up down the panel. */
const LABEL_WIDTH = '5.5rem'

/** What the reader hands back: the columns of a key that can be written. */
export interface KeyEdit {
  name: string
  type: string
  value: Cell
  ttl: Cell
}

interface KeyEditorProps {
  /** The key as it stands on the server, which is what Discard goes back to. */
  keyName: string
  type: string
  value: Cell
  ttl: Cell
  /**
   * Whether this key is not on the server yet — a row the reader has just added. It is the one state
   * in which the **type** is theirs to pick: a type is what a value *is*, so changing one on a key
   * that exists is changing the value, and the driver refuses it. On a new key nothing exists to
   * contradict, and the type is what decides which command writes it.
   */
  isNew: boolean
  busy: boolean
  /**
   * Stages the columns on the row, exactly as the cell editor stages one cell: the footer's Apply is
   * still what writes, so a rename, a new value and a new clock are one write and one undo.
   */
  onApply: (edit: KeyEdit) => void
  /** The engine's own layout, for the value viewer to lay a document out with. */
  formatValue?: ((value: string) => Promise<string>) | undefined
}

/**
 * The panel under a key space, which is **always open**: a Redis row is a key and its value, and the
 * value is the whole reason the reader opened the tab. On a table store a cell editor is one of
 * several ways to reach one cell of forty, so it is a panel the reader asks for; here there are five
 * columns, three of them one line long, and the fourth is a container — so there is nothing for a
 * toggle to save and a grid row is the wrong shape to read one in.
 *
 * **The value is text and not a table of fields.** The driver already spells a container as JSON
 * precisely so that what is drawn is what an edit hands back, and a field table over it would be a
 * second notation for the one thing the app has exactly one notation for — plus a second grid inside a
 * panel, with a header, a gutter and an editor opening under every click, for a value that is usually
 * eleven short lines. `ValueViewer` is the one surface a value is read whole in, here as everywhere.
 *
 * Its layout is a **grid and not a scroller**: four rows, the value's taking the slack, so nothing can
 * ride over the clock beneath it however short the panel is dragged.
 */
export function KeyEditor({
  keyName,
  type,
  value,
  ttl,
  isNew,
  busy,
  onApply,
  formatValue,
}: KeyEditorProps) {
  const text = value ?? ''

  const [name, setName] = useState(keyName)
  const [kind, setKind] = useState(type === '' ? 'string' : type)
  const [draft, setDraft] = useState(text)
  const [clock, setClock] = useState(ttl ?? '')

  const refuses = refusedReason(kind, text)
  const editable = refuses === undefined

  const dirty = name !== keyName || kind !== type || draft !== text || (ttl ?? '') !== clock

  const discard = () => {
    setName(keyName)
    setKind(type === '' ? 'string' : type)
    setDraft(text)
    setClock(ttl ?? '')
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        className="grid min-h-0 flex-1 items-center gap-x-2 gap-y-1.5 p-2"
        style={{
          gridTemplateColumns: `${LABEL_WIDTH} minmax(0, 1fr)`,
          gridTemplateRows: 'auto auto minmax(0, 1fr) auto',
        }}
      >
        <Label>Key Name</Label>
        <input
          spellCheck={false}
          value={name}
          disabled={busy}
          onChange={(event) => setName(event.target.value)}
          className={cn(TOOLBAR_CONTROL, 'data-font h-7 w-full')}
        />

        <Label>Key Type</Label>
        {isNew ? (
          <select
            aria-label="Key type"
            value={kind}
            disabled={busy}
            onChange={(event) => setKind(event.target.value)}
            className={cn(TOOLBAR_CONTROL, 'h-7 w-40 justify-self-start')}
          >
            {KEY_TYPES.map((one) => (
              <option key={one} value={one}>
                {one}
              </option>
            ))}
          </select>
        ) : (
          // A type is what the value *is*, so on a key that exists it is shown and never picked:
          // changing it would be changing the value, which is the field below.
          <span className="data-font text-xs text-muted">{type}</span>
        )}

        <Label align="start">Value</Label>
        <div className="flex min-h-0 self-stretch overflow-hidden rounded-md border border-line">
          <ValueViewer
            className="min-h-0 flex-1"
            value={draft}
            columnType={type}
            onChange={editable && !busy ? setDraft : undefined}
            formatValue={formatValue}
          />
        </div>

        <Label>TTL</Label>
        {/* Two controls and not one, because "no expiry" is not a number of seconds: clearing the
            box is what takes a clock off, which is exactly what the driver reads an empty one as. */}
        <div className="flex items-center gap-2">
          <select
            aria-label="Expiry"
            value={clock === '' ? 'none' : 'seconds'}
            disabled={busy}
            onChange={(event) => setClock(event.target.value === 'none' ? '' : '60')}
            className={cn(TOOLBAR_CONTROL, 'h-7 w-40 shrink-0')}
          >
            <option value="none">No expiry</option>
            <option value="seconds">Expire In (seconds)</option>
          </select>

          {clock === '' ? null : (
            <input
              type="number"
              min={1}
              value={clock}
              disabled={busy}
              onChange={(event) => setClock(event.target.value)}
              className={cn(TOOLBAR_CONTROL, 'data-font h-7 w-32 shrink-0')}
            />
          )}
        </div>
      </div>

      {refuses === undefined ? null : (
        <p className="shrink-0 border-t border-line px-3 py-1 text-xs text-warn">{refuses}</p>
      )}

      <div className="flex h-8 shrink-0 items-center justify-end gap-1 border-t border-line bg-bar px-1.5">
        <Button onClick={discard} disabled={!dirty || busy}>
          Discard
        </Button>
        <Button
          variant="primary"
          onClick={() => onApply({ name, type: kind, value: draft, ttl: clock === '' ? null : clock })}
          disabled={!dirty || !editable || busy}
        >
          Apply
        </Button>
      </div>
    </div>
  )
}

function Label({
  align = 'center',
  children,
}: {
  align?: 'center' | 'start'
  children: string
}) {
  return (
    <span className={cn('text-xs text-muted', align === 'start' && 'self-start pt-1.5')}>
      {children}
    </span>
  )
}
