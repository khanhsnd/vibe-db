import { Terminal } from 'lucide-react'

import { QueryWorkspace } from '@/features/editor/QueryWorkspace'
import { TableDataView } from '@/features/table/TableDataView'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { databaseKey, useConnectionsStore } from '@/stores/connections'
import { useTabsStore } from '@/stores/tabs'

import { TabBar } from './TabBar'

const NO_DATABASES: string[] = []

export function WorkArea({ onNewQuery }: { onNewQuery: () => void }) {
  const activeTab = useTabsStore((s) => s.tabs.find((t) => t.id === s.activeId) ?? null)
  const sessionId = useConnectionsStore((s) =>
    activeTab ? (s.sessions[activeTab.profileId]?.id ?? null) : null,
  )
  const databases = useConnectionsStore((s) =>
    activeTab ? (s.databases[activeTab.profileId] ?? NO_DATABASES) : NO_DATABASES,
  )
  const engine = useConnectionsStore((s) =>
    activeTab ? (s.profiles.find((p) => p.id === activeTab.profileId)?.engine ?? '') : '',
  )
  const schemas = useConnectionsStore((s) =>
    activeTab
      ? (s.schemas[databaseKey(activeTab.profileId, activeTab.namespace.database)] ?? NO_DATABASES)
      : NO_DATABASES,
  )

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TabBar onNewQuery={onNewQuery} />

      <div className="min-h-0 flex-1">
        {activeTab === null ? (
          <Empty
            icon={<Terminal size={26} strokeWidth={1.5} />}
            title="No open tabs"
            hint="Open a query editor, or pick a table from the sidebar."
            action={
              <Button variant="primary" onClick={onNewQuery}>
                New Query
              </Button>
            }
          />
        ) : activeTab.kind === 'table' ? (
          sessionId === null ? (
            <Empty title="Not connected" hint="Reconnect this connection to load the table." />
          ) : (
            <TableDataView
              key={activeTab.id}
              sessionId={sessionId}
              namespace={activeTab.namespace}
              table={activeTab.table}
            />
          )
        ) : (
          <QueryWorkspace
            key={activeTab.id}
            tab={activeTab}
            sessionId={sessionId}
            databases={databases}
            schemas={schemas}
            engine={engine}
          />
        )}
      </div>
    </div>
  )
}
