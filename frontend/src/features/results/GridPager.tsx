import { ChevronLeft, ChevronRight } from 'lucide-react'

import { ALL_ROWS, DEFAULT_PAGE_SIZE, PAGE_SIZES } from '@/bridge/rows'
import { IconButton } from '@/shared/ui/IconButton'
import { RowLimit } from '@/shared/ui/RowLimit'

const numbers = new Intl.NumberFormat('en-US')

const MORE_ON_SERVER = 'The read stopped at its row limit, so the server has more rows than this.'

interface GridPagerProps {
  page: number
  size: number
  total: number
  /**
   * The total is a floor rather than a count, because the read stopped at a cap. Next then
   * stays alive on the last page, since the caller can go and fetch what is missing.
   */
  partial?: boolean
  loading?: boolean
  onPageChange: (page: number) => void
  onSizeChange: (size: number) => void
}

/**
 * The right-hand side of the grid footer: where in the rows we are, and how big a step is. Both
 * grids show it, and neither one cares whether the step is walked on the server or cut out of
 * rows already in memory.
 */
export function GridPager({
  page,
  size,
  total,
  partial = false,
  loading = false,
  onPageChange,
  onSizeChange,
}: GridPagerProps) {
  // All collapses the walk into a single page, so it is sized by the total rather than the size.
  const step = size === ALL_ROWS ? Math.max(total, 1) : size
  const pageCount = Math.max(1, Math.ceil(total / step))
  const first = total === 0 ? 0 : page * step + 1
  const last = Math.min(total, (page + 1) * step)
  // All is one page by definition, so only a bounded window can be walked past what is held.
  const beyond = partial && size !== ALL_ROWS

  return (
    <div className="flex shrink-0 items-center gap-2">
      {loading ? <span className="text-faint">Loading…</span> : null}

      <IconButton
        label="Previous page"
        icon={<ChevronLeft size={14} />}
        disabled={page === 0 || loading}
        onClick={() => onPageChange(page - 1)}
      />
      <IconButton
        label="Next page"
        icon={<ChevronRight size={14} />}
        disabled={loading || (page + 1 >= pageCount && !beyond)}
        onClick={() => onPageChange(page + 1)}
      />

      <span className="font-mono" title={partial ? MORE_ON_SERVER : undefined}>
        {numbers.format(first)}–{numbers.format(last)} of {numbers.format(total)}
        {partial ? '+' : ''}
      </span>
      <span className="text-faint">·</span>
      <span>
        Page {numbers.format(page + 1)} of {numbers.format(pageCount)}
      </span>

      <RowLimit
        label="Rows"
        presets={PAGE_SIZES}
        fallback={DEFAULT_PAGE_SIZE}
        value={size}
        onChange={onSizeChange}
      />
    </div>
  )
}
