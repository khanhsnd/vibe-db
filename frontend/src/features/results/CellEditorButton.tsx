import { PanelBottom } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'

/**
 * The one way in to a surface's cell-editor panel, in one component and therefore in two places:
 * the head strip the grid draws for itself, and a surface's own toolbar where there is one —
 * exactly as `ExportMenu` and `FilterSortButton` sit in both.
 */
export function CellEditorButton({ shown, onToggle }: { shown: boolean; onToggle: () => void }) {
  return (
    <Button
      variant="subtle"
      icon={<PanelBottom size={14} className="text-accent" />}
      aria-pressed={shown}
      onClick={onToggle}
      className={cn(shown && 'bg-selected text-fg')}
    >
      Cell Editor
    </Button>
  )
}
