import { useMemo, useState } from 'react'
import { Search, X } from 'lucide-react'

import { ENGINES, engineInfo, type Engine, type Profile } from '@/bridge/connections'
import { cn } from '@/lib/cn'
import { useConnectionsStore } from '@/stores/connections'

import { EngineIcon } from './EngineIcon'

/** How many of the engines already in use are offered above the grid. */
const RECENT_LIMIT = 4

interface EnginePickerProps {
  value: Engine
  onChange: (engine: Engine) => void
  /** Double-clicking a tile takes the choice, the way Next does. */
  onConfirm: () => void
}

/**
 * The first screen of a new connection: every engine as a tile, its own mark at a size a reader
 * picks out by shape. A list of names in a dropdown says less than the marks side by side — the
 * dolphin, the elephant and the leaf are what a reader already knows those three by — and the grid
 * is what grows as engines land, which is why it is a grid and not a row.
 *
 * Above it are the two things that shorten the walk to a tile: a box that narrows the grid by name,
 * and the engines this reader already connects to, which is nearly always the one they want.
 */
export function EnginePicker({ value, onChange, onConfirm }: EnginePickerProps) {
  const profiles = useConnectionsStore((s) => s.profiles)
  const [query, setQuery] = useState('')

  const recent = useMemo(() => recentEngines(profiles), [profiles])

  const needle = query.trim().toLowerCase()
  const matches = ENGINES.filter((engine) => engine.label.toLowerCase().includes(needle))

  // Enter keeps the tile the reader has already picked where it still matches, and takes the first
  // match where it does not — typing `mon` and pressing Enter is a choice, not a scroll.
  const confirmTyped = () => {
    const chosen = matches.find((engine) => engine.value === value) ?? matches[0]
    if (!chosen) return
    onChange(chosen.value)
    onConfirm()
  }

  const showRecent = needle === '' && recent.length > 0

  return (
    <div className="flex flex-col gap-3">
      <div className="flex h-8 items-center gap-1.5 rounded-md border border-line bg-canvas px-2 transition-colors focus-within:border-accent">
        <Search size={13} className="shrink-0 text-faint" />
        <input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setQuery('')
            if (e.key === 'Enter') confirmTyped()
          }}
          placeholder="Search connection types"
          spellCheck={false}
          className="min-w-0 flex-1 bg-transparent text-sm text-fg outline-none placeholder:text-faint"
        />
        {query ? (
          <button
            type="button"
            aria-label="Clear search"
            onClick={() => setQuery('')}
            className="grid shrink-0 place-items-center text-faint hover:text-fg"
          >
            <X size={13} />
          </button>
        ) : null}
      </div>

      {showRecent ? (
        <div className="flex flex-col gap-1.5">
          <Heading>Recent</Heading>
          <div className="flex flex-wrap gap-1.5">
            {recent.map((engine) => (
              <button
                key={engine}
                type="button"
                aria-pressed={engine === value}
                onClick={() => onChange(engine)}
                onDoubleClick={onConfirm}
                className={cn(
                  'flex items-center gap-1.5 rounded-full border py-1 pr-2.5 pl-1.5 text-xs transition-colors',
                  engine === value
                    ? 'border-accent bg-accent-subtle text-fg'
                    : 'border-line bg-canvas text-muted hover:bg-hover hover:text-fg',
                )}
              >
                <EngineIcon engine={engine} className="h-4 w-4 rounded-full p-0.5" />
                {engineInfo(engine).label}
              </button>
            ))}
          </div>
        </div>
      ) : null}

      {matches.length === 0 ? (
        <p className="py-6 text-center text-xs text-faint">
          No connection type matches “{query.trim()}”.
        </p>
      ) : (
        <div className="flex flex-col gap-1.5">
          {showRecent ? <Heading>All types</Heading> : null}
          <div className="grid grid-cols-3 gap-2.5">
            {matches.map((engine) => {
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
        </div>
      )}
    </div>
  )
}

function Heading({ children }: { children: React.ReactNode }) {
  return <span className="text-2xs font-medium tracking-wide text-faint uppercase">{children}</span>
}

/**
 * The engines this reader already connects to, newest first. A profile is appended to
 * `connections.json` as it is saved, so the end of that list is the most recent one made — no
 * second thing to store, and nothing to keep in step with the connections themselves.
 */
function recentEngines(profiles: Profile[]): Engine[] {
  const seen: Engine[] = []

  for (const profile of [...profiles].reverse()) {
    const { value } = engineInfo(profile.engine)
    if (!seen.includes(value)) seen.push(value)
  }

  return seen.slice(0, RECENT_LIMIT)
}
