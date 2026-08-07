import { cn } from '@/lib/cn'

/**
 * The strip across the top of a dialog that is **tabs and not a wizard**.
 *
 * The two transfer wizards ask one question a screen because their answers depend on each other —
 * what a delimiter is worth asking about depends on the format. A dialog that uses this has no such
 * dependency: every tab is already answered, so a reader who wants the default presses the button
 * on the screen it opened on, and reaching the third answer never means passing the second.
 *
 * **Every tab is reachable at any time**, which is the whole difference, so nothing here can be
 * disabled.
 */
export function TabStrip<T extends string>({
  tabs,
  at,
  onMove,
}: {
  tabs: readonly T[]
  at: T
  onMove: (to: T) => void
}) {
  return (
    <div role="tablist" className="flex shrink-0 gap-1 border-b border-line bg-bar px-2 pt-1.5">
      {tabs.map((tab) => (
        <button
          key={tab}
          type="button"
          role="tab"
          aria-selected={tab === at}
          onClick={() => onMove(tab)}
          className={cn(
            '-mb-px rounded-t-md border border-b-0 px-3 py-1.5 text-xs font-medium',
            tab === at
              ? 'border-line bg-elevated text-fg'
              : 'border-transparent text-muted hover:text-fg',
          )}
        >
          {tab}
        </button>
      ))}
    </div>
  )
}
