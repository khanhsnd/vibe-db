import { cn } from '@/lib/cn'

import type { DiffCounts } from './panes'

const numbers = new Intl.NumberFormat('en-US')

/**
 * What the comparison found, in the colours it drew it in. It is the legend and the count at once:
 * a reader looking at a wash of amber wants to know what amber means, and a reader who has scrolled
 * past the end wants to know how much of it there was. Nothing here is clickable — what narrows the
 * panes is the toolbar's **Differences only**, and a count that also filtered would be two controls
 * wearing one label.
 *
 * `same` is drawn last and unmarked, because it is the only one of the four that is not a finding.
 */
export function CompareSummary({ counts, words }: { counts: DiffCounts; words: CompareWords }) {
  return (
    <div className="flex shrink-0 items-center gap-2 text-2xs whitespace-nowrap">
      <Count value={counts.changed} label="changed" tint="bg-grid-diff-changed text-diff-changed" />
      <Count value={counts.removed} label={words.onlyLeft} tint="bg-grid-diff-removed text-diff-removed" />
      <Count value={counts.added} label={words.onlyRight} tint="bg-grid-diff-added text-diff-added" />
      <span className="text-faint">{numbers.format(counts.same)} identical</span>
    </div>
  )
}

/** How a comparison words the two one-sided outcomes, which differ between its halves. */
export interface CompareWords {
  onlyLeft: string
  onlyRight: string
}

function Count({ value, label, tint }: { value: number; label: string; tint: string }) {
  return (
    <span
      className={cn(
        'rounded px-1.5 py-0.5 font-medium',
        value === 0 ? 'text-faint' : tint,
      )}
    >
      {numbers.format(value)} {label}
    </span>
  )
}
