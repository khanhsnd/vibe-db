import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
  Settings,
} from 'lucide-react'

import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { Checkbox, TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'

import { pageCount } from './paging'

const numbers = new Intl.NumberFormat('en-US')

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
 * The right-hand side of the grid footer: the walk itself — first, back, the page in force,
 * forward, last — and one gear holding how big a step is. Nothing here counts rows; that is the
 * status line's job, so the walk stays a walk. Both grids show it, and neither one cares whether
 * the step is taken on the server or cut out of rows already in memory.
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
  const last = pageCount(size, total) - 1
  // All is one page by definition, so only a bounded window can be walked past what is held.
  const beyond = partial && size !== ALL_ROWS

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {loading ? <Loader2 size={13} className="mr-1 shrink-0 animate-spin text-accent" /> : null}

      <IconButton
        label="First page"
        icon={<ChevronsLeft size={14} />}
        disabled={page === 0 || loading}
        onClick={() => onPageChange(0)}
      />
      <IconButton
        label="Previous page"
        icon={<ChevronLeft size={14} />}
        disabled={page === 0 || loading}
        onClick={() => onPageChange(page - 1)}
      />

      <PageBox
        page={page}
        last={last}
        // A floor is not a ceiling: a page past a partial total is one the caller can still fetch.
        max={beyond ? Number.MAX_SAFE_INTEGER : last}
        disabled={loading}
        onPageChange={onPageChange}
      />

      <IconButton
        label="Next page"
        icon={<ChevronRight size={14} />}
        disabled={loading || (page >= last && !beyond)}
        onClick={() => onPageChange(page + 1)}
      />
      <IconButton
        label="Last page"
        icon={<ChevronsRight size={14} />}
        disabled={loading || page >= last}
        onClick={() => onPageChange(last)}
      />

      <PageSize size={size} onSizeChange={onSizeChange} />
    </div>
  )
}

interface PageBoxProps {
  page: number
  last: number
  max: number
  disabled: boolean
  onPageChange: (page: number) => void
}

/**
 * The page in force, typed into rather than read off arrows. The draft is separate from the page
 * so that clearing the box does not fight the page the grid is showing, and it is dropped on
 * commit: what is then displayed is the page the caller settled on, which may be a clamped one.
 */
function PageBox({ page, last, max, disabled, onPageChange }: PageBoxProps) {
  const [draft, setDraft] = useState<string | null>(null)

  const commit = () => {
    const asked = Number(draft)
    setDraft(null)
    if (draft !== null && Number.isInteger(asked) && asked >= 1) {
      onPageChange(Math.min(asked - 1, max))
    }
  }

  return (
    <input
      aria-label="Page"
      title={`Page ${numbers.format(page + 1)} of ${numbers.format(last + 1)}`}
      inputMode="numeric"
      disabled={disabled}
      value={draft ?? String(page + 1)}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === 'Enter') commit()
        if (event.key === 'Escape') setDraft(null)
      }}
      className={`${TOOLBAR_CONTROL} mx-1 w-12 text-center font-mono`}
    />
  )
}

/**
 * How big a step is, behind a gear: whether the read is bounded at all, and by how many. Turning
 * the bound off asks for every row — the read that streams and reports itself — which is why it is
 * a checkbox rather than another entry in a list of sizes: it is a different kind of read.
 *
 * The typed count is kept while the bound is off, so turning it back on restores what it was.
 */
function PageSize({ size, onSizeChange }: { size: number; onSizeChange: (size: number) => void }) {
  const limited = size !== ALL_ROWS
  const [draft, setDraft] = useState(() => String(limited ? size : DEFAULT_PAGE_SIZE))

  const countOf = (text: string) => {
    const rows = Number(text)
    return Number.isInteger(rows) && rows > 0 ? rows : null
  }

  const type = (text: string) => {
    setDraft(text)
    const rows = countOf(text)
    if (rows !== null) onSizeChange(rows)
  }

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <IconButton label="Records per page" icon={<Settings size={14} />} />
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          side="top"
          align="end"
          sideOffset={6}
          className="z-50 rounded-md border border-line-strong bg-elevated p-2.5 shadow-[var(--shadow-overlay)]"
        >
          <div className="flex items-center gap-2 text-sm text-fg">
            <Checkbox
              label="Limit records"
              checked={limited}
              onChange={(event) =>
                onSizeChange(
                  event.target.checked ? (countOf(draft) ?? DEFAULT_PAGE_SIZE) : ALL_ROWS,
                )
              }
            />

            <input
              type="number"
              min={1}
              step={100}
              aria-label="Records per page"
              disabled={!limited}
              value={draft}
              onChange={(event) => type(event.target.value)}
              className={`${TOOLBAR_CONTROL} w-20 font-mono`}
            />
            records per page
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
