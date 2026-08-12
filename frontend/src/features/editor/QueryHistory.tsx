import { useMemo, useState } from 'react'

import type { HistoryEntry } from '@/bridge/history'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import {
  NO_SORT,
  oneLine,
  type Cell,
  type GridData,
  type SortKey,
} from '@/features/results/resultSet'
import { useGridOrder } from '@/features/results/useGridOrder'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { useSchemaStore } from '@/stores/schema'

import { useQueryHistory } from './useQueryHistory'

const NO_ENTRIES: HistoryEntry[] = []

interface QueryHistoryProps {
  profileId: string
  /** Two clicks put the statement back in the editor above — see `QueryWorkspace`. */
  onPick: (entry: HistoryEntry) => void
}

/**
 * The statements this connection has run, newest first — a `ResultGrid`, because it is rows and
 * columns. A **double-click loads a row's statement into the editor**, which is what the log is
 * for; the grid is read-only, so a click has nothing else to mean.
 *
 * It is sorted in memory by a header's own menu, there being no statement of its own to re-order.
 */
export function QueryHistory({ profileId, onPick }: QueryHistoryProps) {
  const { entries, loading, error, refresh, clear } = useQueryHistory(profileId)
  const [sort, setSort] = useState<readonly SortKey[]>(NO_SORT)
  const ask = useSchemaStore((s) => s.ask)

  const log = entries ?? NO_ENTRIES
  const data = useMemo(() => historyRows(log), [log])
  const order = useGridOrder(data.rows, data.columns, sort)
  const rows = useMemo(() => order.map((row) => data.rows[row] ?? []), [order, data.rows])

  /** The grid draws the ordered rows, so a row it reports is a place in that order. */
  const openRow = (row: number) => {
    const entry = log[order[row] ?? -1]
    if (entry) onPick(entry)
  }

  const confirmClear = () =>
    ask({
      kind: 'confirm',
      title: 'Clear History',
      message: 'Forget every statement this app has run? Nothing on any server changes.',
      submitLabel: 'Clear',
      danger: true,
      onSubmit: clear,
    })

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      {/* Clearing the log acts on the whole of it, so it sits over the rows rather than in the
          footer, which acts on a row. */}
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={log.length === 0}
          onClick={confirmClear}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Clear History
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        {entries === null ? (
          error ? (
            <Empty title={error.message} hint={error.detail} />
          ) : (
            <Loading label="Reading the history…" />
          )
        ) : log.length === 0 ? (
          <Empty
            title="Nothing has run here yet"
            hint="Every statement you execute on this connection is kept here, ready to run again."
          />
        ) : (
          <ResultGrid
            result={{ columns: data.columns, rows }}
            onActivateRow={openRow}
            sort={sort}
            onSortChange={setSort}
            onRefresh={refresh}
          />
        )}
      </div>

      {/* A failed read leaves whatever is drawn where it is and says so under it, exactly as the
          table list does with the same failure. */}
      {entries !== null && error !== null ? <ErrorStrip error={error} /> : null}

      <GridFooter editable={false} busy={loading} onRefresh={refresh}>
        <span>{log.length} statements</span>
      </GridFooter>
    </div>
  )
}

function historyRows(entries: HistoryEntry[]): GridData {
  return {
    columns: [
      column('When', 'timestamp'),
      column('Statement'),
      column('Database'),
      column('Runs', 'int'),
      column('Rows', 'int'),
      column('Time (ms)', 'int'),
      column('Message'),
    ],
    rows: entries.map((entry): Cell[] => [
      localStamp(entry.ranAt),
      oneLine(entry.sql),
      entry.schema === '' ? entry.database : `${entry.database}.${entry.schema}`,
      String(entry.runs),
      String(entry.rows),
      String(entry.elapsedMs),
      entry.error === '' ? 'OK' : `${entry.errorCode}: ${entry.error}`,
    ]),
  }
}

/**
 * When it ran, in the reader's own time zone and in a spelling that sorts as text — the grid orders
 * this column by comparing the cells it drew, and `8/4/26` would sort before `9/9/25`.
 */
function localStamp(ms: number): string {
  const at = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, '0')

  return (
    `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ` +
    `${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}`
  )
}

const column = (name: string, type = 'text') => ({ name, type, fullType: type })
