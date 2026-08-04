import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { onRowsStreamed, tables, type Cell, type TablePage } from '@/bridge/tables'
import type { Filter } from '@/features/results/filter'
import type { SortKey } from '@/features/results/resultSet'

const NO_SORT = { column: '', descending: false }
const NO_FILTERS: Filter[] = []

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
export function useTableData(sessionId: string, namespace: Namespace, table: string) {
  const { database, schema } = namespace

  const [page, setPage] = useState(0)
  const [limit, setLimit] = useState<number>(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<SortKey | null>(null)
  const [filters, setFilters] = useState<Filter[]>(NO_FILTERS)
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
        sort: sort ?? NO_SORT,
        filters,
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
  }, [sessionId, database, schema, table, limit, page, sort, filters, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  const goToPage = useCallback((next: number) => setPage(Math.max(0, next)), [])

  const changeLimit = useCallback((next: number) => {
    setLimit(next)
    setPage(0)
  }, [])

  /** A new order makes the page number meaningless, so the reader goes back to the first. */
  const changeSort = useCallback((next: SortKey | null) => {
    setSort(next)
    setPage(0)
  }, [])

  /** A narrower table has fewer pages, so the reader goes back to the first of them. */
  const changeFilters = useCallback((next: Filter[]) => {
    setFilters(next)
    setPage(0)
  }, [])

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
    filters,
    goToPage,
    changeLimit,
    changeSort,
    changeFilters,
    refresh,
    stopFetch,
  }
}
