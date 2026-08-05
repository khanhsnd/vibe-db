import type { ReactNode } from 'react'

import { colourInfo } from '@/bridge/connections'
import { cn } from '@/lib/cn'

/**
 * The wash a connection's colour puts behind a row, or `undefined` for a connection that has not
 * picked one. It is mixed here rather than kept as a second token per colour so that one number —
 * `--conn-tint` — decides how strong every one of them is, on both themes.
 */
export function colourTint(colour: string): string | undefined {
  const info = colourInfo(colour)
  if (info.value === '') return undefined
  return `color-mix(in srgb, ${info.colour} var(--conn-tint), transparent)`
}

/**
 * The colour itself, as the dot the menu is picked from. It is a function rather than a component
 * for the same reason the menu glyphs are — `useTreeMenu` builds its items in a `.ts` file and
 * names what they wear without becoming a component file itself.
 *
 * The colour in force wears a ring rather than a tick: a tick would need a gutter of its own
 * beside nine otherwise identical rows.
 */
export function colourSwatch(colour: string, selected: boolean): ReactNode {
  const info = colourInfo(colour)

  return (
    <span
      aria-hidden
      style={info.value === '' ? undefined : { backgroundColor: info.colour }}
      className={cn(
        'h-2.5 w-2.5 shrink-0 rounded-full',
        // No colour is drawn as the absence of one — an empty ring — rather than as a grey dot,
        // which would read as a tenth colour.
        info.value === '' && 'border border-line-strong',
        selected && 'ring-2 ring-fg/70',
      )}
    />
  )
}
