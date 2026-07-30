import { MOCK_RESULT } from '@/features/results/mockResult'
import { ResultGrid } from '@/features/results/ResultGrid'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'

import { EditorPane } from './EditorPane'
import { QueryActionBar } from './QueryActionBar'

interface QueryWorkspaceProps {
  sql: string
  onSqlChange: (sql: string) => void
  rowLimit: number
  onRowLimitChange: (limit: number) => void
}

export function QueryWorkspace({
  sql,
  onSqlChange,
  rowLimit,
  onRowLimitChange,
}: QueryWorkspaceProps) {
  return (
    <Split
      id="vibe-db.workarea"
      orientation="vertical"
      panelIds={['editor', 'results']}
      className="h-full min-h-0"
    >
      <SplitPanel id="editor" defaultSize="38%" minSize="15%">
        <EditorPane value={sql} onChange={onSqlChange} />
      </SplitPanel>

      <SplitHandle orientation="vertical" />

      <SplitPanel id="results" defaultSize="62%" minSize="20%">
        <div className="flex h-full min-h-0 flex-col">
          <QueryActionBar
            running={false}
            rowLimit={rowLimit}
            onRun={() => {}}
            onStop={() => {}}
            onRowLimitChange={onRowLimitChange}
          />
          <div className="min-h-0 flex-1">
            <ResultGrid result={MOCK_RESULT} />
          </div>
        </div>
      </SplitPanel>
    </Split>
  )
}
