import { ChevronLeft, ChevronRight } from 'lucide-react'

import { ALL_ROWS, DEFAULT_PAGE_SIZE, PAGE_SIZES } from '@/bridge/rows'
import { IconButton } from '@/shared/ui/IconButton'
import { RowLimit } from '@/shared/ui/RowLimit'

const numbers = new Intl.NumberFormat('en-US')

interface TablePagerProps {
  page: number
  limit: number
  total: number
  loading: boolean
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
}

/** The right-hand side of the grid footer: where in the walk we are, and how big a step is. */
export function TablePager({
  page,
  limit,
  total,
  loading,
  onPageChange,
  onLimitChange,
}: TablePagerProps) {
  // All collapses the walk into a single page, so it is sized by the total rather than the limit.
  const size = limit === ALL_ROWS ? Math.max(total, 1) : limit
  const pageCount = Math.max(1, Math.ceil(total / size))
  const first = total === 0 ? 0 : page * size + 1
  const last = Math.min(total, (page + 1) * size)

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
        disabled={page + 1 >= pageCount || loading}
        onClick={() => onPageChange(page + 1)}
      />

      <span className="font-mono">
        {numbers.format(first)}–{numbers.format(last)} of {numbers.format(total)}
      </span>
      <span className="text-faint">·</span>
      <span>
        Page {numbers.format(page + 1)} of {numbers.format(pageCount)}
      </span>

      <RowLimit
        label="Rows"
        presets={PAGE_SIZES}
        fallback={DEFAULT_PAGE_SIZE}
        value={limit}
        onChange={onLimitChange}
      />
    </div>
  )
}
