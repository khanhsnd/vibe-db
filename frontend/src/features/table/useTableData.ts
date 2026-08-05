import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS } from '@/bridge/rows'
import { onRowsStreamed, tables, type Cell, type TablePage } from '@/bridge/tables'
import { activeFilter, type Group } from '@/features/results/filter'
import type { SortKey } from '@/features/results/resultSet'
import { IDLE_VIEW, useTableViewsStore } from '@/stores/tableViews'

/** A read of every row, while it runs: what to stop, and how many rows have arrived so far. */
export interface RowFetch {
  runId: string
  fetched: number
}

/**
 * The rows are paged on the server, so the sort and the filters have to be too: ordering or
 * narrowing the page in hand would only shuffle the hundred rows the reader can already see.
 *
 * `All` is the one read that cannot be answered in one piece — a million rows would have to cross
 * the bridge as a single JavaScript string — so it is **streamed**: the rows arrive in batches
 * under a run id, this hook stitches them together, and `fetch` reports the count as they land so
 * the reader can watch it and stop it. Such a read has no deadline; stopping is what bounds it,
 * and what did arrive before the stop is kept.
 */
export function useTableData(
  sessionId: string,
  namespace: Namespace,
  table: string,
  tabId: string,
) {
  const { database, schema } = namespace

  // What was asked for is the tab's and outlives it being looked away from; what came back is this
  // hook's, and is read again on the way in — see `stores/tableViews.ts`.
  const view = useTableViewsStore((s) => s.views[tabId] ?? IDLE_VIEW)
  const change = useTableViewsStore((s) => s.change)
  const { page, limit, sort, criteria } = view

  const [nonce, setNonce] = useState(0)
  const [data, setData] = useState<TablePage | null>(null)
  const [loadedAt, setLoadedAt] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [fetch, setFetch] = useState<RowFetch | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)

    const runId = crypto.randomUUID()
    const streamed = limit === ALL_ROWS
    const streaming: Cell[][] = []

    const stopListening = streamed
      ? onRowsStreamed(runId, {
          rows: (batch) => {
            for (const row of batch) streaming.push(row)
          },
          counted: (fetched) => {
            if (active) setFetch({ runId, fetched })
          },
        })
      : null

    if (streamed) setFetch({ runId, fetched: 0 })

    tables
      .loadRows(sessionId, {
        runId,
        database,
        schema,
        table,
        limit,
        offset: limit === ALL_ROWS ? 0 : page * limit,
        sort: [...sort],
        // Only what actually narrows crosses the wire: an unticked condition and one still short
        // of a value are the builder's business, not the server's.
        filter: activeFilter(criteria),
      })
      .then((next) => {
        if (!active) return
        setData(streamed ? { ...next, rows: streaming } : next)
        setLoadedAt(Date.now())
        setError(null)
      })
      .catch((thrown: unknown) => {
        if (!active) return

        // Stopping is a choice rather than a failure, so the rows that did arrive are kept and
        // the total becomes what is actually held. The columns are the ones already on screen: a
        // read stopped before it answered named none, so a first read stopped shows nothing.
        //
        // The statement goes, though, rather than staying behind: only the answer carries it, so
        // what is left is the *previous* page's — `LIMIT 100 OFFSET 0` beside thirty thousand rows.
        // A read whose statement this side never learned says so instead of naming the wrong one.
        const failure = toAppError(thrown)
        if (failure.code !== 'QUERY_CANCELLED') {
          setError(failure)
          return
        }
        setData((current) =>
          current === null
            ? null
            : { ...current, rows: streaming, total: streaming.length, statement: '' },
        )
        setLoadedAt(Date.now())
      })
      .finally(() => {
        if (!active) return
        setLoading(false)
        setFetch(null)
      })

    return () => {
      active = false
      stopListening?.()
      // A read nobody is waiting for any more — the reader changed the limit, closed the tab —
      // must not go on scanning a million rows and emitting them at a listener that has gone.
      if (streamed) void tables.stopRows(runId)
    }
  }, [sessionId, database, schema, table, limit, page, sort, criteria, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  const goToPage = useCallback(
    (next: number) => change(tabId, { page: Math.max(0, next) }),
    [change, tabId],
  )

  const changeLimit = useCallback(
    (next: number) => change(tabId, { limit: next, page: 0 }),
    [change, tabId],
  )

  /** A new order makes the page number meaningless, so the reader goes back to the first. */
  const changeSort = useCallback(
    (next: SortKey[]) => change(tabId, { sort: next, page: 0 }),
    [change, tabId],
  )

  /** A narrower table has fewer pages, so the reader goes back to the first of them. */
  const changeCriteria = useCallback(
    (next: Group) => change(tabId, { criteria: next, page: 0 }),
    [change, tabId],
  )

  const stopFetch = useCallback(() => {
    if (fetch !== null) void tables.stopRows(fetch.runId)
  }, [fetch])

  return {
    data,
    loadedAt,
    loading,
    fetch,
    error,
    page,
    limit,
    sort,
    criteria,
    goToPage,
    changeLimit,
    changeSort,
    changeCriteria,
    refresh,
    stopFetch,
  }
}
