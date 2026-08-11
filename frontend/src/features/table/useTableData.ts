import { useCallback, useEffect, useMemo, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS } from '@/bridge/rows'
import { onRowsStreamed, tables, type Cell } from '@/bridge/tables'
import { activeFilter, type Group } from '@/features/results/filter'
import type { SortKey } from '@/features/results/resultSet'
import { useTableRowsStore } from '@/stores/tableRows'
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
 * **A page in hand is not read again.** What a read was asked for is spelled out as `request`, and a
 * tab holding a page that answers it reads nothing on the way back in — the rows the reader left,
 * their un-applied edits included, are the rows they return to. Only a *changed* request — another
 * page, another order, another filter — or **Refresh** sends for more.
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

  // What was asked for is the tab's and outlives it being looked away from; so is what came back —
  // see `stores/tableViews.ts` and `stores/tableRows.ts`.
  const view = useTableViewsStore((s) => s.views[tabId] ?? IDLE_VIEW)
  const change = useTableViewsStore((s) => s.change)
  const { page, limit, sort, criteria } = view

  const held = useTableRowsStore((s) => s.pages[tabId] ?? null)
  const hold = useTableRowsStore((s) => s.hold)
  const markStale = useTableRowsStore((s) => s.stale)

  const [nonce, setNonce] = useState(0)
  const [loading, setLoading] = useState(held === null)
  const [fetch, setFetch] = useState<RowFetch | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  /**
   * The whole of what this read asks for, so a page in hand can say whether it answers it. The
   * session is part of it: a connection dialled again is a different server as far as a page is
   * concerned, and rows read before it went down are not an answer about the one now open.
   */
  const request = useMemo(
    () => JSON.stringify({ sessionId, database, schema, table, limit, page, sort, criteria }),
    [sessionId, database, schema, table, limit, page, sort, criteria],
  )

  useEffect(() => {
    // Read through the store rather than through the render's own snapshot: this must not re-run
    // when the answer lands, only when the question changes or Refresh makes the answer stale.
    if (useTableRowsStore.getState().pages[tabId]?.request === request) {
      setLoading(false)
      return
    }

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
        hold(tabId, {
          request,
          data: streamed ? { ...next, rows: streaming } : next,
          loadedAt: Date.now(),
        })
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

        const stopped = useTableRowsStore.getState().pages[tabId]
        if (stopped === undefined) return

        hold(tabId, {
          request,
          data: { ...stopped.data, rows: streaming, total: streaming.length, statement: '' },
          loadedAt: Date.now(),
        })
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
  }, [sessionId, tabId, database, schema, table, limit, page, sort, criteria, request, hold, nonce])

  /**
   * Sends for the rows again. The page in hand stays on screen — a re-read must not blink — but it
   * answers nothing from now on, which is what makes the read above fetch rather than skip. The
   * nonce is what re-runs it, a request that has not changed being exactly the case Refresh is for.
   */
  const refresh = useCallback(() => {
    markStale(tabId)
    setNonce((current) => current + 1)
  }, [markStale, tabId])

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
    data: held?.data ?? null,
    loadedAt: held?.loadedAt ?? null,
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
