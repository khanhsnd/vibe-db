import { Columns3, ListTree } from 'lucide-react'

import type { ExportFormat } from '@/bridge/export'
import { CellEditorButton } from '@/features/results/CellEditorButton'
import { ExportMenu } from '@/features/results/ExportMenu'
import { FilterSortButton } from '@/features/results/FilterSortButton'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'

import { KeySearch } from './KeySearch'
import type { KeySearch as Search } from './keySpace'

export type PanelKind = 'cell' | 'column'

/**
 * What the strip carries over a **key space** instead of what it carries over a table. Its presence is
 * the whole switch, and it is set from `EngineInfo.keys` — never from an engine's name.
 *
 * Three things go and two arrive. **Filter & Sort** goes because a key space cannot answer it: Redis
 * narrows a read one way, a glob against a key's name, so the twenty operators of the builder are
 * twenty ways of asking for something that would have to be answered by reading every value on the
 * server. The **Cell Editor** toggle goes because that panel is always open there — there is nothing
 * for a toggle to save on five columns, three of them one line long. What arrives is the search that
 * replaces the builder, and **Flatten**, which is how the tree is turned off.
 */
export interface KeySpaceControls {
  search: Search
  onSearchChange: (search: Search) => void
  flatten: boolean
  onFlattenToggle: () => void
}

interface TableToolbarProps {
  active: ReadonlySet<PanelKind>
  onToggle: (kind: PanelKind) => void
  /** Whether the Filter & Sort panel is showing, and how much it is holding — see the button. */
  filterShown: boolean
  filterCount: number
  onFilterToggle: () => void
  /** Left out until there are rows to write, which is what keeps Export off an empty table. */
  onExport?: ((format: ExportFormat) => void) | undefined
  onGenerateCurl?: (() => void) | undefined
  /** Set only where the namespace is a key space, and then it replaces three of the four above. */
  keySpace?: KeySpaceControls | undefined
}

/**
 * The one strip over a table's grid: the panels a reader can open on the left with Filter & Sort
 * beside them, and Export against the right, where every grid keeps it. The grid draws no head
 * strip of its own beneath this one — `headStrip={false}` is what says so.
 */
export function TableToolbar({
  active,
  onToggle,
  filterShown,
  filterCount,
  onFilterToggle,
  onExport,
  onGenerateCurl,
  keySpace,
}: TableToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
    >
      {keySpace ? (
        <>
          <KeySearch search={keySpace.search} onChange={keySpace.onSearchChange} />

          <Button
            variant="subtle"
            icon={<ListTree size={14} />}
            aria-pressed={keySpace.flatten}
            onClick={keySpace.onFlattenToggle}
            className={cn(keySpace.flatten && 'bg-selected text-fg')}
          >
            Flatten
          </Button>
        </>
      ) : (
        <>
          <CellEditorButton shown={active.has('cell')} onToggle={() => onToggle('cell')} />

          <FilterSortButton shown={filterShown} count={filterCount} onToggle={onFilterToggle} />
        </>
      )}

      <Button
        variant="subtle"
        icon={<Columns3 size={14} />}
        aria-pressed={active.has('column')}
        onClick={() => onToggle('column')}
        className={cn(active.has('column') && 'bg-selected text-fg')}
      >
        Column
      </Button>

      <div className="min-w-2 flex-1" />

      {onExport ? <ExportMenu onExport={onExport} onGenerateCurl={onGenerateCurl} /> : null}
    </div>
  )
}
