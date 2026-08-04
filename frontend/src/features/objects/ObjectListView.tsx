import { useState } from 'react'

import { engineWords, type Namespace } from '@/bridge/connections'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import { focusOf, type Selection } from '@/features/results/resultSet'
import { tableActions } from '@/features/schema/tableActions'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore } from '@/stores/tabs'

import { useTableList } from './useTableList'

interface ObjectListViewProps {
  sessionId: string
  profileId: string
  namespace: Namespace
}

/**
 * Every table of a namespace with what the catalogue keeps about it — the pane the Tables folder
 * opens. It is a `ResultGrid`, read-only, and the toolbar over it acts on the table the selection
 * names: the first column, which is the one column every engine's list promises.
 *
 * What a table is *called* is the engine's, so a document store's list says Collection throughout —
 * see `engineWords`. Only the wording changes; every button here runs the same bound method.
 */
export function ObjectListView({ sessionId, profileId, namespace }: ObjectListViewProps) {
  const { data, loading, error, refresh } = useTableList(sessionId, namespace)
  const [selection, setSelection] = useState<Selection | null>(null)

  const ask = useSchemaStore((s) => s.ask)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const openTable = useTabsStore((s) => s.openTable)
  const openDesign = useTabsStore((s) => s.openDesign)

  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === profileId)?.engine ?? '',
  )
  const words = engineWords(engine)

  const nameAt = (row: number) => data?.rows[row]?.[0] ?? ''
  const focus = focusOf(selection)
  const table = focus ? nameAt(focus.row) : ''

  const on = tableActions(
    { profileId, namespace, table },
    {
      ask,
      // A change the tree re-reads leaves these numbers stale too, and they are what the reader
      // opened the list for — so the same change re-reads the list behind it.
      changeSchema: async (id, target, change) => {
        await changeSchema(id, target, change)
        setSelection(null)
        refresh()
      },
      openRows: openTable,
      design: openDesign,
    },
    words,
  )

  /** Two clicks open, as everywhere outside a writable grid: the row's table in a data tab. */
  const openRow = (row: number) => {
    const name = nameAt(row)
    if (name !== '') openTable({ profileId, namespace, table: name })
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button variant="subtle" icon={ICON.open} disabled={table === ''} onClick={on.open}>
          Open {words.table}
        </Button>
        <Button variant="subtle" icon={ICON.design} disabled={table === ''} onClick={on.design}>
          Design {words.table}
        </Button>
        <Button variant="subtle" icon={ICON.add} onClick={on.create}>
          New {words.table}
        </Button>
        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={table === ''}
          onClick={on.remove}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete {words.table}
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        {data === null ? (
          error ? (
            <Empty title={error.message} hint={error.detail} />
          ) : (
            <Loading label="Reading the catalogue…" />
          )
        ) : data.rows.length === 0 ? (
          <Empty
            title={`No ${words.tables.toLowerCase()} here yet`}
            hint={`New ${words.table} opens the designer on one this namespace does not have.`}
          />
        ) : (
          <ResultGrid
            result={data}
            selection={selection}
            onSelectionChange={setSelection}
            onActivateRow={openRow}
            onRefresh={refresh}
          />
        )}
      </div>

      {/* A failed re-read leaves the list already drawn where it is and says so under it, which
          is what the table view does with the same failure. */}
      {data !== null && error !== null ? (
        <p className="shrink-0 truncate border-t border-danger/40 bg-danger-subtle px-2 py-1 text-xs text-danger">
          {error.message}
        </p>
      ) : null}

      <GridFooter editable={false} busy={loading} onRefresh={refresh}>
        <span>
          {data === null ? '' : `${data.rows.length} ${words.tables.toLowerCase()}`}
        </span>
      </GridFooter>
    </div>
  )
}
