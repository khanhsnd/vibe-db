import { Code2, Download } from 'lucide-react'

import { EXPORT_FORMATS, type ExportFormat } from '@/bridge/export'
import { Button } from '@/shared/ui/Button'
import { IconButton } from '@/shared/ui/IconButton'
import { MenuButton } from '@/shared/ui/MenuButton'

/**
 * Writing what a grid holds to a file, in the strip above that grid: the grid's own head strip,
 * or the surface's toolbar where it has one, so a reader never meets two strips over one table.
 * One button either way, because two would drift the day after they were written.
 */
export function ExportMenu({
  onExport,
  onGenerateCurl,
}: {
  onExport: (format: ExportFormat) => void
  onGenerateCurl?: (() => void) | undefined
}) {
  return (
    <div className="flex items-center gap-0.5">
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
      {onGenerateCurl ? (
        <IconButton label="Generate cURL" icon={<Code2 size={15} />} onClick={onGenerateCurl} />
      ) : null}
    </div>
  )
}
