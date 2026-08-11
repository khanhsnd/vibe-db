import type { ReactNode } from 'react'

import { differs, type DiffStatus, type RowDiff } from '@/bridge/compare'
import type { Cell, TableColumn } from '@/bridge/tables'
import {
  CHECK_ON,
  type DiffMark,
  type GridData,
  type GridOutline,
  type ResultColumn,
} from '@/features/results/resultSet'

import { rowKey } from './migrate'
import type { StructureRow } from './structureTree'

/**
 * A grid of a comparison: the rows, what each of them is, and which of them are ticked to be
 * carried across. `take` is the one editable column of a read-only grid — see
 * `ResultColumn.readOnly`, which is what keeps a click on any other column from opening an editor.
 */
export interface CompareGrid {
  data: GridData
  markRow: (row: number) => DiffMark | undefined
  markCell: (row: number, column: number) => boolean
  /**
   * Every pick a click on the tick means. It is a *list* because a folder's tick stands for its
   * whole section — which is what makes "take all of these" the same control as "take this one"
   * rather than a fourth kind of button.
   */
  picksAt: (row: number) => string[]
}

/**
 * The grids a comparison is drawn as: the two sides, holding the same rows in the same order, and
 * the strip of ticks **between** them.
 *
 * The strip is a pane of its own and not a column of either side, for two reasons. A tick belongs
 * to *both* readings of the row and to neither table; and a column inside a pane slides out of
 * sight the moment that pane is scrolled sideways, which is exactly what a reader must never have
 * to chase. Stacked panes have no between, so there it is absent and each pane carries its own.
 */
export interface ComparePanes {
  left: CompareGrid
  middle?: CompareGrid
  right: CompareGrid
}

/** What the strip draws one row of: whether it differs at all, and what ticking it would take. */
interface Tick {
  status: DiffStatus
  picks: string[]
}

export interface DiffCounts {
  same: number
  changed: number
  added: number
  removed: number
}

export function countStatuses(statuses: readonly DiffStatus[]): DiffCounts {
  const counts: DiffCounts = { same: 0, changed: 0, added: 0, removed: 0 }
  for (const status of statuses) counts[status] += 1
  return counts
}

/** Whether a comparison found nothing at all to report, which is what an Empty pane says. */
export function allSame(counts: DiffCounts): boolean {
  return counts.changed + counts.added + counts.removed === 0
}

const TAKE = 'Take'

/**
 * How wide the strip of ticks is. It is exported because the strip is a grid of exactly this one
 * column and the frame around it has to be the same width — a frame any wider leaves a gap, and any
 * narrower puts a scrollbar under a column nobody can scroll.
 */
export const TAKE_WIDTH = 84
const NAME_WIDTH = 220
const DECLARATION_WIDTH = 260

/** The tick column, which is the only writable one of a comparison and the same on every grid. */
const takeColumn: ResultColumn = {
  name: TAKE,
  type: 'text',
  fullType: 'text',
  editor: { kind: 'check' },
  width: TAKE_WIDTH,
}

/** Whether every pick a row stands for is already taken, which is what its tick shows. */
function allTaken(picks: readonly string[], taken: ReadonlySet<string>): boolean {
  return picks.length > 0 && picks.every((pick) => taken.has(pick))
}

/**
 * The strip between the two panes: one column, one row per pair, and no chrome of its own. It
 * carries the row's own wash as well as its tick, so the colour runs across the whole row rather
 * than stopping at each pane's edge.
 */
function tickStrip(ticks: readonly Tick[], taken: ReadonlySet<string>): CompareGrid {
  return {
    data: {
      columns: [takeColumn],
      rows: ticks.map((tick): Cell[] => [allTaken(tick.picks, taken) ? CHECK_ON : '']),
    },
    markRow: (row) => {
      const status = ticks[row]?.status
      return status === undefined || status === 'same' ? undefined : status
    },
    markCell: () => false,
    picksAt: (row) => ticks[row]?.picks ?? [],
  }
}

const readOnly = (name: string, width?: number, outline?: GridOutline): ResultColumn => ({
  name,
  type: 'text',
  fullType: 'text',
  readOnly: true,
  width,
  outline,
})

/**
 * A table's whole shape as **two trees, one per side**, built from one list of rows: the table, its
 * sections folded or open, and each thing under the one it belongs to. Row *n* of one is row *n* of
 * the other by construction, so the pair reads across and folds together — the same rule the data
 * half's two panes are held up by.
 *
 * The ticks are the **strip between them** and belong to neither tree, which is what keeps them
 * where the reader is looking however far either side is scrolled.
 *
 * The hierarchy is the Name column's `outline` — each grid is still handed a flat list, in the
 * order it reads, and neither of them holds a shape or decides what is open.
 */
export function structurePanes(
  rows: readonly StructureRow[],
  taken: ReadonlySet<string>,
  onToggle: (branch: string) => void,
  glyph: (row: StructureRow) => ReactNode,
): ComparePanes {
  const outline: GridOutline = {
    depth: (row) => rows[row]?.depth ?? 0,
    open: (row) => rows[row]?.open,
    onToggle: (row) => {
      const branch = rows[row]?.branch
      if (branch !== undefined) onToggle(branch)
    },
    icon: (row) => {
      const one = rows[row]
      return one ? glyph(one) : null
    },
  }

  const paneOf = (side: 'left' | 'right'): CompareGrid => ({
    data: {
      columns: [
        readOnly('Name', NAME_WIDTH, outline),
        readOnly('Declaration', DECLARATION_WIDTH),
        // What moved is said once, beside the side being written by default. Twice would be the
        // same sentence read against itself.
        ...(side === 'right' ? [readOnly('Difference')] : []),
      ],
      rows: rows.map((one): Cell[] => [
        one[side].name,
        one[side].declaration,
        ...(side === 'right' ? [one.detail] : []),
      ]),
    },
    markRow: (row) => {
      const one = rows[row]
      return one ? markFor(side, one.status) : undefined
    },
    // The declaration is what the reader is looking for on a row that differs: the point of the
    // row is that this side's reading of the thing and the other's do not agree.
    markCell: (row, column) => column === 1 && differs(rows[row]?.status ?? 'same'),
    picksAt: (row) => rows[row]?.picks ?? [],
  })

  const ticks = rows.map((one): Tick => ({ status: one.status, picks: one.picks }))

  return {
    left: paneOf('left'),
    middle: tickStrip(ticks, taken),
    right: paneOf('right'),
  }
}

/**
 * The rows of a data comparison as two panes, and the same strip of ticks between them.
 *
 * **Stacked, there is no between**: one pane sits above the other, so each carries the tick as its
 * own first column and it is still the same tick — a row is taken or it is not, and which pane the
 * reader reached for is not a second question.
 */
export function dataPanes(
  columns: TableColumn[],
  pairs: readonly RowDiff[],
  at: readonly number[],
  taken: ReadonlySet<string>,
  stacked: boolean,
): ComparePanes {
  const ticks = pairs.map(
    (pair, row): Tick => ({
      status: pair.status,
      picks: differs(pair.status) ? [rowKey(pair, at[row] ?? row)] : [],
    }),
  )

  const shown: ResultColumn[] = [
    ...(stacked ? [takeColumn] : []),
    ...columns.map((one) => ({
      name: one.name,
      type: one.type,
      fullType: one.fullType,
      primaryKey: one.primaryKey,
      readOnly: true,
    })),
  ]
  const blank: Cell[] = columns.map(() => '')
  // Where the panes carry the tick themselves, a value sits one column further along than the row
  // holds it — the tick is the grid's own and not one of the table's.
  const offset = stacked ? 1 : 0

  const paneOf = (side: 'left' | 'right'): CompareGrid => ({
    data: {
      columns: shown,
      rows: pairs.map((pair, row) => [
        ...(stacked ? [allTaken(ticks[row]?.picks ?? [], taken) ? CHECK_ON : ''] : []),
        ...(pair[side] ?? blank),
      ]),
    },
    markRow: (row) => {
      const pair = pairs[row]
      return pair ? markFor(side, pair.status) : undefined
    },
    markCell: (row, column) =>
      column >= offset && (pairs[row]?.differs[column - offset] ?? false),
    picksAt: (row) => ticks[row]?.picks ?? [],
  })

  return {
    left: paneOf('left'),
    ...(stacked ? {} : { middle: tickStrip(ticks, taken) }),
    right: paneOf('right'),
  }
}

/**
 * What one side draws for a row of the data half. A side that holds the row draws the difference;
 * the side that does not draws `absent`, which is neither a value nor an empty one.
 *
 * Which pairs are drawn at all is not here but in `fieldFilter.shownAt` — one place for every
 * narrowing of the panes, so the differences and the fields cannot answer that question twice.
 */
function markFor(side: 'left' | 'right', status: DiffStatus): DiffMark | undefined {
  switch (status) {
    case 'same':
      return undefined
    case 'changed':
      return 'changed'
    case 'added':
      return side === 'right' ? 'added' : 'absent'
    default:
      return side === 'left' ? 'removed' : 'absent'
  }
}

