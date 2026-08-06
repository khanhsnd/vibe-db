import { cn } from '@/lib/cn'
import { Empty } from '@/shared/ui/Empty'
import { CopyButton } from '@/shared/ui/CopyButton'

/**
 * The statements a migration would run, exactly as it will run them.
 *
 * They are the **engine's own spelling**, asked for rather than built here — `PreviewTableChanges`
 * and `PreviewRowWrite` hand the same list Apply is given to the driver — so the script the reader
 * copies is the script that runs. Nothing on this side has ever spelled a statement, which is the
 * rule that makes that true.
 *
 * It is a transcript rather than a grid: a statement is laid out over as many lines as it takes and
 * a row of them in cells would be one clipped column — see [grid.md](../../../docs/grid.md).
 */
export function CompareScript({
  statements,
  into,
  empty,
}: {
  statements: readonly string[]
  /** The table the script writes to, said once above it: only one of the two ever changes. */
  into: string
  /** What to say when there is nothing to run — which is not the same as nothing to compare. */
  empty: string
}) {
  if (statements.length === 0) {
    return <Empty title="Nothing to run" hint={empty} />
  }

  const whole = statements.map((statement) => `${statement};`).join('\n\n')

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-7 shrink-0 items-center gap-2 border-b border-line bg-bar px-2">
        <span className="min-w-0 truncate text-2xs text-muted">
          {statements.length} statement{statements.length === 1 ? '' : 's'} against{' '}
          <span className="font-mono text-fg">{into}</span>
        </span>
        <div className="flex-1" />
        <CopyButton text={whole} label="Copy the whole script" />
      </div>

      <ol className="scroll-thin min-h-0 flex-1 overflow-auto">
        {statements.map((statement, at) => (
          <li
            key={`${at}:${statement}`}
            className={cn(
              'flex gap-2 border-b border-line/60 px-2 py-1',
              'font-mono text-2xs whitespace-pre-wrap text-fg',
            )}
          >
            <span className="w-6 shrink-0 text-right text-faint tabular-nums">{at + 1}</span>
            <span className="selectable min-w-0 flex-1">{statement};</span>
          </li>
        ))}
      </ol>
    </div>
  )
}
