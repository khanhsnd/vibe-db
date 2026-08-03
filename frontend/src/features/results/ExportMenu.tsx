import { Download } from 'lucide-react'

import { EXPORT_FORMATS, type ExportFormat } from '@/bridge/export'
import { Button } from '@/shared/ui/Button'
import { MenuButton } from '@/shared/ui/MenuButton'

/**
 * Writing what a grid holds to a file, in the strip above that grid: the grid's own head strip,
 * or the surface's toolbar where it has one, so a reader never meets two strips over one table.
 * One button either way, because two would drift the day after they were written.
 */
export function ExportMenu({ onExport }: { onExport: (format: ExportFormat) => void }) {
  return (
    <MenuButton
      align="end"
      items={EXPORT_FORMATS.map((format) => ({
        label: format.label,
        icon: <Download size={13} />,
        onSelect: () => onExport(format.value),
      }))}
    >
      <Button variant="subtle" icon={<Download size={14} className="text-accent" />}>
        Export
      </Button>
    </MenuButton>
  )
}
