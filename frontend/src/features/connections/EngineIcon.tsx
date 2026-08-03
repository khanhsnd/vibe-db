import { engineInfo, type Engine } from '@/bridge/connections'
import { cn } from '@/lib/cn'

/**
 * Which engine a connection speaks, as a chip. It is a letter on a colour rather than a line icon
 * because the two engines are both databases: a `Database` glyph twice over says nothing, and the
 * letter still reads where the colour cannot — a grey-scale screen, a colour-blind reader.
 */
const CHIPS: Record<Engine, { letter: string; className: string }> = {
  mysql: { letter: 'M', className: 'bg-engine-mysql' },
  postgres: { letter: 'P', className: 'bg-engine-postgres' },
}

export function EngineIcon({ engine, className }: { engine: string; className?: string }) {
  const { value, label } = engineInfo(engine)
  const chip = CHIPS[value]

  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        'grid h-4 w-4 shrink-0 place-items-center rounded-sm',
        'text-2xs leading-none font-bold text-on-accent',
        chip.className,
        className,
      )}
    >
      {chip.letter}
    </span>
  )
}
