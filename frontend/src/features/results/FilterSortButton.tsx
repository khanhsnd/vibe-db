import { SlidersHorizontal } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'

interface FilterSortButtonProps {
  /** Whether the panel is showing, which the button reads back as pressed. */
  shown: boolean
  onToggle: () => void
  /**
   * The conditions and sort keys the grid is holding. It is drawn as a count beside the label so
   * that a reader who closed the panel can still see the grid is narrowed — hiding the panel is
   * not widening the grid, and a button saying nothing would suggest otherwise.
   */
  count: number
}

/**
 * The one way in to `FilterSortPanel`, in one component and therefore in two places: the head
 * strip the grid draws for itself, and a surface's own toolbar where there is one — exactly as
 * `ExportMenu` sits in both.
 */
export function FilterSortButton({ shown, onToggle, count }: FilterSortButtonProps) {
  return (
    <Button
      variant="subtle"
      icon={<SlidersHorizontal size={14} className="text-accent" />}
      aria-pressed={shown}
      onClick={onToggle}
      className={cn(shown && 'bg-selected text-fg')}
    >
      Filter &amp; Sort
      {count > 0 ? (
        <span className="rounded-full bg-accent px-1.5 text-2xs text-on-accent">{count}</span>
      ) : null}
    </Button>
  )
}
