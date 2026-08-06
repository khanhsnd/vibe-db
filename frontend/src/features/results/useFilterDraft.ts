import { useEffect, useState } from 'react'

import { appendTo, newCondition, NO_FILTER, type GridFiltering, type Group } from './filter'
import type { SortKey } from './resultSet'

/**
 * The conditions and the sort keys as the reader is building them, before `Apply Filter & Sort`
 * hands them over.
 *
 * It is the **grid's** and not the panel's, because the panel is taken down when it is hidden: a
 * reader who removed a condition and then closed the panel has not put it back, and a header's
 * `Add Filter` has somewhere to land whether the panel was open or not.
 */
export interface FilterDraft {
  criteria: Group
  setCriteria: (next: Group) => void
  keys: readonly SortKey[]
  setKeys: (next: readonly SortKey[]) => void
  /** Whether this grid can be ordered at all — a surface that passed no `onSortChange` cannot. */
  canSort: boolean
  /** Whether the draft differs from what is in force, which is what `Criteria Edited` says. */
  edited: boolean
  /** Hands the draft over, sending only the half of it that changed. */
  apply: () => void
  /** A fresh condition on one field at the end of the root — what a header's `Add Filter` adds. */
  addCondition: (column: string) => void
}

export function useFilterDraft(
  filtering: GridFiltering | undefined,
  sort: readonly SortKey[],
  onSortChange: ((sort: SortKey[]) => void) | undefined,
): FilterDraft {
  const inForce = filtering?.criteria ?? NO_FILTER
  const [criteria, setCriteria] = useState(inForce)
  const [keys, setKeys] = useState(sort)

  // The draft follows what is in force **as that changes**, so a condition added from the cell menu
  // or a sort taken off a header appears in the builder rather than being overwritten by the draft
  // beside it.
  useEffect(() => setCriteria(inForce), [inForce])
  useEffect(() => setKeys(sort), [sort])

  return {
    criteria,
    setCriteria,
    keys,
    setKeys,
    canSort: onSortChange !== undefined,
    edited: criteria !== inForce || keys !== sort,
    /** Nothing is sent that has not changed: Apply on an untouched panel is not a re-read. */
    apply: () => {
      if (criteria !== inForce) filtering?.onChange(criteria)
      if (keys !== sort) onSortChange?.([...keys])
    },
    addCondition: (column) =>
      setCriteria((current) => appendTo(current, [], newCondition(column))),
  }
}
