import { ALL_ROWS } from '@/bridge/rows'

/**
 * Where a page begins and how many there are, in one place: the pager walks them, the status line
 * reports them, and the two surfaces cut them out of different things — but all four must agree,
 * or the arrows and the words under them say different numbers.
 *
 * `All` collapses the walk into a single page, so its step is the whole count rather than a size.
 */
export const pageStep = (size: number, total: number) =>
  size === ALL_ROWS ? Math.max(total, 1) : size

export const pageCount = (size: number, total: number) =>
  Math.max(1, Math.ceil(total / pageStep(size, total)))
