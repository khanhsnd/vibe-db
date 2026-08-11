import { create } from 'zustand'

import type { Cell } from '@/bridge/tables'
import type { CellRef } from '@/features/results/resultSet'

/** One cell staged against the rows a grid is showing. */
export interface PendingEdit {
  ref: CellRef
  value: Cell
  /** What the value is to be written *as*, where the engine's value carries a type — see `CellEdit`. */
  valueType?: string
}

/** Everything staged against one grid: edited cells by `cellKey`, rows added at the end, rows marked to go. */
export interface StagedEdits {
  cells: Record<string, PendingEdit>
  drafts: Cell[][]
  deleted: ReadonlySet<number>
}

/** A grid nobody has touched, held steady so selectors stay referentially equal. */
export const NOTHING_STAGED: StagedEdits = { cells: {}, drafts: [], deleted: new Set() }

interface GridEditsState {
  staged: Record<string, StagedEdits>
  stage: (id: string, edit: (current: StagedEdits) => StagedEdits) => void
  discard: (id: string) => void
}

/**
 * What is waiting to be written back to each grid, keyed by the tab it is drawn in.
 *
 * It is a store rather than `useGridEdits`' own state because **only the active tab is mounted**: a
 * row added and not yet applied would otherwise be undone by a glance at another tab. It is dropped
 * by a *new* read, because the row numbers a pending change names mean nothing in the next one, and
 * by `tabs.close` — see `stores/tableRows.ts`, which keeps the read those numbers belong to.
 */
export const useGridEditsStore = create<GridEditsState>((set) => ({
  staged: {},

  stage: (id, edit) =>
    set((s) => ({ staged: { ...s.staged, [id]: edit(s.staged[id] ?? NOTHING_STAGED) } })),

  discard: (id) =>
    set((s) => {
      const { [id]: gone, ...rest } = s.staged
      return gone === undefined ? s : { staged: rest }
    }),
}))
