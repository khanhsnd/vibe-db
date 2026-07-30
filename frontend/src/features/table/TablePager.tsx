import { ChevronLeft, ChevronRight } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import { IconButton } from '@/shared/ui/IconButton'

import { PAGE_SIZES } from './useTableData'

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
  const pageCount = Math.max(1, Math.ceil(total / limit))
  const first = total === 0 ? 0 : page * limit + 1
  const last = Math.min(total, (page + 1) * limit)

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

      <label className="flex shrink-0 items-center gap-1.5">
        Rows
        <select
          value={limit}
          onChange={(e) => onLimitChange(Number(e.target.value))}
          className="h-6 rounded-md border border-line bg-canvas px-1.5 font-mono text-xs text-fg outline-none focus:border-accent"
        >
          {PAGE_SIZES.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
