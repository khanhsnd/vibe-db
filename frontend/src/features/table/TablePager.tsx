import { ChevronLeft, ChevronRight } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import { ALL_ROWS, DEFAULT_PAGE_SIZE, PAGE_SIZES } from '@/bridge/rows'
import { IconButton } from '@/shared/ui/IconButton'
import { RowLimit } from '@/shared/ui/RowLimit'

const numbers = new Intl.NumberFormat('en-US')

interface TablePagerProps {
  page: number
  limit: number
  total: number
  loading: boolean
  error: AppError | null
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
}

export function TablePager({
  page,
  limit,
  total,
  loading,
  error,
  onPageChange,
  onLimitChange,
}: TablePagerProps) {
  // All collapses the walk into a single page, so it is sized by the total rather than the limit.
  const size = limit === ALL_ROWS ? Math.max(total, 1) : limit
  const pageCount = Math.max(1, Math.ceil(total / size))
  const first = total === 0 ? 0 : page * size + 1
  const last = Math.min(total, (page + 1) * size)

  return (
    <div className="flex h-8 shrink-0 items-center gap-2 border-t border-line bg-panel px-2 text-xs text-muted">
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

      <span className="shrink-0 font-mono">
        {numbers.format(first)}–{numbers.format(last)} of {numbers.format(total)}
      </span>
      <span className="text-faint">·</span>
      <span className="shrink-0">
        Page {numbers.format(page + 1)} of {numbers.format(pageCount)}
      </span>

      {loading ? <span className="shrink-0 text-faint">Loading…</span> : null}

      <div className="min-w-2 flex-1" />

      {error ? <span className="truncate text-danger">{error.message}</span> : null}

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
