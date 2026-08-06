import { useMemo, useState, type ReactNode } from 'react'
import { Minus, Plus } from 'lucide-react'

import type { Cell } from '@/bridge/tables'
import { ResultGrid } from '@/features/results/ResultGrid'
import {
  focusOf,
  selectedRows,
  type ResultColumn,
  type Selection,
} from '@/features/results/resultSet'
import { ValueViewer } from '@/features/results/ValueViewer'
import { Button } from '@/shared/ui/Button'
import { IconButton } from '@/shared/ui/IconButton'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { cn } from '@/lib/cn'

import {
  freshField,
  isSample,
  readMembers,
  renumbered,
  shapeOf,
  writeMembers,
  type Member,
  type ValueShape,
} from './keyValue'

/**
 * What the panel opens at, and how far it may be dragged. It is taller than the cell editor's own
 * bounds because it holds four things and not one — a name, a type, a value and a clock — and the
 * floor is what leaves one row of the value readable under them.
 */
export const KEY_EDITOR_HEIGHT = 280
export const KEY_EDITOR_MIN_HEIGHT = 176

/** The width the four labels share, so their controls line up down the panel. */
const LABEL_WIDTH = '5.5rem'

/** What the reader hands back: the three columns of a key that can be written. */
export interface KeyEdit {
  name: string
  value: Cell
  ttl: Cell
}

interface KeyEditorProps {
  /** The key as it stands on the server, which is what Discard goes back to. */
  keyName: string
  type: string
  value: Cell
  ttl: Cell
  busy: boolean
  /**
   * Stages the three on the row, exactly as the cell editor stages one cell: the footer's Apply is
   * still what writes, so a rename, a new value and a new clock are one write and one undo.
   */
  onApply: (edit: KeyEdit) => void
  /** The engine's own layout, for the value viewer a string is read in. */
  formatValue?: ((value: string) => Promise<string>) | undefined
}

/**
 * The panel under a key space, which is **always open**: a Redis row is a key and its value, and the
 * value is the whole reason the reader opened the tab. On a table store a cell editor is one of
 * several ways to reach one cell of forty, so it is a panel the reader asks for; here there are five
 * columns, three of them one line long, and the fourth is a container — so there is nothing for a
 * toggle to save and a grid row is the wrong shape to edit one in.
 *
 * A container is edited **member by member**, which is what a hash or a sorted set actually is. The
 * notation underneath is unchanged: the members are read out of the JSON the driver wrote and written
 * back into the JSON it parses, so nothing here is a second spelling — see `keyValue.ts`.
 */
export function KeyEditor({
  keyName,
  type,
  value,
  ttl,
  busy,
  onApply,
  formatValue,
}: KeyEditorProps) {
  const shape = shapeOf(type)
  const text = value ?? ''

  const [name, setName] = useState(keyName)
  const [draft, setDraft] = useState(text)
  const [clock, setClock] = useState(ttl ?? '')
  const [selection, setSelection] = useState<Selection | null>(null)

  /**
   * Why the value cannot be written, where it cannot. Both reasons are the driver's own and are said
   * *before* the reader types rather than after they press Apply: a sample would write away whatever
   * the grid did not show, and a stream is the server's own ids and is append-only.
   */
  const refuses = isSample(text)
    ? 'This value is a sample of a larger one. Writing it back would drop whatever the grid did not show.'
    : shape === 'opaque'
      ? `A ${type} value is read here and not written: its shape is the server's own.`
      : undefined

  const editable = refuses === undefined
  const members = useMemo(() => readMembers(shape, draft), [shape, draft])

  const next: KeyEdit = { name, value: draft, ttl: clock === '' ? null : clock }
  const dirty = name !== keyName || draft !== text || (ttl ?? '') !== clock

  const discard = () => {
    setName(keyName)
    setDraft(text)
    setClock(ttl ?? '')
    setSelection(null)
  }

  /** A member list is the value: every change to one is written straight back into the JSON. */
  const changeMembers = (held: readonly Member[]) =>
    setDraft(writeMembers(shape, renumbered(shape, held)))

  const addMember = () => {
    if (members === null) return
    changeMembers([...members, { field: freshField(shape, members), value: '' }])
  }

  const removeMembers = () => {
    if (members === null) return
    const gone = new Set(selectedRows(selection))
    setSelection(null)
    changeMembers(members.filter((_, index) => !gone.has(index)))
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="scroll-thin flex min-h-0 flex-1 flex-col gap-2 overflow-auto p-2">
        <Row label="Key Name">
          <input
            spellCheck={false}
            value={name}
            disabled={busy}
            onChange={(event) => setName(event.target.value)}
            className={cn(TOOLBAR_CONTROL, 'data-font h-7 w-full')}
          />
        </Row>

        {/* A type is what the value *is*, so it is shown and never picked: changing it would be
            changing the value, which is the field below. The driver refuses the edit for the same
            reason, and a dead dropdown here would only invite it. */}
        <Row label="Key Type">
          <span className="data-font text-xs text-muted">{type}</span>
        </Row>

        <Row label="Value" align="start">
          <div className="flex min-h-24 flex-1 flex-col overflow-hidden rounded-md border border-line">
            {members === null ? (
              <ValueViewer
                className="min-h-0 flex-1"
                value={draft}
                columnType={type}
                onChange={editable ? setDraft : undefined}
                formatValue={formatValue}
              />
            ) : (
              <>
                <div className="min-h-0 flex-1">
                  <ResultGrid
                    result={{ columns: memberColumns(shape), rows: memberRows(members) }}
                    dense
                    headStrip={false}
                    selection={selection}
                    onSelectionChange={setSelection}
                    onEditCell={
                      editable
                        ? (ref, held) =>
                            changeMembers(
                              members.map((member, index) =>
                                index !== ref.row
                                  ? member
                                  : ref.column === 0
                                    ? { ...member, field: held ?? '' }
                                    : { ...member, value: held ?? '' },
                              ),
                            )
                        : undefined
                    }
                  />
                </div>

                <div className="flex h-7 shrink-0 items-center gap-1 border-t border-line bg-bar px-1.5">
                  <IconButton
                    label="Add member"
                    icon={<Plus size={13} />}
                    disabled={!editable || busy}
                    onClick={addMember}
                  />
                  <IconButton
                    label="Remove member"
                    icon={<Minus size={13} />}
                    disabled={!editable || busy || focusOf(selection) === null}
                    onClick={removeMembers}
                  />

                  <div className="min-w-2 flex-1" />

                  <span className="font-mono text-2xs text-faint tabular-nums">
                    {members.length} {members.length === 1 ? 'member' : 'members'}
                  </span>
                </div>
              </>
            )}
          </div>
        </Row>

        {/* Two controls and not one, because "no expiry" is not a number of seconds: clearing the
            box is what takes a clock off, which is exactly what the driver reads an empty one as. */}
        <Row label="TTL">
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
        </Row>
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
          onClick={() => onApply(next)}
          disabled={!dirty || !editable || busy}
        >
          Apply
        </Button>
      </div>
    </div>
  )
}

function Row({
  label,
  align = 'center',
  children,
}: {
  label: string
  align?: 'center' | 'start'
  children: ReactNode
}) {
  return (
    <div
      className={cn('flex gap-2', align === 'start' ? 'min-h-0 flex-1 items-start' : 'items-center')}
    >
      <span
        className={cn('shrink-0 text-xs text-muted', align === 'start' && 'pt-1')}
        style={{ width: LABEL_WIDTH }}
      >
        {label}
      </span>
      {children}
    </div>
  )
}

/**
 * The two columns a member is. An array's field is its **index** and is not the reader's to type: a
 * list is ordered by where its elements sit, so renumbering it by hand would be renumbering nothing.
 */
function memberColumns(shape: ValueShape): ResultColumn[] {
  return [
    {
      name: shape === 'array' ? 'Index' : 'Field',
      type: 'string',
      fullType: 'string',
      width: 160,
      readOnly: shape === 'array',
    },
    { name: 'Value', type: 'string', fullType: 'string' },
  ]
}

function memberRows(members: readonly Member[]): Cell[][] {
  return members.map((member) => [member.field, member.value])
}
