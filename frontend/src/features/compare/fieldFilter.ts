import { differs, type RowDiff } from '@/bridge/compare'
import type { Cell, TableColumn } from '@/bridge/tables'

/**
 * What the reader is looking for in each compared field, keyed by the field's **own name** rather
 * than by its position, so a narrowing survives a re-read.
 *
 * A needle is a **substring, matched case-insensitively** — the one notation this app has for "holds
 * this text", the same as the sidebar's filter box and a database filter. There is no operator to
 * pick and no pattern to write: a comparison is read to find the row that differs, and a box per
 * field is the shortest way from a table nobody can scan to the rows that were the question.
 *
 * It narrows **what is drawn** and asks no server anything. Both sides are already in memory and
 * were paired there; a read that narrowed them would be two fresh windows, and row *n* of one pane
 * would no longer be row *n* of the other.
 */
export type FieldFilters = Readonly<Record<string, string>>

/** One narrowing in force: which compared column, and the text a value has to hold. */
interface Needle {
  at: number
  text: string
}

/** How many fields are narrowing the panes, which is what the button says while it is shut. */
export function filterCount(filters: FieldFilters): number {
  return Object.values(filters).filter((text) => text.trim() !== '').length
}

/**
 * Which pairs the panes draw, and **where each of them sits in the whole comparison**. The position
 * travels because it is what a tick is named by: a row the filters hid must be the same row again
 * when they are widened.
 */
export function shownAt(
  pairs: readonly RowDiff[],
  columns: readonly TableColumn[],
  differencesOnly: boolean,
  filters: FieldFilters,
): number[] {
  const needles = needlesOf(columns, filters)

  return pairs.flatMap((pair, at) =>
    (!differencesOnly || differs(pair.status)) && holds(pair, needles) ? [at] : [],
  )
}

/** The columns a name was typed against, found once rather than per row. */
function needlesOf(columns: readonly TableColumn[], filters: FieldFilters): Needle[] {
  return columns.flatMap((column, at) => {
    const text = (filters[column.name] ?? '').trim().toLowerCase()
    return text === '' ? [] : [{ at, text }]
  })
}

/**
 * Whether a pair is drawn: every needle has to be held by **one of the two sides**. A row only the
 * left holds has no right side at all, and the side that has not got it must never be the reason it
 * stops being drawn — a blank opposite a row is absent and not empty, which is the difference the
 * reader opened the tab to see.
 */
function holds(pair: RowDiff, needles: readonly Needle[]): boolean {
  return needles.every((needle) => held(pair.left, needle) || held(pair.right, needle))
}

function held(row: Cell[] | null, needle: Needle): boolean {
  // A needle is never blank, so a value the row has not got matches nothing rather than everything.
  const value = row?.[needle.at] ?? ''
  return value.toLowerCase().includes(needle.text)
}
