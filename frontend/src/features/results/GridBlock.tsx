import { DENSE_HEADER_HEIGHT, DENSE_ROW_HEIGHT } from '@/shared/theme/density'

import { ResultGrid, type ResultGridProps } from './ResultGrid'

/**
 * A grid at **its own rows' height**, for a panel that stacks several of them under headings.
 *
 * Every other grid in the app fills the panel a splitter gave it and scrolls inside it. A panel of
 * *facts* is the other shape: five rows under one heading, six under the next, and the whole column
 * scrolling as one. A grid given no height of its own would collapse there, and a grid given a
 * guessed one would either clip its last row or leave a gap under it — so the height is the sum it
 * actually needs, and the scrolling stays the panel's.
 *
 * It is `dense` because that is what these panels are for — a pane under the rows is read by
 * scanning it — and it is a wrapper rather than a flag on the grid so that nothing about a grid's
 * own scroller changes to suit a panel.
 */
export function GridBlock(props: ResultGridProps) {
  return (
    <div
      // A flex column, so the grid's own `flex-1` fits it to exactly this height: a grid left to
      // size itself would grow by the height of a horizontal scrollbar and spill out of the stack.
      className="flex shrink-0 flex-col"
      style={{ height: DENSE_HEADER_HEIGHT + props.result.rows.length * DENSE_ROW_HEIGHT }}
    >
      <ResultGrid {...props} dense />
    </div>
  )
}
