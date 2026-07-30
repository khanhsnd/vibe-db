import { useEffect, useRef } from 'react'

import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { IDLE_RUN, useQueriesStore } from '@/stores/queries'
import { useTabsStore, type QueryTab } from '@/stores/tabs'

import { EditorPane } from './EditorPane'
import { QueryOutput } from './QueryOutput'
import { QueryToolbar } from './QueryToolbar'

interface QueryWorkspaceProps {
  tab: QueryTab
  sessionId: string | null
  databases: string[]
}

export function QueryWorkspace({ tab, sessionId, databases }: QueryWorkspaceProps) {
  const run = useQueriesStore((s) => s.runs[tab.id] ?? IDLE_RUN)
  const execute = useQueriesStore((s) => s.execute)
  const cancel = useQueriesStore((s) => s.cancel)
  const editQuery = useTabsStore((s) => s.editQuery)

  const canRun = sessionId !== null && tab.sql.trim() !== ''

  const start = (explain: boolean) => {
    if (!canRun || sessionId === null) return
    void execute(tab.id, sessionId, {
      database: tab.database,
      sql: tab.sql,
      maxRows: tab.rowLimit,
      explain,
    })
  }

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
          <EditorPane value={tab.sql} onChange={(sql) => editQuery(tab.id, { sql })} />
        </SplitPanel>

        <SplitHandle orientation="vertical" />

        <SplitPanel id="output" defaultSize="62%" minSize="20%">
          <QueryOutput run={run} />
        </SplitPanel>
      </Split>
    </div>
  )
}
