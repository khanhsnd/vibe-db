import { useId } from 'react'

import { cn } from '@/lib/cn'

export interface RadioOption<T extends string> {
  value: T
  label: string
  /** A second line under the label, for a choice whose consequence is worth spelling out. */
  hint?: string | undefined
  disabled?: boolean | undefined
}

/**
 * A column of choices, one of which is taken. It is a radio list rather than a `<select>` because
 * both places it is used are choices the reader makes once and has to be able to *compare* — the
 * format a file is, and what an import does to the rows already there. A dropdown hides five of the
 * six answers behind the one that happens to be selected, which for the import modes is exactly the
 * screen where that must not happen.
 */
export function RadioList<T extends string>({
  name,
  value,
  options,
  onChange,
}: {
  name: string
  value: T
  options: ReadonlyArray<RadioOption<T>>
  onChange: (value: T) => void
}) {
  const group = useId()

  return (
    <div role="radiogroup" aria-label={name} className="flex flex-col gap-0.5">
      {options.map((option) => (
        <label
          key={option.value}
          className={cn(
            'flex items-start gap-2 rounded-md px-2 py-1.5',
            option.disabled ? 'opacity-45' : 'cursor-pointer hover:bg-hover',
          )}
        >
          <input
            type="radio"
            name={group}
            className="mt-0.5 h-3.5 w-3.5 accent-[var(--accent)]"
            checked={value === option.value}
            disabled={option.disabled}
            onChange={() => onChange(option.value)}
          />
          <span className="min-w-0">
            <span className="block text-sm text-fg">{option.label}</span>
            {option.hint ? (
              <span className="block text-xs text-faint">{option.hint}</span>
            ) : null}
          </span>
        </label>
      ))}
    </div>
  )
}
