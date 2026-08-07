import { useCallback, useEffect, useRef, useState } from 'react'

import { engineInfo, namespaceOf } from '@/bridge/connections'
import type { HistoryEntry } from '@/bridge/history'
import { needsConfirming, queries, type ScriptReview } from '@/bridge/query'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { IDLE_RUN, useQueriesStore } from '@/stores/queries'
import { useTabsStore, type QueryTab } from '@/stores/tabs'

import { EditorPane, type EditorHandle } from './EditorPane'
import { QueryOutput } from './QueryOutput'
import { QueryToolbar } from './QueryToolbar'
import { RunGuard } from './RunGuard'
import { ScriptProblems } from './ScriptProblems'
import { useQuerySchema } from './useQuerySchema'
import { useScriptReview } from './useScriptReview'

interface QueryWorkspaceProps {
  tab: QueryTab
  sessionId: string | null
  databases: string[]
  schemas: string[]
  engine: string
}

export function QueryWorkspace({
  tab,
  sessionId,
  databases,
  schemas,
  engine,
}: QueryWorkspaceProps) {
  const run = useQueriesStore((s) => s.runs[tab.id] ?? IDLE_RUN)
  const execute = useQueriesStore((s) => s.execute)
  const rerun = useQueriesStore((s) => s.rerun)
  const cancel = useQueriesStore((s) => s.cancel)
  const editQuery = useTabsStore((s) => s.editQuery)
  const retarget = useTabsStore((s) => s.retarget)

  const { hasSchemas } = engineInfo(engine)
  const { database, schema } = tab.namespace

  // The tables completion offers — this tab's namespace, and every other one the script joins to.
  const querySchema = useQuerySchema(tab.profileId, tab.namespace, engine, tab.sql)

  // A selection narrows what runs, the way every other client works. It is a ref because
  // dragging over the text would otherwise re-render the whole workspace.
  const selection = useRef('')

  // The editor answers one question from outside: show me that offset. It is what makes the
  // problems strip worth clicking.
  const editor = useRef<EditorHandle>(null)

  const canRun = sessionId !== null && tab.sql.trim() !== ''

  // What the whole buffer says about itself, on its own clock: it is what the editor underlines
  // and what the strip above it reports, and it follows the buffer rather than the selection so
  // the reader is warned about a statement before they think to select it.
  const review = useScriptReview(sessionId, tab.sql)

  /**
   * The run the guard is asking about, and non-null exactly while it is asking. It carries the text
   * it reviewed, so what the reader confirms is what was put in front of them however the selection
   * has moved since — and whether a plan was asked for, since **Explain runs the statement too** and
   * must therefore be confirmed like any other run.
   */
  const [asking, setAsking] = useState<{
    review: ScriptReview
    sql: string
    explain: boolean
  } | null>(null)

  const send = (sql: string, explain: boolean) => {
    if (sessionId === null) return
    void execute(tab.id, sessionId, {
      database,
      schema,
      sql,
      maxRows: tab.rowLimit,
      explain,
    })
  }

  /**
   * Execute or Explain, with the one question that stands between the reader and a statement they
   * cannot take back. Both go through it: Explain reads a plan **and runs the statement**, so a
   * `DELETE` explained is a `DELETE` done, and the guard is what says so first.
   *
   * What is asked about is reviewed **here and now** rather than read off the strip: the strip
   * follows the whole buffer on a debounce, and what runs may be a *selection* made since.
   */
  const start = (explain: boolean) => {
    if (!canRun || sessionId === null) return

    const sql = selection.current.trim() || tab.sql

    void queries
      .review(sessionId, sql)
      .then((review) => {
        if (needsConfirming(review).length === 0) {
          send(sql, explain)
          return
        }
        setAsking({ review, sql, explain })
      })
      // A review that cannot be had is not a reason to refuse to run: the reader asked, and the
      // server is still the thing that decides. It is the guard that is missing, not the query.
      .catch(() => send(sql, explain))
  }

  const confirmRun = () => {
    if (asking === null) return

    send(asking.sql, asking.explain)
    setAsking(null)
  }

  /**
   * Lays the buffer out. The whole buffer, never the selection: a statement pulled out of its
   * script, re-indented and put back would be indented against nothing.
   */
  const format = useCallback(() => {
    if (sessionId === null || tab.sql.trim() === '') return
    void queries
      .format(sessionId, tab.sql)
      .then((laid) => laid !== tab.sql && editQuery(tab.id, { sql: laid }))
      .catch(() => {})
  }, [editQuery, sessionId, tab.id, tab.sql])

  // Refresh replays the statement that produced the rows on screen, not whatever the
  // editor holds now, so applying an edit cannot silently run something else.
  const refresh = useCallback(() => {
    if (sessionId !== null) void rerun(tab.id, sessionId)
  }, [rerun, tab.id, sessionId])

  // The same replay, but for as many rows as the pager needs. The toolbar's own Limit is left
  // alone: it sizes the *next* Execute, not the result already on screen.
  const fetchRows = useCallback(
    (maxRows: number) => {
      if (sessionId !== null) void rerun(tab.id, sessionId, maxRows)
    },
    [rerun, tab.id, sessionId],
  )

  /**
   * A statement picked out of the History pane takes the editor **and the namespace it ran in**: a
   * statement names tables of one database, and one run against another would fail on names that
   * are not there. The editor's own undo is what puts back whatever was being typed.
   */
  const pickHistory = (entry: HistoryEntry) => {
    editQuery(tab.id, { sql: entry.sql })
    if (entry.database !== '') retarget(tab.id, namespaceOf(entry.database, entry.schema))
  }

  // Both shortcuts are bound on the window, because F5 reloads the WebView whenever focus sits
  // outside this panel. The ref keeps the listeners off the keystroke path.
  const latest = useRef({ start, format })
  latest.current = { start, format }

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const modifier = event.ctrlKey || event.metaKey

      if (modifier && event.shiftKey && event.key.toLowerCase() === 'f') {
        event.preventDefault()
        latest.current.format()
        return
      }
      if (event.key === 'F5' || (event.key === 'Enter' && modifier)) {
        event.preventDefault()
        latest.current.start(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QueryToolbar
        databases={databases}
        schemas={schemas}
        namespace={tab.namespace}
        hasSchemas={hasSchemas}
        rowLimit={tab.rowLimit}
        running={run.running}
        canRun={canRun}
        onNamespaceChange={(namespace) => retarget(tab.id, namespace)}
        onRowLimitChange={(rowLimit) => editQuery(tab.id, { rowLimit })}
        onExecute={() => start(false)}
        onExplain={() => start(true)}
        onFormat={format}
        onStop={() => cancel(tab.id)}
      />

      <ScriptProblems review={review} onReveal={(offset) => editor.current?.reveal(offset)} />

      <Split
        id="vibe-db.workarea"
        orientation="vertical"
        panelIds={['editor', 'output']}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="editor" defaultSize="38%" minSize="15%">
          <EditorPane
            ref={editor}
            value={tab.sql}
            schema={querySchema}
            engine={engine}
            review={review}
            onChange={(sql) => editQuery(tab.id, { sql })}
            onSelectionChange={(selected) => {
              selection.current = selected
            }}
          />
        </SplitPanel>

        <SplitHandle orientation="vertical" />

        <SplitPanel id="output" defaultSize="62%" minSize="20%">
          <QueryOutput
            run={run}
            sessionId={sessionId}
            profileId={tab.profileId}
            onRefresh={refresh}
            onFetch={fetchRows}
            onStop={() => cancel(tab.id)}
            onPickHistory={pickHistory}
          />
        </SplitPanel>
      </Split>

      <RunGuard
        review={asking?.review ?? null}
        onCancel={() => setAsking(null)}
        onConfirm={confirmRun}
      />
    </div>
  )
}
