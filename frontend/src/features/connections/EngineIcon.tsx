import { engineInfo, type Engine } from '@/bridge/connections'
import { cn } from '@/lib/cn'

import { ENGINE_GLYPHS, GLYPH_BOX } from './engineGlyphs'

/**
 * Which engine a connection speaks, as a chip: the engine's own mark in white on its own colour.
 * A `Database` glyph seven times over would say nothing — the dolphin, the elephant, the leaf, the
 * stack of discs, the turned page, the pane and the ring are what a reader knows each by, and the
 * shape still reads where the colour cannot.
 */
const CHIPS: Record<Engine, string> = {
  mysql: 'bg-engine-mysql',
  postgres: 'bg-engine-postgres',
  sqlite: 'bg-engine-sqlite',
  sqlserver: 'bg-engine-sqlserver',
  oracle: 'bg-engine-oracle',
  mongo: 'bg-engine-mongo',
  redis: 'bg-engine-redis',
}

export function EngineIcon({ engine, className }: { engine: string; className?: string }) {
  const { value, label } = engineInfo(engine)
  const glyph = ENGINE_GLYPHS[value]

  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        'grid h-4 w-4 shrink-0 place-items-center rounded-sm p-px text-on-accent',
        CHIPS[value],
        className,
      )}
    >
      <svg viewBox={`0 0 ${GLYPH_BOX} ${GLYPH_BOX}`} className="h-full w-full">
        <path
          d={glyph.path}
          fill="currentColor"
          fillRule={glyph.evenOdd ? 'evenodd' : 'nonzero'}
        />
      </svg>
    </span>
  )
}
