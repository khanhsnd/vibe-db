import type { AppError } from '@/bridge/errors'

/**
 * A failure reported under the thing it happened to, in one line: a cell that would not save, a
 * catalogue read that failed while rows were already on screen. It is a strip rather than a panel
 * because what it interrupts is still worth looking at — see `ErrorPanel`, which is what a surface
 * with *nothing* to show puts in its place.
 *
 * **The text is selectable**, like every other failure in the app: the first thing anybody does
 * with a server's wording is paste it somewhere. It is truncated to one line for the layout's sake
 * and selecting it still copies the whole of it; the rest is in the tooltip either way.
 */
export function ErrorStrip({ error }: { error: AppError }) {
  return (
    <p
      title={error.detail ? `${error.message}\n${error.detail}` : error.message}
      className="selectable shrink-0 truncate border-t border-danger/40 bg-danger-subtle px-2 py-1 text-xs text-danger"
    >
      {error.message}
    </p>
  )
}
