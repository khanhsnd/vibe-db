import type { ReactNode } from 'react'

interface EmptyProps {
  icon?: ReactNode
  title: string
  hint?: string | undefined
  action?: ReactNode
}

/**
 * The text is selectable because half of what an empty pane says is a *failure* — a read that
 * could not be made, with the server's own wording in the hint — and the first thing anybody does
 * with that wording is paste it somewhere.
 */
export function Empty({ icon, title, hint, action }: EmptyProps) {
  return (
    <div className="grid h-full place-items-center p-8">
      <div className="selectable flex max-w-[280px] flex-col items-center gap-2 text-center">
        {/* The glyph sits in a ring of the panel's own shade rather than loose on the canvas, so
            an empty pane reads as a placed thing and not as a half-drawn one. */}
        {icon ? (
          <div className="mb-1 grid h-12 w-12 place-items-center rounded-full border border-line bg-panel text-faint">
            {icon}
          </div>
        ) : null}
        <p className="text-sm font-medium text-muted">{title}</p>
        {hint ? <p className="text-xs leading-relaxed text-faint">{hint}</p> : null}
        {action ? <div className="mt-1">{action}</div> : null}
      </div>
    </div>
  )
}
