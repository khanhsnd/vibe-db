import { useCallback, useEffect, useMemo, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'
import { tables, type Cell, type RowValue, type TableColumn } from '@/bridge/tables'

import { cellKey, primaryKeyOf, type CellRef } from './resultSet'

const NO_ROWS: Cell[][] = []
const NO_DRAFTS: Cell[][] = []
const NO_DELETIONS: ReadonlySet<number> = new Set()

/** The stored table a grid's rows came from, and so what a write names. */
export interface EditTarget {
  database: string
  schema: string
  table: string
}

/** Always the *real* columns and rows, never a picker's visible subset: a write names columns. */
export interface EditSource {
  target: EditTarget
  columns: TableColumn[]
  rows: Cell[][]
}

interface PendingEdit {
  ref: CellRef
  value: Cell
}

/**
 * Holds every change to a grid locally until Apply writes it back: edited cells, rows added
 * at the end, and stored rows marked for deletion. A mistyped value therefore costs nothing,
 * and one round trip per keystroke is avoided. Discard drops them all; so does a fresh read,
 * because the row numbers a pending change names mean nothing in the next one.
 */
export function useGridEdits(
  source: EditSource | null,
  sessionId: string | null,
  onApplied: () => void,
) {
  const [edits, setEdits] = useState<Record<string, PendingEdit>>({})
  const [drafts, setDrafts] = useState<Cell[][]>(NO_DRAFTS)
  const [deleted, setDeleted] = useState<ReadonlySet<number>>(NO_DELETIONS)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const base = source?.rows ?? NO_ROWS
  const width = source?.columns.length ?? 0

  const discard = useCallback(() => {
    setEdits({})
    setDrafts(NO_DRAFTS)
    setDeleted(NO_DELETIONS)
    setError(null)
  }, [])

  // A pending change names a row number, which means nothing in the next read.
  useEffect(discard, [discard, base])

  // One array per set of rows, not per render: the grid draws every row from it, and a new
  // identity on every render would redraw the whole viewport under the reader.
  const rows = useMemo(() => [...overlay(base, edits), ...drafts], [base, edits, drafts])

  const dirtyKeys = useMemo(() => new Set(Object.keys(edits)), [edits])

  /** Every cell of a row that does not exist on the server yet counts as pending. */
  const isDirty = (row: number, column: number) =>
    row >= base.length || dirtyKeys.has(cellKey(row, column))

  const pending = dirtyKeys.size + drafts.length + deleted.size > 0

  const setCell = (ref: CellRef, value: Cell) => {
    const draft = ref.row - base.length
    if (draft >= 0) {
      setDrafts((current) =>
        current.map((row, index) => (index === draft ? replace(row, ref.column, value) : row)),
      )
      return
    }

    const original = base[ref.row]?.[ref.column] ?? null

    setEdits((current) => {
      const next = { ...current }
      const key = cellKey(ref.row, ref.column)

      if (value === original) delete next[key]
      else next[key] = { ref, value }

      return next
    })
  }

  /**
   * Appends a row, empty unless the surface can fill something in for the reader: `seed` is cells by
   * column index, and it exists because on some engines the reader's first act would otherwise be to
   * retype what they were already pointing at — a Redis key is added *beside* another one, so the
   * key it was added from is the name the box opens with.
   */
  const addRow = (seed?: Record<number, Cell>) =>
    setDrafts((current) => [
      ...current,
      Array.from({ length: width }, (_, at) => seed?.[at] ?? null),
    ])

  /**
   * Marks the given stored rows for deletion, and drops outright a row that was only ever a
   * draft — there is nothing on the server to delete. Rows rather than a range, because a
   * selection made with `Ctrl` is not one span.
   */
  const deleteRows = (rows: number[]) => {
    if (rows.length === 0) return

    const marked = new Set(deleted)
    const dropped = new Set<number>()

    for (const row of rows) {
      const draft = row - base.length
      if (draft >= 0) dropped.add(draft)
      else marked.add(row)
    }

    setDeleted(marked)
    if (dropped.size > 0) {
      setDrafts((current) => current.filter((_, index) => !dropped.has(index)))
    }
  }

  /** Edits first, then inserts, then deletions, so an edit to a doomed row is simply skipped. */
  const apply = async () => {
    if (source === null || sessionId === null) return
    const { target, columns } = source

    setSaving(true)
    try {
      for (const edit of renamesLast(columns, Object.values(edits))) {
        const row = base[edit.ref.row]
        const column = columns[edit.ref.column]
        if (!row || !column || deleted.has(edit.ref.row)) continue

        await tables.updateCell(sessionId, {
          ...target,
          column: column.name,
          value: edit.value,
          key: primaryKeyOf(columns, row),
        })
      }

      for (const draft of drafts) {
        await tables.insertRow(sessionId, { ...target, values: filledValues(columns, draft) })
      }

      for (const index of deleted) {
        const row = base[index]
        if (row) {
          await tables.deleteRow(sessionId, { ...target, key: primaryKeyOf(columns, row) })
        }
      }

      discard()
      onApplied()
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  return {
    rows,
    deleted,
    pending,
    saving,
    error,
    isDirty,
    setCell,
    addRow,
    deleteRows,
    discard,
    apply,
  }
}

/**
 * The same edits with every change to a **primary key** at the end. Each write names its row by the
 * key that row had when it was read, so a rename that went first would leave the edits after it
 * addressed to a row that is no longer there — and the engine that makes this ordinary rather than
 * exotic is the one whose primary key *is* the value the reader types: renaming a Redis key and
 * changing what it holds is one press of Apply. The sort is stable, so nothing else moves.
 */
function renamesLast(columns: TableColumn[], edits: PendingEdit[]): PendingEdit[] {
  const renames = (edit: PendingEdit) =>
    columns[edit.ref.column]?.primaryKey === true ? 1 : 0

  return [...edits].sort((a, b) => renames(a) - renames(b))
}

function replace(row: Cell[], column: number, value: Cell): Cell[] {
  return row.map((cell, index) => (index === column ? value : cell))
}

function overlay(rows: Cell[][], edits: Record<string, PendingEdit>): Cell[][] {
  const pending = Object.values(edits)
  if (pending.length === 0) return rows

  const next = rows.map((row) => [...row])
  for (const edit of pending) {
    const row = next[edit.ref.row]
    if (row) row[edit.ref.column] = edit.value
  }
  return next
}

/**
 * Only the cells actually filled in travel with an insert, so a column left alone takes the
 * server's own default rather than an explicit NULL.
 */
function filledValues(columns: TableColumn[], row: Cell[]): RowValue[] {
  return columns.flatMap((column, index) => {
    const value = row[index] ?? null
    return value === null ? [] : [{ column: column.name, value }]
  })
}
