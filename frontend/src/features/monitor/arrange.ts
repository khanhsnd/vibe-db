import { filteredRows } from '@/features/results/filterRows'
import { sortedOrder, type Cell, type ResultColumn, type SortKey } from '@/features/results/resultSet'
import type { Group } from '@/features/results/filter'

/**
 * What a monitor's pane actually draws: the rows a server answered with, narrowed and ordered.
 *
 * Three things narrow or reorder them and they are **one function** because of the references. A row
 * is acted on by the reference beside it — a process is ended by one, a slow statement is taken away
 * by one — so a list narrowed without its references carried along would end the wrong process. That
 * is the whole reason this is not three calls at the call site.
 *
 * All three happen **here rather than on the server**, which is the pane's own rule: the rows are
 * already in memory, all of them, and a monitor that asked again per keystroke would be asking a busy
 * server the very question the reader is watching it answer. `pg_settings` has four hundred rows and
 * `dm_os_performance_counters` some thousands, neither is a paged read, and one of the engines
 * (`SLOWLOG GET`) will not take a condition at all — so doing it here is what keeps six engines'
 * panes answering alike.
 *
 * The order is: **find, then filter, then sort.** The first two are both narrowings and commute, so
 * which comes first is arbitrary and the cheaper one leads. Sorting comes last because sorting rows
 * that are about to be thrown away is work nobody sees.
 */
export interface Arranged<Ref> {
  rows: Cell[][]
  refs: Ref[]
}

export function arrange<Ref>(
  rows: Cell[][],
  refs: Ref[],
  columns: ResultColumn[],
  find: string,
  criteria: Group,
  sort: readonly SortKey[],
): Arranged<Ref> {
  const needle = find.trim().toLowerCase()

  // The box narrows to rows holding the text in **any** cell, which is what a reader typing into an
  // unlabelled box means — they are looking for a word, not for a column.
  const found =
    needle === ''
      ? rows.map((_, index) => index)
      : rows
          .map((_, index) => index)
          .filter((index) =>
            (rows[index] ?? []).some(
              (cell) => cell !== null && cell.toLowerCase().includes(needle),
            ),
          )

  const kept = new Set(filteredRows(rows, columns, criteria))
  const narrowed = found.filter((index) => kept.has(index))

  // `sortedOrder` answers in the coordinates of the rows it was handed, so it is given the narrowed
  // rows and its answer is read back through them. Handing it the whole list and sorting the indexes
  // would have sorted rows that are not on screen.
  const shown = narrowed.map((index) => rows[index] ?? [])
  const order = sortedOrder(shown, columns, sort)

  return {
    rows: order.map((at) => shown[at] ?? []),
    refs: order.flatMap((at) => {
      const ref = refs[narrowed[at] ?? -1]
      // A list with no references — either half of the status — narrows its rows and carries none.
      return ref === undefined ? [] : [ref]
    }),
  }
}
