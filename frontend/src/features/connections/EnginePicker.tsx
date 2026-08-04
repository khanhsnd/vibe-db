import { ENGINES, type Engine } from '@/bridge/connections'
import { cn } from '@/lib/cn'

import { EngineIcon } from './EngineIcon'

interface EnginePickerProps {
  value: Engine
  onChange: (engine: Engine) => void
  /** Double-clicking a tile takes the choice, the way Next does. */
  onConfirm: () => void
}

/**
 * The first screen of a new connection: every engine as a tile, its own mark at a size a reader
 * picks out by shape. A list of names in a dropdown says less than the marks side by side — the
 * dolphin, the elephant and the leaf are what a reader already knows the three by — and the grid is
 * what grows when a fourth engine lands.
 */
export function EnginePicker({ value, onChange, onConfirm }: EnginePickerProps) {
  return (
    <div className="grid grid-cols-3 gap-2.5">
      {ENGINES.map((engine) => {
        const active = engine.value === value

        return (
          <button
            key={engine.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(engine.value)}
            onDoubleClick={onConfirm}
            className={cn(
              'flex flex-col items-center gap-2 rounded-lg border px-2 py-4 transition-colors',
              active
                ? 'border-accent bg-accent-subtle text-fg'
                : 'border-line bg-canvas text-muted hover:bg-hover hover:text-fg',
            )}
          >
            <EngineIcon engine={engine.value} className="h-11 w-11 rounded-lg p-1.5" />
            <span className="text-sm font-medium">{engine.label}</span>
          </button>
        )
      })}
    </div>
  )
}
