import { useMemo, useState } from 'react'
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

import { KEY_TYPES } from './keySpace'
import {
  freshField,
  freshValue,
  hasMembers,
  readMembers,
  refusedReason,
  sampledTotal,
  shapeOf,
  writeMembers,
  type Member,
  type ValueShape,
} from './keyValue'

/**
 * What the two columns of a laid-out value are called. A hash holds a **field** and what it holds; a
 * sorted set holds an **element** and its **score**, which is a number and is drawn as one — the grid
 * aligns a column from its type, so the ranks read down the right edge as a column of numbers should.
 */
const MEMBER_COLUMNS: Record<'fields' | 'scores', ResultColumn[]> = {
  fields: [
    { name: 'Field', type: 'string', fullType: 'string', width: 200 },
    { name: 'Value', type: 'string', fullType: 'string' },
  ],
  scores: [
    { name: 'Element', type: 'string', fullType: 'string', width: 320 },
    { name: 'Score', type: 'double', fullType: 'double', width: 160 },
  ],
}

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
 * **A hash is laid out as its fields; everything else is read whole as text.** A hash is the one value
 * whose members have names the reader thinks in — `absexp`, `data` — so two columns say more than the
 * object does; a list, a set and a sorted set are read in `ValueViewer` like every other value in the
 * app, and so is a string. Either way it is the **same JSON**: the members are read out of what
 * `internal/driver/redis/value.go` wrote and written back into what `parseValue` reads, so this is a
 * layout and never a second notation. Text this side cannot read as members falls back to the viewer,
 * which is what keeps a box under the reader while they are retyping a value into another shape.
 *
 * Its layout is a **grid and not a scroller**: four rows, the value's taking the slack, so nothing can
 * ride over the clock beneath it however short the panel is dragged.
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
  const text = value ?? ''

  const [name, setName] = useState(keyName)
  const [kind, setKind] = useState(type === '' ? 'string' : type)
  const [draft, setDraft] = useState(text)
  const [picked, setPicked] = useState<Selection | null>(null)

  // The clock is held as what the reader is *saying* — how long, or when — rather than as the seconds
  // it becomes, because those two are the same number said two ways and only one of them is theirs.
  const [clockAs, setClockAs] = useState<ClockKind>(ttl === null || ttl === '' ? 'none' : 'in')
  const [seconds, setSeconds] = useState(ttl ?? '')
  const [when, setWhen] = useState('')

  const shape = shapeOf(kind)

  /**
   * The members of a value that has them, where the text can be read as them. Anything else — and a
   * hash whose text has been retyped into some other shape — is `null`, and the plain viewer takes
   * over: a reader changing the type of a key must always have a box to type the new shape into.
   */
  const members = useMemo(
    () => (hasMembers(shape) ? readMembers(draft) : null),
    [shape, draft],
  )

  /** How many the value really holds, where the reader is only being shown some of them. */
  const total = sampledTotal(draft)

  /** A member list *is* the value: every change to one goes straight back into the JSON. */
  const changeMembers = (held: readonly Member[]) => setDraft(writeMembers(held))

  /**
   * The clock as the column carries it: **seconds from now**, which is what `EXPIRE` takes and what
   * the driver reads. A moment the reader picked is converted here rather than sent as a date,
   * because a date is not what a key holds — a key holds a length of time, and the two would drift
   * apart the moment one was written and the other read.
   */
  const chosen = clockAs === 'at' ? secondsUntil(when) : null
  const nextTtl: Cell =
    clockAs === 'none'
      ? null
      : clockAs === 'in'
        ? (seconds.trim() === '' ? null : seconds)
        : chosen === null
          ? null
          : String(chosen)

  // A moment already past is not an expiry, and a zero would be read by the server as *now* — which
  // deletes the key. So it is refused here rather than sent.
  const clockRefused =
    clockAs === 'at' && chosen === null
      ? 'Pick a moment in the future: what a key holds is how long it has left, not a date.'
      : undefined

  const valueRefused = refusedReason(kind, text)
  const editable = valueRefused === undefined
  const refuses = valueRefused ?? clockRefused

  const valueChanged = draft !== text || kind !== type
  const dirty = name !== keyName || valueChanged || (nextTtl ?? '') !== (ttl ?? '')

  /**
   * Whether what the reader has changed can be written. A value that cannot be — a sample, a stream —
   * blocks **the value** and nothing else: a rename is `RENAME` and a clock is `EXPIRE`, and neither
   * so much as reads what the key holds. Blocking those too would mean a key whose value happened to
   * be large could never be renamed or given an expiry at all, which is the opposite of what the
   * refusal is protecting.
   */
  const blocked = (valueChanged && !editable) || clockRefused !== undefined

  const discard = () => {
    setName(keyName)
    setKind(type === '' ? 'string' : type)
    setDraft(text)
    setClockAs(ttl === null || ttl === '' ? 'none' : 'in')
    setSeconds(ttl ?? '')
    setWhen('')
    setPicked(null)
  }

  /** Changing how the clock is said keeps what it says: the same moment, spelled the other way. */
  const sayClockAs = (next: ClockKind) => {
    const left = Number.parseInt(seconds, 10)
    const held = Number.isNaN(left) || left <= 0 ? DEFAULT_SECONDS : left

    if (next === 'in') setSeconds(String(held))
    if (next === 'at') setWhen(localInput(new Date(Date.now() + held * 1000)))
    setClockAs(next)
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

        {/* A type is what the value *is*, so picking a different one is a promise about the box
            below: the key is emptied and refilled as that type, from the text standing there. Which
            is why the two are one write and never two — see `driver.CellEdit.ValueType`. A value the
            new type cannot read is refused before a command is sent, and says so. */}
        <Label>Key Type</Label>
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

        {/* A hash is laid out as its fields, which is how a reader thinks of one; every other value is
            read whole as text. Both are the same JSON underneath — the members are read out of what
            the driver wrote and written back into what it parses — so this is a *layout* and not a
            second notation. */}
        <Label align="start">Value</Label>
        <div className="flex min-h-0 flex-col self-stretch overflow-hidden rounded-md border border-line">
          {members === null ? (
            <ValueViewer
              className="min-h-0 flex-1"
              value={draft}
              columnType={type}
              onChange={editable && !busy ? setDraft : undefined}
              formatValue={formatValue}
            />
          ) : (
            <>
              <div className="min-h-0 flex-1">
                <ResultGrid
                  result={{
                    columns: MEMBER_COLUMNS[shape === 'scores' ? 'scores' : 'fields'],
                    rows: memberRows(members),
                  }}
                  dense
                  headStrip={false}
                  selection={picked}
                  onSelectionChange={setPicked}
                  onEditCell={
                    editable && !busy
                      ? (ref, held) =>
                          changeMembers(
                            members.map((member, at) =>
                              at !== ref.row
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

              <div className="flex h-6 shrink-0 items-center gap-1 border-t border-line bg-bar px-1">
                <IconButton
                  label={shape === 'scores' ? 'Add element' : 'Add field'}
                  icon={<Plus size={12} />}
                  disabled={!editable || busy}
                  onClick={() =>
                    changeMembers([
                      ...members,
                      { field: freshField(shape, members), value: freshValue(shape) },
                    ])
                  }
                />
                <IconButton
                  label={shape === 'scores' ? 'Remove element' : 'Remove field'}
                  icon={<Minus size={12} />}
                  disabled={!editable || busy || focusOf(picked) === null}
                  onClick={() => {
                    const gone = new Set(selectedRows(picked))
                    setPicked(null)
                    changeMembers(members.filter((_, at) => !gone.has(at)))
                  }}
                />

                <div className="min-w-2 flex-1" />

                {/* A sample says how much of it this is, because "50 elements" beside a value that
                    holds a hundred and seventeen is the one number a reader must not be given. */}
                <span className="font-mono text-2xs text-faint tabular-nums">
                  {total === null ? members.length : `${members.length} of ${total}`}{' '}
                  {memberWord(shape, total ?? members.length)}
                </span>
              </div>
            </>
          )}
        </div>

        {/* Three ways of saying one thing, because a reader means them differently: nothing at all,
            a length of time, or a moment. Only the first is a different fact — a key with no clock is
            not a key whose clock is long — and the other two are the same number said two ways, so
            switching between them keeps what was said. */}
        <Label>TTL</Label>
        <div className="flex items-center gap-2">
          <select
            aria-label="Expiry"
            value={clockAs}
            disabled={busy}
            onChange={(event) => sayClockAs(event.target.value as ClockKind)}
            className={cn(TOOLBAR_CONTROL, 'h-7 w-44 shrink-0')}
          >
            <option value="none">No expiry</option>
            <option value="in">Expire In (seconds)</option>
            <option value="at">Expire At (Local Time)</option>
          </select>

          {clockAs === 'in' ? (
            <input
              type="number"
              min={1}
              value={seconds}
              disabled={busy}
              onChange={(event) => setSeconds(event.target.value)}
              className={cn(TOOLBAR_CONTROL, 'data-font h-7 w-32 shrink-0')}
            />
          ) : null}

          {clockAs === 'at' ? (
            <>
              {/* The platform's own picker, which is how every other date in the app is picked —
                  see the grid's temporal cells. Seconds are offered because a key's clock has them. */}
              <input
                type="datetime-local"
                step={1}
                value={when}
                disabled={busy}
                onChange={(event) => setWhen(event.target.value)}
                className={cn(TOOLBAR_CONTROL, 'data-font h-7 w-56 shrink-0')}
              />
              {chosen === null ? null : (
                <span className="shrink-0 font-mono text-2xs text-faint tabular-nums">
                  {chosen.toLocaleString()}s from now
                </span>
              )}
            </>
          ) : null}
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
          onClick={() => onApply({ name, type: kind, value: draft, ttl: nextTtl })}
          disabled={!dirty || blocked || busy}
        >
          Apply
        </Button>
      </div>
    </div>
  )
}

function memberRows(members: readonly Member[]): Cell[][] {
  return members.map((member) => [member.field, member.value])
}

function memberWord(shape: ValueShape, count: number): string {
  const one = shape === 'scores' ? 'element' : 'field'
  return count === 1 ? one : `${one}s`
}

/** How a reader says a clock. Only `none` is a different fact; the other two are one number, twice. */
type ClockKind = 'none' | 'in' | 'at'

/** What a clock opens at where the reader has never given one — a minute, and plainly a placeholder. */
const DEFAULT_SECONDS = 60

/**
 * The seconds a chosen moment is from now, and `null` where it is not a moment in the future. It is
 * rounded rather than floored so that picking a minute from now is sixty seconds and not fifty-nine.
 */
function secondsUntil(local: string): number | null {
  if (local === '') return null

  const when = new Date(local).getTime()
  if (Number.isNaN(when)) return null

  const left = Math.round((when - Date.now()) / 1000)
  return left > 0 ? left : null
}

/** A moment as the platform's own picker spells one, which is **local** time and to the second. */
function localInput(when: Date): string {
  const pad = (part: number) => String(part).padStart(2, '0')

  const date = `${when.getFullYear()}-${pad(when.getMonth() + 1)}-${pad(when.getDate())}`
  const time = `${pad(when.getHours())}:${pad(when.getMinutes())}:${pad(when.getSeconds())}`
  return `${date}T${time}`
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
