import { useCallback, useEffect, useMemo, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'
import type { QueryResult } from '@/bridge/query'
import { tables, type Cell } from '@/bridge/tables'

import { cellKey, primaryKeyOf, type CellRef } from './resultSet'

interface PendingEdit {
  ref: CellRef
  value: Cell
}

/**
 * Holds cell edits locally until Apply writes them back, so a mistyped value costs nothing
 * and one round trip per statement is avoided. Discard drops them; a fresh result clears them.
 */
export function useResultEdits(
  result: QueryResult | null,
  sessionId: string | null,
  onApplied: () => void,
) {
  const [edits, setEdits] = useState<Record<string, PendingEdit>>({})
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    setEdits({})
    setError(null)
  }, [result])

  const rows = useMemo(() => overlay(result?.rows ?? [], edits), [result, edits])
  const dirty = useMemo(() => new Set(Object.keys(edits)), [edits])

  const setCell = useCallback(
    (ref: CellRef, value: Cell) => {
      const original = result?.rows[ref.row]?.[ref.column] ?? null

      setEdits((current) => {
        const next = { ...current }
        const key = cellKey(ref.row, ref.column)

        if (value === original) delete next[key]
        else next[key] = { ref, value }

        return next
      })
    },
    [result],
  )

  const discard = useCallback(() => {
    setEdits({})
    setError(null)
  }, [])

  const apply = useCallback(async () => {
    if (result === null || sessionId === null) return

    setSaving(true)
    try {
      for (const edit of Object.values(edits)) {
        const row = result.rows[edit.ref.row]
        const column = result.columns[edit.ref.column]
        if (!row || !column) continue

        await tables.updateCell(sessionId, {
          database: result.database,
          table: result.table,
          column: column.name,
          value: edit.value,
          key: primaryKeyOf(result.columns, row),
        })
      }

      setEdits({})
      setError(null)
      onApplied()
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }, [edits, result, sessionId, onApplied])

  return { rows, dirty, saving, error, setCell, discard, apply }
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
