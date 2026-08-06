import { useEffect, useRef, type RefObject } from 'react'

import { ResultGrid } from '@/features/results/ResultGrid'
import type { CellRef } from '@/features/results/resultSet'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'

import { TAKE_WIDTH, type CompareGrid, type ComparePanes as Panes } from './panes'

/** Which way round the two panes sit. Side by side is the default: a row is read across. */
export type CompareLayout = 'columns' | 'rows'

interface ComparePanesProps {
  panes: Panes
  layout: CompareLayout
  /** What each side is called, drawn over it: neither grid can say for itself which table it is. */
  leftLabel: string
  rightLabel: string
  /** Flips the tick a row stands for. Every pane carries the same one, so any of them flips it. */
  onTake: (picks: string[]) => void
}

/**
 * The two sides of a comparison — the shapes or the rows — drawn as two `ResultGrid`s that hold the
 * same rows in the same order, so row *n* of one is row *n* of the other and the pair reads across.
 *
 * **The ticks are a third grid between them.** A tick is not a fact about either table, and a
 * column living inside a pane slides out of sight the moment that pane is scrolled sideways — which
 * is the one control a reader must never have to chase. So the strip is its own pane, as wide as
 * the single column it holds, and it moves only up and down.
 *
 * **Every pane scrolls as one**, and the splitter between the two sides **resizes both**: each
 * takes half to begin with and a drag moves the boundary, so one side is never left holding a strip
 * too narrow to read. The layout is part of the splitter's id, so the width the reader gave two
 * side-by-side panes is not read back as the height of two stacked ones.
 */
export function ComparePanes({
  panes,
  layout,
  leftLabel,
  rightLabel,
  onTake,
}: ComparePanesProps) {
  const leftView = useRef<HTMLDivElement>(null)
  const middleView = useRef<HTMLDivElement>(null)
  const rightView = useRef<HTMLDivElement>(null)

  useScrollSync([leftView, middleView, rightView], [layout, panes])

  const orientation = layout === 'columns' ? 'horizontal' : 'vertical'

  return (
    <Split
      id={`vibe-db.compare.${layout}`}
      orientation={orientation}
      panelIds={['left', 'right']}
      className="min-h-0 flex-1"
    >
      <SplitPanel id="left" defaultSize="50%" minSize="15%" className="min-h-0">
        <Side pane={panes.left} viewportRef={leftView} label={leftLabel} onTake={onTake} />
      </SplitPanel>

      <SplitHandle orientation={orientation} />

      {/* The strip rides on the right panel's leading edge rather than standing between the two
          panels, because what sits between them is the splitter — and a panel of its own would be
          a third thing the reader could drag. */}
      <SplitPanel id="right" defaultSize="50%" minSize="15%" className="min-h-0">
        <div className="flex h-full min-h-0">
          {panes.middle ? (
            <TickStrip pane={panes.middle} viewportRef={middleView} onTake={onTake} />
          ) : null}

          <div className="min-w-0 flex-1">
            <Side pane={panes.right} viewportRef={rightView} label={rightLabel} onTake={onTake} />
          </div>
        </div>
      </SplitPanel>
    </Split>
  )
}

function Side({
  pane,
  viewportRef,
  label,
  onTake,
}: {
  pane: CompareGrid
  viewportRef: RefObject<HTMLDivElement | null>
  label: string
  onTake: (picks: string[]) => void
}) {
  // Only a tick column takes an edit — every other one is `readOnly` — so an edit that reaches here
  // is a tick, and what it means is the row's own pick.
  const take = (ref: CellRef) => onTake(pane.picksAt(ref.row))

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* Which table this half is. It is the one thing neither grid can say for itself, and two
          panes of `id`, `name`, `email` are otherwise indistinguishable. */}
      <div className="flex h-6 shrink-0 items-center border-b border-line bg-bar px-2">
        <span className="min-w-0 truncate font-mono text-2xs text-fg">{label}</span>
      </div>

      <div className="min-h-0 flex-1">
        <ResultGrid
          dense
          result={pane.data}
          markRow={pane.markRow}
          markCell={pane.markCell}
          onEditCell={take}
          viewportRef={viewportRef}
        />
      </div>
    </div>
  )
}

/**
 * The ticks, between the two sides. It is `strip`, so it draws neither a row gutter nor scrollbars
 * of its own — the panes either side carry both, and it is exactly as wide as its one column.
 */
function TickStrip({
  pane,
  viewportRef,
  onTake,
}: {
  pane: CompareGrid
  viewportRef: RefObject<HTMLDivElement | null>
  onTake: (picks: string[]) => void
}) {
  return (
    <div
      className="flex h-full min-h-0 shrink-0 flex-col border-r border-line"
      style={{ width: TAKE_WIDTH }}
    >
      {/* The blank where each side has its name: the strip belongs to neither, and its own header
          says what it is one line below. */}
      <div className="h-6 shrink-0 border-b border-line bg-bar" />

      <div className="min-h-0 flex-1">
        <ResultGrid
          dense
          strip
          result={pane.data}
          markRow={pane.markRow}
          onEditCell={(ref: CellRef) => onTake(pane.picksAt(ref.row))}
          viewportRef={viewportRef}
        />
      </div>
    </div>
  )
}

/**
 * Moves several scrollers together. Whichever is scrolled writes its position onto the others, and
 * **already being there is what stops the echo**: setting a scroll position raises a scroll event
 * of its own, so the others' handlers run next — find they already agree, and do nothing.
 *
 * That test, and not a flag, is deliberately the whole of it. A flag has to be released on a
 * timer, and a timer that is a frame never fires while the window is not drawing: the panes would
 * come apart the moment it was minimised and stay apart. Two positions being equal is a fact about
 * the elements and is true whether or not anything is being painted.
 *
 * It converges even where two cannot agree — a pane scrolled past what another can reach clamps,
 * the other writes the clamped position back, and the second comparison then matches.
 *
 * **Sideways is carried only between scrollers that have somewhere to go sideways.** The tick strip
 * holds one column and can never scroll across; without that test it would answer every pane's
 * scroll with its own nought and drag them all back to the first column.
 *
 * `deps` is whatever changes the elements underneath — the layout, and the rows being redrawn —
 * because the refs are filled by the grids and there is nothing else to watch them by.
 */
function useScrollSync(views: RefObject<HTMLDivElement | null>[], deps: unknown[]) {
  useEffect(() => {
    const scrollers = views.map((view) => view.current).filter((one) => one !== null)
    if (scrollers.length < 2) return

    const followers = scrollers.map((from) => {
      const follow = () => {
        for (const to of scrollers) {
          if (to === from) continue

          if (to.scrollTop !== from.scrollTop) to.scrollTop = from.scrollTop
          if (movesAcross(from) && movesAcross(to) && to.scrollLeft !== from.scrollLeft) {
            to.scrollLeft = from.scrollLeft
          }
        }
      }

      from.addEventListener('scroll', follow)
      return { from, follow }
    })

    return () => {
      for (const { from, follow } of followers) from.removeEventListener('scroll', follow)
    }
    // The refs themselves are stable; what is watched is whatever replaced the elements they
    // point at, which is what `deps` names.
  }, deps)
}

const movesAcross = (view: HTMLElement) => view.scrollWidth > view.clientWidth
