import { useEffect, useState, type KeyboardEvent } from 'react'
import { X } from 'lucide-react'

import { IconButton } from '@/shared/ui/IconButton'

import {
  arityOf,
  promotionOf,
  valueLabel,
  type Arity,
  type Condition,
  type FilterOp,
} from './filter'
import { BOX, PickerPanel, SearchList } from './FilterPicker'

interface ValuePickProps {
  condition: Condition
  /** The values the column holds. Absent where the surface cannot ask, and the boxes stand alone. */
  suggest?: ((column: string) => Promise<string[]>) | undefined
  /**
   * The values, **and the operator they ask for**: ticking a second value is what turns `= a` into
   * `is in list (a, b)`, so the two travel together or the condition would read as the one thing and
   * run as the other.
   */
  onChange: (values: string[], operator: FilterOp) => void
}

/**
 * What a condition compares against, picked rather than typed: the values the column actually
 * holds, ticked off a searchable list, with a box beside them for one it does not hold — a column's
 * values are what it is usually asked about, not all a server will take.
 *
 * What the panel looks like follows the values rather than the other way about: one value is a box,
 * more than one is a row of chips, and a range is **From** and **To**. An empty box means *unset*
 * rather than the empty string, which is what leaves `is empty` a question of its own — and why the
 * empty string is left out of the list.
 */
export function ValuePick({ condition, suggest, onChange }: ValuePickProps) {
  const [open, setOpen] = useState(false)

  return (
    <PickerPanel
      label="Value"
      text={valueLabel(condition)}
      tone="accent"
      width={248}
      open={open}
      onOpenChange={setOpen}
    >
      {/* Mounted with the panel, so each open reads the column again and a value added since the
          last one is there. */}
      <ValuePanel
        condition={condition}
        suggest={suggest}
        onChange={onChange}
        onDone={() => setOpen(false)}
      />
    </PickerPanel>
  )
}

interface ValuePanelProps extends ValuePickProps {
  onDone: () => void
}

function ValuePanel({ condition, suggest, onChange, onDone }: ValuePanelProps) {
  const arity = arityOf(condition.operator)
  /** A range is two boxes rather than a list of values, its two being a floor and a ceiling. */
  const ranged = arity === 2
  const promotion = promotionOf(condition.operator)
  /** Whether a tick **adds** to what is there, which is what draws a check box on every row. */
  const multiple = arity === 'list' || promotion !== null

  /** The boxes as the reader left them, empties and all — an empty one is stripped on the way out. */
  const [boxes, setBoxes] = useState(() => startBoxes(condition, arity))
  /** Which box a picked value lands in: the last one the reader was typing into. */
  const [active, setActive] = useState(0)
  /**
   * The operator the values grew out of, so unticking back to one gives it back — for as long as
   * the panel is open, which is as long as a tick that was a mistake is worth undoing. A reopened
   * panel finds `is in list` and takes it as what the reader meant, the operator menu being the way
   * back by then.
   */
  const [demotion, setDemotion] = useState<FilterOp | null>(null)
  const [typed, setTyped] = useState('')
  const [options, setOptions] = useState<string[] | null>(null)

  useEffect(() => {
    if (!suggest) {
      setOptions([])
      return
    }

    let live = true
    // A column whose values cannot be read is a list nobody gets; the boxes still take anything.
    // The empty string is dropped: an empty box already means unset, so it could not be picked.
    suggest(condition.column).then(
      (found) => live && setOptions(found.filter((one) => one !== '')),
      () => live && setOptions([]),
    )

    return () => {
      live = false
    }
  }, [suggest, condition.column])

  const ticked = boxes.filter((one) => one !== '')
  /** Chips rather than a box: more than one value cannot be shown in one. */
  const listed = !ranged && (arity === 'list' || ticked.length > 1)

  const commit = (next: string[]) => {
    setBoxes(next)
    const values = next.filter((one) => one !== '')

    // A second value means `is in list`, and dropping back to one gives back the operator it grew
    // out of — see `demotion`. Nothing here is silent: the operator part beside this panel says so
    // the moment it changes, so the condition on screen is always the condition that will run.
    if (values.length > 1 && promotion !== null) {
      if (demotion === null) setDemotion(condition.operator)
      onChange(values, promotion)
      return
    }
    if (values.length <= 1 && demotion !== null) {
      setDemotion(null)
      onChange(values, demotion)
      return
    }
    onChange(values, condition.operator)
  }

  const pick = (option: string) => {
    if (ranged) {
      commit(boxes.map((one, at) => (at === active ? option : one)))
      return
    }
    if (!multiple) {
      // A bound is one value by nature — `> a` and `> b` are one comparison, not both — so the tick
      // replaces, and the panel has nothing left to ask.
      commit([option])
      onDone()
      return
    }

    commit(ticked.includes(option) ? ticked.filter((one) => one !== option) : [...ticked, option])
  }

  const add = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' || typed === '') return

    event.preventDefault()
    if (!ticked.includes(typed)) commit([...ticked, typed])
    setTyped('')
  }

  return (
    <div className="flex flex-col gap-1.5">
      {ranged ? (
        boxes.map((value, at) => (
          <label key={at} className="flex items-center gap-1.5">
            <span className="w-8 shrink-0 text-2xs text-muted">{at === 0 ? 'From' : 'To'}</span>
            <input
              autoFocus={at === 0}
              value={value}
              spellCheck={false}
              onFocus={() => setActive(at)}
              onChange={(event) =>
                commit(boxes.map((one, index) => (index === at ? event.target.value : one)))
              }
              onKeyDown={(event) => event.key === 'Enter' && onDone()}
              className={BOX}
            />
          </label>
        ))
      ) : listed ? (
        <>
          {ticked.length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {ticked.map((value) => (
                <span
                  key={value}
                  className="flex items-center gap-1 rounded border border-line-strong bg-panel py-0.5 pr-0.5 pl-1.5 font-mono text-2xs text-fg"
                >
                  <span className="max-w-32 truncate">{value}</span>
                  <IconButton
                    label={`Remove ${value}`}
                    icon={<X size={11} />}
                    onClick={() => commit(ticked.filter((one) => one !== value))}
                  />
                </span>
              ))}
            </div>
          ) : null}

          <input
            value={typed}
            placeholder="Type a value, then Enter"
            spellCheck={false}
            onChange={(event) => setTyped(event.target.value)}
            onKeyDown={add}
            className={`${BOX} w-full placeholder:text-faint`}
          />
        </>
      ) : (
        <label className="flex items-center gap-1.5">
          <span className="w-8 shrink-0 text-2xs text-muted">Value</span>
          <input
            autoFocus
            value={boxes[0] ?? ''}
            spellCheck={false}
            onChange={(event) => commit([event.target.value])}
            onKeyDown={(event) => event.key === 'Enter' && onDone()}
            className={BOX}
          />
        </label>
      )}

      <SearchList
        options={options ?? []}
        picked={new Set(ticked)}
        loading={options === null}
        multiple={multiple && !ranged}
        emptyLabel="No values to suggest."
        onPick={pick}
      />
    </div>
  )
}

/** As many boxes as the operator takes, holding whatever the condition already had. */
function startBoxes(condition: Condition, arity: Arity): string[] {
  if (arity === 'list') return condition.values
  return Array.from({ length: arity }, (_, at) => condition.values[at] ?? '')
}
