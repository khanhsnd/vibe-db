import type { LucideIcon } from 'lucide-react'

import { cn } from '@/lib/cn'

/**
 * One control, two or three mutually exclusive answers — and the answer is **readable without
 * opening anything**, which is the whole reason it is a row of buttons rather than a `select`.
 *
 * It is the app's one way of saying *which of these am I looking at*, as against a
 * [`Button`](Button.tsx) held down with `aria-pressed`, which says whether one thing is **on**. The
 * difference matters on a toolbar: a pressed button reads as a shade beside its neighbours, and three
 * of them side by side read as three buttons with one shaded — which is not the same sentence as "this
 * pane is the one you are on". The chosen one here wears the accent outright.
 *
 * A comparison's two halves and a monitor's three panes are both this control.
 */
export function Segmented<T extends string>({
  label,
  options,
  value,
  onChange,
  iconOnly = false,
  prefix,
}: {
  label: string
  options: ReadonlyArray<{ id: T; label: string; icon?: LucideIcon }>
  value: T
  onChange: (id: T) => void
  iconOnly?: boolean
  /** A word before the buttons, where the control needs one to be read as a sentence. */
  prefix?: string
}) {
  return (
    <div className="flex shrink-0 items-center gap-1.5">
      {prefix ? <span className="text-xs text-muted">{prefix}</span> : null}

      <div role="group" aria-label={label} className="flex shrink-0 rounded-md border border-line">
        {options.map((option, at) => {
          const Icon = option.icon
          const chosen = option.id === value

          return (
            <button
              key={option.id}
              type="button"
              aria-pressed={chosen}
              aria-label={iconOnly ? option.label : undefined}
              title={option.label}
              onClick={() => onChange(option.id)}
              className={cn(
                'flex h-6 max-w-[160px] items-center gap-1.5 px-2 text-xs font-medium transition-colors',
                at === 0 ? 'rounded-l-md' : 'border-l border-line',
                at === options.length - 1 && 'rounded-r-md',
                chosen ? 'bg-accent text-on-accent' : 'text-muted hover:bg-hover hover:text-fg',
              )}
            >
              {Icon ? <Icon size={13} className="shrink-0" /> : null}
              {iconOnly ? null : <span className="min-w-0 truncate">{option.label}</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}
