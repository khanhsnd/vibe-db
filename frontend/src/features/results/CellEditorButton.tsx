import { PanelBottom } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'

/**
 * The one way in to a surface's cell-editor panel, in one component and therefore in two places:
 * the head strip the grid draws for itself, and a surface's own toolbar where there is one —
 * exactly as `ExportMenu` and `FilterSortButton` sit in both.
 *
 * `label` is for the surface whose panel only **reads**. `CellEditor` already draws itself without
 * its editing controls when the rows cannot be written back, so a button still calling it an editor
 * would be the one thing on such a surface promising something the panel does not do — a server
 * monitor's panes are all reads. It is a word and nothing else: the same panel, the same toggle.
 */
export function CellEditorButton({
  shown,
  onToggle,
  label = 'Cell Editor',
}: {
  shown: boolean
  onToggle: () => void
  label?: string
}) {
  return (
    <Button
      variant="subtle"
      icon={<PanelBottom size={14} className="text-accent" />}
      aria-pressed={shown}
      onClick={onToggle}
      className={cn(shown && 'bg-selected text-fg')}
    >
      {label}
    </Button>
  )
}
