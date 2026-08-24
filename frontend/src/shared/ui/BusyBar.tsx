import { RunningCat } from './RunningCat'

/**
 * That a surface is working, said **under** its toolbar and across the whole width of it: the cat
 * running on the spot, the word for what is happening, and a four-pixel line sweeping beneath the
 * two for as long as the answer is out.
 *
 * It is a strip of its own rather than a chip in the toolbar, and that is the point. A toolbar is a
 * row of controls of one height, and a cat and a word wedged between two of them are neither — they
 * pushed what was beside them about and read as something gone wrong with the alignment. Below the
 * toolbar there is nothing to be out of line with, and the whole width is available to a thing whose
 * job is to be *noticed*.
 *
 * Which is the job, because **a spinning icon is where a reader is not looking.** The one on the
 * Refresh button under a grid is thirteen pixels in a bar of thirteen-pixel icons, so a statement
 * taking forty seconds looked, from a foot away, exactly like one that had already come back.
 *
 * The sweep and the run are the CSS's alone (`.busy-bar`, `.cat-run`), so nothing re-renders while
 * they go: a busy tab costs exactly what an idle one does, which is nothing.
 */
export function BusyBar({ label }: { label: string }) {
  return (
    <div role="progressbar" aria-label={label} aria-busy="true" className="shrink-0">
      {/* The toolbar's own surface and its own left padding, so the strip reads as the row under it
          rather than as something laid over the pane. */}
      <div className="flex h-5 items-center gap-1.5 bg-bar px-2 text-xs text-muted">
        <RunningCat className="text-fg" />
        {label}
      </div>

      <div className="busy-bar" />
    </div>
  )
}
