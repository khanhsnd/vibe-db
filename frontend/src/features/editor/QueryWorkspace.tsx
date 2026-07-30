import { useCallback, useEffect, useRef } from 'react'

import type { TableSchema } from '@/bridge/connections'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { databaseKey, useConnectionsStore } from '@/stores/connections'
import { IDLE_RUN, useQueriesStore } from '@/stores/queries'
import { useTabsStore, type QueryTab } from '@/stores/tabs'

import { EditorPane } from './EditorPane'
import { QueryOutput } from './QueryOutput'
import { QueryToolbar } from './QueryToolbar'

const NO_SCHEMA: TableSchema[] = []

interface QueryWorkspaceProps {
  tab: QueryTab
  sessionId: string | null
  databases: string[]
}

export function QueryWorkspace({ tab, sessionId, databases }: QueryWorkspaceProps) {
  const run = useQueriesStore((s) => s.runs[tab.id] ?? IDLE_RUN)
  const execute = useQueriesStore((s) => s.execute)
  const rerun = useQueriesStore((s) => s.rerun)
  const cancel = useQueriesStore((s) => s.cancel)
  const editQuery = useTabsStore((s) => s.editQuery)

  const schema = useConnectionsStore(
    (s) => s.schemas[databaseKey(tab.profileId, tab.database)] ?? NO_SCHEMA,
  )
  const loadSchema = useConnectionsStore((s) => s.loadSchema)

  useEffect(() => {
    if (tab.database !== '') void loadSchema(tab.profileId, tab.database)
  }, [loadSchema, tab.profileId, tab.database])

  // A selection narrows what runs, the way every other client works. It is a ref because
  // dragging over the text would otherwise re-render the whole workspace.
  const selection = useRef('')

  const canRun = sessionId !== null && tab.sql.trim() !== ''

  const start = (explain: boolean) => {
    if (!canRun || sessionId === null) return
    void execute(tab.id, sessionId, {
      database: tab.database,
      sql: selection.current.trim() || tab.sql,
      maxRows: tab.rowLimit,
      explain,
    })
  }

  // Refresh replays the statement that produced the rows on screen, not whatever the
  // editor holds now, so applying an edit cannot silently run something else.
  const refresh = useCallback(() => {
    if (sessionId !== null) void rerun(tab.id, sessionId)
  }, [rerun, tab.id, sessionId])

  // The shortcut is bound on the window, because F5 reloads the WebView whenever
  // focus sits outside this panel. The ref keeps the listener off the keystroke path.
  const latest = useRef(start)
  latest.current = start

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const shortcut =
        event.key === 'F5' || (event.key === 'Enter' && (event.ctrlKey || event.metaKey))
      if (!shortcut) return

      event.preventDefault()
      latest.current(false)
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QueryToolbar
        databases={databases}
        database={tab.database}
        rowLimit={tab.rowLimit}
        running={run.running}
        canRun={canRun}
        onDatabaseChange={(database) => editQuery(tab.id, { database })}
        onRowLimitChange={(rowLimit) => editQuery(tab.id, { rowLimit })}
        onExecute={() => start(false)}
        onExplain={() => start(true)}
        onStop={() => cancel(tab.id)}
      />

      <Split
        id="vibe-db.workarea"
        orientation="vertical"
        panelIds={['editor', 'output']}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="editor" defaultSize="38%" minSize="15%">
          <EditorPane
            value={tab.sql}
            schema={schema}
            onChange={(sql) => editQuery(tab.id, { sql })}
            onSelectionChange={(selected) => {
              selection.current = selected
            }}
          />
        </SplitPanel>

        <SplitHandle orientation="vertical" />

        <SplitPanel id="output" defaultSize="62%" minSize="20%">
          <QueryOutput run={run} sessionId={sessionId} onRefresh={refresh} />
        </SplitPanel>
      </Split>
    </div>
  )
}
