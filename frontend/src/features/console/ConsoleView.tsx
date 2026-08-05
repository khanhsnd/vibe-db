import { useCallback, useMemo } from 'react'
import { Eraser, Square } from 'lucide-react'

import { engineInfo } from '@/bridge/connections'
import { NamespacePicker } from '@/features/connections/NamespacePicker'
import { ResultPane } from '@/features/results/ResultPane'
import { Button } from '@/shared/ui/Button'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { entriesOf, useConsoleStore } from '@/stores/console'
import { IDLE_RUN, useQueriesStore } from '@/stores/queries'
import { useTabsStore, type ConsoleTab } from '@/stores/tabs'

import { ConsoleLog } from './ConsoleLog'
import { ConsolePrompt } from './ConsolePrompt'

interface ConsoleViewProps {
  tab: ConsoleTab
  sessionId: string | null
  databases: string[]
  schemas: string[]
  engine: string
}

/**
 * A connection driven one statement at a time: the transcript of everything sent, the prompt it is
 * sent from, and — below both — the rows of the last statement that returned any, in the same
 * `ResultPane` a query tab shows. Nothing about *running* is this tab's own: it executes through
 * the same store a query tab does, so a console's rows are paged, sorted, exported, edited and
 * stopped exactly as any other result is.
 *
 * What it does not have is the editor: no completion, no formatting, no reviewing a whole script
 * before it runs. A console is for one line at a time, and a reader with a script to think about
 * wants a query tab — see [features.md](../../../../docs/features.md).
 */
export function ConsoleView({ tab, sessionId, databases, schemas, engine }: ConsoleViewProps) {
  const run = useQueriesStore((s) => s.runs[tab.id] ?? IDLE_RUN)
  const rerun = useQueriesStore((s) => s.rerun)
  const cancel = useQueriesStore((s) => s.cancel)
  const entries = useConsoleStore((s) => entriesOf(s, tab.id))
  const send = useConsoleStore((s) => s.send)
  const clear = useConsoleStore((s) => s.clear)
  const retarget = useTabsStore((s) => s.retarget)

  const history = useMemo(() => entries.map((entry) => entry.sql), [entries])

  const refresh = useCallback(() => {
    if (sessionId !== null) void rerun(tab.id, sessionId)
  }, [rerun, tab.id, sessionId])

  const fetchRows = useCallback(
    (maxRows: number) => {
      if (sessionId !== null) void rerun(tab.id, sessionId, maxRows)
    },
    [rerun, tab.id, sessionId],
  )

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div
        role="toolbar"
        className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2"
      >
        <NamespacePicker
          databases={databases}
          schemas={schemas}
          namespace={tab.namespace}
          hasSchemas={engineInfo(engine).hasSchemas}
          onChange={(namespace) => retarget(tab.id, namespace)}
        />

        <span className="mx-1 h-4 w-px bg-line" />

        <Button
          variant={run.running ? 'danger' : 'default'}
          icon={<Square size={11} fill="currentColor" />}
          disabled={!run.running}
          onClick={() => cancel(tab.id)}
        >
          Stop
        </Button>

        <Button
          variant="default"
          icon={<Eraser size={12} />}
          disabled={entries.length === 0}
          onClick={() => clear(tab.id)}
        >
          Clear
        </Button>
      </div>

      <Split
        id="vibe-db.console"
        orientation="vertical"
        panelIds={['transcript', 'rows']}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="transcript" defaultSize="45%" minSize="20%">
          <div className="flex h-full min-h-0 flex-col">
            <ConsoleLog entries={entries} running={run.running} />
            <ConsolePrompt
              history={history}
              disabled={sessionId === null || run.running}
              onSend={(sql) => {
                if (sessionId !== null) void send(tab.id, sessionId, tab.namespace, sql)
              }}
            />
          </div>
        </SplitPanel>

        <SplitHandle orientation="vertical" />

        <SplitPanel id="rows" defaultSize="55%" minSize="20%">
          <ResultPane
            run={run}
            sessionId={sessionId}
            onRefresh={refresh}
            onFetch={fetchRows}
            onStop={() => cancel(tab.id)}
          />
        </SplitPanel>
      </Split>
    </div>
  )
}
