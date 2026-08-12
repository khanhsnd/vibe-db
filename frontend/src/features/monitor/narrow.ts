import type { ProcessRef } from '@/bridge/monitor'

/**
 * What the box over a monitor narrows a list to: the rows holding the text in **any** of their cells.
 *
 * It is done here rather than by the server for the reason a key space's prefix tree is: the rows are
 * already in memory, all of them, and a monitor that asked the server again per keystroke would be
 * asking a busy server the very question the reader is watching it answer. `pg_settings` has four
 * hundred rows and `dm_os_performance_counters` some thousands, and neither is a paged read.
 *
 * The references travel with the rows, which is the whole point of this being one function: a row is
 * acted on by its reference, and a narrowed list whose references were still the unnarrowed ones would
 * kill the wrong process.
 */
export interface Narrowed {
  rows: (string | null)[][]
  refs: ProcessRef[]
}

export function narrow(
  rows: (string | null)[][],
  refs: ProcessRef[],
  find: string,
): Narrowed {
  const needle = find.trim().toLowerCase()
  if (needle === '') return { rows, refs }

  const kept: Narrowed = { rows: [], refs: [] }
  rows.forEach((row, at) => {
    if (!row.some((cell) => cell !== null && cell.toLowerCase().includes(needle))) return

    kept.rows.push(row)
    // A list with no references — either half of the status — narrows its rows and carries none.
    const ref = refs[at]
    if (ref) kept.refs.push(ref)
  })
  return kept
}
