import { useEffect, useMemo, useState, type KeyboardEvent } from 'react'
import { Search, X } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Field, TextInput } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'
import { ICON } from '@/shared/ui/menuIcons'

import { arityOf, symbolOf, type Arity, type Condition } from './filter'

const LIST_HEIGHT = 168

interface FilterValueDialogProps {
  condition: Condition
  /** The values the column holds. Absent where the surface cannot ask, and the boxes stand alone. */
  suggest?: ((column: string) => Promise<string[]>) | undefined
  onCommit: (values: string[]) => void
  onClose: () => void
}

/**
 * What a condition compares against, picked rather than typed: the values the column actually
 * holds, with a box beside them for one it does not — a column's values are what it is usually
 * asked about, not all a server will take.
 *
 * How many boxes there are is the operator's business: one for most, **From** and **To** for a
 * range, and a list operator has none, its values being whatever was ticked. An empty box means
 * *unset* rather than the empty string, which is what leaves `is empty` a question of its own.
 */
export function FilterValueDialog({
  condition,
  suggest,
  onCommit,
  onClose,
}: FilterValueDialogProps) {
  const arity = arityOf(condition.operator)
  const listed = arity === 'list'

  const [values, setValues] = useState<string[]>(() => boxes(condition, arity))
  /** Which box a picked value lands in — the last one the reader was typing into. */
  const [active, setActive] = useState(0)
  const [needle, setNeedle] = useState('')
  const [typed, setTyped] = useState('')
  const [options, setOptions] = useState<string[] | null>(null)

  useEffect(() => {
    if (!suggest) {
      setOptions([])
      return
    }

    let live = true
    // A column whose values cannot be read is a list nobody gets; the boxes still take anything.
    suggest(condition.column).then(
      (found) => live && setOptions(found),
      () => live && setOptions([]),
    )

    return () => {
      live = false
    }
  }, [suggest, condition.column])

  const matches = useMemo(() => {
    const search = needle.trim().toLowerCase()
    return (options ?? []).filter((one) => one.toLowerCase().includes(search))
  }, [options, needle])

  const picked = new Set(values)

  const pick = (option: string) => {
    if (!listed) {
      setValues(values.map((one, at) => (at === active ? option : one)))
      return
    }
    setValues(picked.has(option) ? values.filter((one) => one !== option) : [...values, option])
  }

  const add = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' || typed === '') return

    event.preventDefault()
    if (!picked.has(typed)) setValues([...values, typed])
    setTyped('')
  }

  // An empty box is a value the reader has not given, and a condition missing one narrows nothing
  // until it is filled in — see `isComplete`.
  const commit = () => onCommit(values.filter((one) => one !== ''))

  return (
    <Dialog
      open
      onOpenChange={(open) => !open && onClose()}
      title={condition.column}
      description={symbolOf(condition.operator)}
      icon={ICON.filter}
      width={380}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={commit}>
            OK
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        {listed ? (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-muted">Values</span>

            {values.length === 0 ? (
              <span className="text-xs text-faint">Tick a value below, or type one here.</span>
            ) : (
              <div className="flex flex-wrap gap-1">
                {values.map((value) => (
                  <span
                    key={value}
                    className="flex items-center gap-1 rounded border border-line-strong bg-panel py-0.5 pr-0.5 pl-1.5 font-mono text-2xs text-fg"
                  >
                    {value}
                    <IconButton
                      label={`Remove ${value}`}
                      icon={<X size={11} />}
                      onClick={() => setValues(values.filter((one) => one !== value))}
                    />
                  </span>
                ))}
              </div>
            )}

            <TextInput
              value={typed}
              placeholder="Type a value, then Enter"
              onChange={(event) => setTyped(event.target.value)}
              onKeyDown={add}
            />
          </div>
        ) : (
          values.map((value, at) => (
            <Field key={at} label={boxLabel(arity, at)}>
              {(controlId) => (
                <TextInput
                  id={controlId}
                  autoFocus={at === 0}
                  value={value}
                  onFocus={() => setActive(at)}
                  onChange={(event) =>
                    setValues(values.map((one, index) => (index === at ? event.target.value : one)))
                  }
                  onKeyDown={(event) => event.key === 'Enter' && commit()}
                />
              )}
            </Field>
          ))
        )}

        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-muted">Suggested Values</span>

          <div
            className="scroll-thin overflow-y-auto rounded-md border border-line bg-canvas"
            style={{ height: LIST_HEIGHT }}
          >
            {options === null ? (
              <p className="px-2 py-1.5 text-xs text-faint">Reading the column…</p>
            ) : matches.length === 0 ? (
              <p className="px-2 py-1.5 text-xs text-faint">
                {options.length === 0 ? 'No values to suggest.' : 'Nothing matches that search.'}
              </p>
            ) : (
              matches.map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => pick(option)}
                  className={cn(
                    'data-font flex w-full items-center gap-2 px-2 py-1 text-left hover:bg-hover',
                    picked.has(option) ? 'text-fg' : 'text-muted',
                  )}
                >
                  <span
                    className={cn(
                      'grid h-3.5 w-3.5 shrink-0 place-items-center rounded-sm border text-2xs',
                      picked.has(option)
                        ? 'border-accent bg-accent text-on-accent'
                        : 'border-line-strong',
                    )}
                  >
                    {picked.has(option) ? '✓' : ''}
                  </span>
                  <span className="min-w-0 truncate">{option}</span>
                </button>
              ))
            )}
          </div>

          <div className="relative">
            <Search size={13} className="absolute top-1/2 left-2 -translate-y-1/2 text-faint" />
            <TextInput
              value={needle}
              placeholder="Search"
              onChange={(event) => setNeedle(event.target.value)}
              // Room for the glyph, in a style rather than a class: `pl-*` beside the control's
              // own `px-*` would be settled by the stylesheet's order instead of by this file.
              style={{ paddingLeft: 26 }}
            />
          </div>
        </div>
      </div>
    </Dialog>
  )
}

/** As many boxes as the operator takes, holding whatever the condition already had. */
function boxes(condition: Condition, arity: Arity): string[] {
  if (arity === 'list') return condition.values
  return Array.from({ length: arity }, (_, at) => condition.values[at] ?? '')
}

const boxLabel = (arity: Arity, at: number) =>
  arity === 2 ? (at === 0 ? 'From' : 'To') : 'Value'
