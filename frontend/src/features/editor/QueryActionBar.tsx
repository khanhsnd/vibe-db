import { Play, RotateCw, Square } from 'lucide-react'

import { Button } from '@/shared/ui/Button'
import { IconButton } from '@/shared/ui/IconButton'

const ROW_LIMITS = [100, 1000, 10_000] as const

interface QueryActionBarProps {
  running: boolean
  rowLimit: number
  onRun: () => void
  onStop: () => void
  onRowLimitChange: (limit: number) => void
}

export function QueryActionBar({
  running,
  rowLimit,
  onRun,
  onStop,
  onRowLimitChange,
}: QueryActionBarProps) {
  return (
    <div className="flex h-8 shrink-0 items-center gap-1.5 border-y border-line bg-panel px-2">
      <Button variant="primary" icon={<Play size={12} />} onClick={onRun} disabled={running}>
        Run
      </Button>
      <span className="mr-1 font-mono text-[10.5px] text-faint">F5</span>

      <Button variant="default" icon={<Square size={11} />} onClick={onStop} disabled={!running}>
        Stop
      </Button>

      <IconButton label="Refresh" icon={<RotateCw size={13} />} />

      <div className="flex-1" />

      <label className="flex items-center gap-1.5 text-[11.5px] text-muted">
        Limit
        <select
          value={rowLimit}
          onChange={(e) => onRowLimitChange(Number(e.target.value))}
          className="h-6 rounded-md border border-line bg-canvas px-1.5 font-mono text-[11.5px] text-fg outline-none focus:border-accent"
        >
          {ROW_LIMITS.map((limit) => (
            <option key={limit} value={limit}>
              {limit.toLocaleString('en-US')}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
