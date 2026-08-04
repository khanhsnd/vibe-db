import type { ReactNode } from 'react'
import { ArrowDownNarrowWide, ArrowUpNarrowWide, X } from 'lucide-react'

import { cn } from '@/lib/cn'
import { IconButton } from '@/shared/ui/IconButton'

import { filterLabel, type GridFiltering } from './filter'
import type { SortKey } from './resultSet'

interface FilterStripProps {
  filtering: GridFiltering
  sort: SortKey | null
  onSortChange?: ((sort: SortKey | null) => void) | undefined
}

/**
 * What the grid is narrowed by, in one strip over the header: a chip per filter and one for the
 * sort, each dropped by its own ✕. It is opened and closed from the cell menu's **Show/Hide
 * Filter & Sort**, and the conditions hold whether it is showing or not — so a strip that has
 * been hidden is not a grid that has been widened.
 *
 * Conditions are added from the cell menu rather than typed here: a filter means the value the
 * reader is pointing at, and a box to spell it in again would be a second way to say one thing.
 */
export function FilterStrip({ filtering, sort, onSortChange }: FilterStripProps) {
  const { filters, onChange } = filtering
  const empty = filters.length === 0 && sort === null

  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2"
    >
      <span className="shrink-0 text-2xs tracking-wide text-faint uppercase">Filter &amp; Sort</span>

      {empty ? (
        <span className="truncate text-xs text-muted">
          Right-click a cell to filter or sort by its value.
        </span>
      ) : null}

      {filters.map((filter, index) => (
        <Chip
          key={`${filter.column}:${filter.operator}:${filter.value}:${index}`}
          label={filterLabel(filter)}
          onRemove={() => onChange(filters.filter((_, at) => at !== index))}
        />
      ))}

      {sort !== null ? (
        <Chip
          label={sort.column}
          icon={
            sort.descending ? <ArrowDownNarrowWide size={11} /> : <ArrowUpNarrowWide size={11} />
          }
          onRemove={() => onSortChange?.(null)}
        />
      ) : null}

      <div className="min-w-2 flex-1" />

      <IconButton
        label="Hide Filter & Sort"
        icon={<X size={13} />}
        onClick={() => filtering.onShownChange(false)}
      />
    </div>
  )
}

interface ChipProps {
  label: string
  icon?: ReactNode
  onRemove: () => void
}

function Chip({ label, icon, onRemove }: ChipProps) {
  return (
    <span
      className={cn(
        'flex min-w-0 shrink-0 items-center gap-1 rounded border border-line-strong bg-panel',
        'py-0.5 pr-0.5 pl-1.5 font-mono text-2xs text-fg',
      )}
    >
      {icon ? <span className="shrink-0 text-accent">{icon}</span> : null}
      <span className="truncate">{label}</span>
      <IconButton label={`Remove ${label}`} icon={<X size={11} />} onClick={onRemove} />
    </span>
  )
}
