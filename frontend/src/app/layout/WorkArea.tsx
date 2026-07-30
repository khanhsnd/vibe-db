import { useState } from 'react'
import { Terminal } from 'lucide-react'

import { EditorPane } from '@/features/editor/EditorPane'
import { QueryActionBar } from '@/features/editor/QueryActionBar'
import { ResultGrid } from '@/features/results/ResultGrid'
import { MOCK_RESULT } from '@/features/results/mockResult'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useTabsStore } from '@/stores/tabs'

import { TabBar } from './TabBar'

export function WorkArea({ onNewQuery }: { onNewQuery: () => void }) {
  const hasTabs = useTabsStore((s) => s.tabs.length > 0)
  const [sql, setSql] = useState('SELECT * FROM users WHERE id > 10;')
  const [rowLimit, setRowLimit] = useState(1000)

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TabBar onNewQuery={onNewQuery} />

      {hasTabs ? (
        <Split
          id="vibe-db.workarea"
          orientation="vertical"
          panelIds={['editor', 'results']}
          className="min-h-0 flex-1"
        >
          <SplitPanel id="editor" defaultSize="38%" minSize="15%">
            <EditorPane value={sql} onChange={setSql} />
          </SplitPanel>

          <SplitHandle orientation="vertical" />

          <SplitPanel id="results" defaultSize="62%" minSize="20%">
            <div className="flex h-full min-h-0 flex-col">
              <QueryActionBar
                running={false}
                rowLimit={rowLimit}
                onRun={() => {}}
                onStop={() => {}}
                onRowLimitChange={setRowLimit}
              />
              <div className="min-h-0 flex-1">
                <ResultGrid result={MOCK_RESULT} />
              </div>
            </div>
          </SplitPanel>
        </Split>
      ) : (
        <Empty
          icon={<Terminal size={26} strokeWidth={1.5} />}
          title="No open tabs"
          hint="Open a query editor, or pick a table from the sidebar."
          action={<Button variant="primary" onClick={onNewQuery}>New Query</Button>}
        />
      )}
    </div>
  )
}
