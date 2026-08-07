import { useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { tables, type Cell } from '@/bridge/tables'

import { columnAt, KEY_COLUMN, VALUE_COLUMN } from './keySpace'

/** What the panel is looking at while a whole value is on its way, and once it has arrived. */
export interface WholeValue {
  /** The whole value, or `null` while there is none to show and the sample stands. */
  value: Cell
  loading: boolean
  error: AppError | null
}

const NOTHING: WholeValue = { value: null, loading: false, error: null }

/**
 * The whole of one key's value, for the panel under a key space.
 *
 * The grid's own value is a **sample** on purpose: a page of a hundred keys must not be a million
 * members, which is why `redis.previewItems` exists at all. But the panel is looking at exactly one
 * key, and there is nothing there for the bound to protect — so where the page could only carry part
 * of a value, this asks for the rest of it.
 *
 * It is an ordinary row read with `whole` set, not a method of its own: the same call, the same
 * filter tree, narrowed to the one key by name. The driver answers a filter naming exactly one key by
 * **looking it up** rather than scanning for it, so this costs a round trip and not a walk of the key
 * space — see `redis.keyNames`.
 *
 * It asks for nothing at all where the value already is whole, which is nearly every key.
 */
export function useWholeValue(asked: {
  sessionId: string
  namespace: Namespace
  table: string
  /** The key to read, and the empty string where there is none picked. */
  key: string
  /** Whether the page could only carry part of that key's value. Nothing is read where it could. */
  sampled: boolean
  /**
   * When the page it came from was read. It is a **dependency and nothing else**: a re-read leaves
   * the key and the table alone, so without it a value fetched once would stand for ever — including
   * across the reader's own Apply, which is exactly when it has just changed.
   */
  at: number | null
}): WholeValue {
  const { sessionId, table, key, sampled, at } = asked
  const { database, schema } = asked.namespace
  const [held, setHeld] = useState<WholeValue>(NOTHING)

  useEffect(() => {
    if (!sampled || key === '') {
      setHeld(NOTHING)
      return
    }

    let active = true
    setHeld({ value: null, loading: true, error: null })

    tables
      .loadRows(sessionId, {
        runId: crypto.randomUUID(),
        database,
        schema,
        table,
        limit: 1,
        offset: 0,
        sort: [],
        filter: {
          kind: 'group',
          conjunction: 'and',
          children: [{ kind: 'condition', column: KEY_COLUMN, operator: 'eq', values: [key] }],
        },
        whole: true,
      })
      .then((page) => {
        if (!active) return

        const at = columnAt(page.columns, VALUE_COLUMN)
        // A key that has gone between the page and this read answers with no rows, and the sample
        // the reader is looking at is the truest thing left to show them.
        setHeld({ value: page.rows[0]?.[at] ?? null, loading: false, error: null })
      })
      .catch((thrown: unknown) => {
        if (active) setHeld({ value: null, loading: false, error: toAppError(thrown) })
      })

    return () => {
      active = false
    }
  }, [sessionId, database, schema, table, key, sampled, at])

  return held
}
