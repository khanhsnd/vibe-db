import { useCallback, useMemo, useState, type ReactNode } from 'react'
import {
  Activity,
  CirclePause,
  Gauge,
  Search,
  SlidersHorizontal,
  Timer,
  type LucideIcon,
} from 'lucide-react'

import { engineInfo } from '@/bridge/connections'
import {
  killProcesses,
  listProcesses,
  loadServerStatus,
  loadServerVariables,
  previewProcessKills,
  processKill,
  type ProcessAction,
  type ProcessRef,
} from '@/bridge/monitor'
import type { TableColumn } from '@/bridge/tables'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import { selectedRows, type GridData, type Selection } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Loading } from '@/shared/ui/Loading'
import { MenuButton } from '@/shared/ui/MenuButton'
import { ICON } from '@/shared/ui/menuIcons'
import { Segmented } from '@/shared/ui/Segmented'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore, type MonitorPane, type MonitorTab } from '@/stores/tabs'

import { narrow } from './narrow'
import { useServerReport } from './useServerReport'

/** How often the pane may re-read itself, in seconds. `0` is off, and is what a pick holds it at. */
const INTERVALS: readonly number[] = [0, 2, 5, 10, 30]

/**
 * What each pane is called and what it wears, in the order the toolbar draws them: what the server is
 * *doing*, what it has *counted*, and what it was *set to*.
 */
const PANES: ReadonlyArray<{ id: MonitorPane; label: string; icon: LucideIcon }> = [
  { id: 'processes', label: 'Processes', icon: Activity },
  { id: 'status', label: 'Status', icon: Gauge },
  { id: 'variables', label: 'Variables', icon: SlidersHorizontal },
]

/**
 * What a server is doing and what it was started with — what the `Server Monitor` node opens. Three
 * panes over one `ResultGrid`, in whatever columns that engine's own catalogue keeps, like every other
 * list in this app.
 *
 * There is **one of these per connection**, and asking for a second pane moves this tab rather than
 * opening another: a process belongs to the server, and the server is what the tab is.
 *
 * See [docs/features/monitor.md](../../../../docs/features/monitor.md).
 */
export function MonitorView({ tab, sessionId }: { tab: MonitorTab; sessionId: string }) {
  const [selection, setSelection] = useState<Selection | null>(null)
  const [find, setFind] = useState('')

  const editMonitor = useTabsStore((s) => s.editMonitor)
  const ask = useSchemaStore((s) => s.ask)
  const reportError = useConnectionsStore((s) => s.reportError)
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === tab.profileId)?.engine ?? '',
  )
  const info = engineInfo(engine)

  const read = useCallback((): Promise<Report> => {
    if (tab.pane === 'status') return loadServerStatus(sessionId)
    if (tab.pane === 'variables') return loadServerVariables(sessionId)
    return listProcesses(sessionId)
  }, [sessionId, tab.pane])

  // The clock is **held while anything is picked**. A list that moved under a pick would leave the
  // reader's rows pointing at somebody else's process, and the next press would end that one — which
  // is the one mistake a monitor must not make, and is why this is not merely a convenience.
  const picking = selectedRows(selection).length > 0
  const { data, loading, error, refresh } = useServerReport(read, picking ? 0 : tab.every)

  const shown = useMemo(
    () => narrow(data?.rows ?? [], data?.refs ?? [], find),
    [data, find],
  )

  const grid: GridData | null = data
    ? {
        columns: data.columns.map((column) => ({ ...column, readOnly: true })),
        rows: shown.rows,
      }
    : null

  const picked = useMemo(
    () => selectedRows(selection).flatMap((row) => {
      const ref = shown.refs[row]
      return ref ? [ref] : []
    }),
    [selection, shown.refs],
  )

  /**
   * Ends what is picked, having first asked the Go side what that will run. The statements are what
   * the confirmation shows: there is no Changes pane here, so the preview *is* the sentence — and
   * whether it has to be answered for at all is a `domain` rule already applied against this
   * connection's environment.
   */
  const kill = async (action: ProcessAction) => {
    if (picked.length === 0) return
    const kills = picked.map((ref) => processKill(ref, action))

    const run = async () => {
      await killProcesses(sessionId, kills)
      setSelection(null)
      refresh()
    }

    try {
      const preview = await previewProcessKills(sessionId, kills)
      if (!preview.confirm) {
        await run()
        return
      }

      ask({
        kind: 'confirm',
        title: action === 'cancel' ? 'Cancel Statement' : `End ${info.processLabel}`,
        message: killMessage(action, picked, preview.statements, info.processLabel),
        submitLabel: action === 'cancel' ? 'Cancel Statement' : 'End',
        danger: true,
        onSubmit: run,
      })
    } catch (thrown) {
      // Nothing is waiting on the answer at this point — the dialog is not up yet — so the failure
      // goes where every other action raised from a toolbar puts one: the bar.
      reportError(thrown)
    }
  }

  const processes = tab.pane === 'processes'

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        {/* Which pane is being read is a `Segmented` and not three held-down buttons: on a toolbar a
            pressed button reads as a shade beside its neighbours, and three of those read as three
            buttons with one shaded rather than as "this is the pane you are on". It is the control a
            comparison's two halves already use. */}
        <Segmented
          label="Which half of the server is being read"
          options={PANES}
          value={tab.pane}
          onChange={(pane) => {
            editMonitor(tab.id, { pane })
            setSelection(null)
          }}
        />

        <div className="mx-1 h-4 w-px bg-line" />

        {/* Both kills are drawn only where the engine has one: `KILL QUERY` and `pg_cancel_backend`
            stop a statement, and a MongoDB client is disconnected by the client — see
            `cancelsStatement` and `closesConnection`. */}
        {processes && info.cancelsStatement ? (
          <Button
            variant="subtle"
            icon={<CirclePause size={13} />}
            disabled={picked.length === 0}
            onClick={() => void kill('cancel')}
          >
            Cancel Statement
          </Button>
        ) : null}
        {processes && info.closesConnection ? (
          <Button
            variant="subtle"
            icon={ICON.delete}
            disabled={picked.length === 0}
            onClick={() => void kill('close')}
            className="text-danger hover:bg-danger-subtle hover:text-danger"
          >
            End {info.processLabel}
          </Button>
        ) : null}

        <Button variant="subtle" icon={ICON.refresh} onClick={refresh}>
          Refresh
        </Button>

        {/* How often the pane asks again. It is a menu rather than a row of buttons because it is set
            once and then read as a label — and the label is what says the pane is live. */}
        <MenuButton
          items={INTERVALS.map((seconds) => ({
            label: seconds === 0 ? 'Off' : `Every ${seconds}s`,
            onSelect: () => editMonitor(tab.id, { every: seconds }),
          }))}
        >
          <Button variant="subtle" icon={<Timer size={13} />}>
            {tab.every === 0 ? 'Auto Refresh: Off' : `Every ${tab.every}s`}
          </Button>
        </MenuButton>

        <div className="ml-auto flex items-center gap-2">
          {/* Why the pane has stopped moving, said where the reader is looking. */}
          {picking && tab.every > 0 ? (
            <span className="text-2xs text-faint">Held while a row is picked</span>
          ) : null}

          <div className="flex h-6 items-center gap-1.5 rounded-md border border-line-strong bg-elevated px-2">
            <Search size={12} className="shrink-0 text-faint" />
            <input
              value={find}
              onChange={(event) => {
                setFind(event.target.value)
                setSelection(null)
              }}
              placeholder="Find"
              aria-label="Narrow the list"
              className="w-40 bg-transparent text-xs text-fg outline-none placeholder:text-faint"
            />
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1">
        {grid === null ? (
          error ? (
            <Empty title={error.message} hint={error.detail} />
          ) : (
            <Loading label="Asking the server…" />
          )
        ) : grid.rows.length === 0 ? (
          <Empty
            title={emptyTitle(tab.pane, find !== '', info.processLabel)}
            hint={find === '' ? undefined : 'Clear the box to see the whole list.'}
          />
        ) : (
          <ResultGrid
            result={grid}
            selection={selection}
            onSelectionChange={setSelection}
            onRefresh={refresh}
          />
        )}
      </div>

      {/* A failed re-read leaves the list already drawn where it is and says so under it, which is
          what a table view does with the same failure — a monitor that blanked itself the moment a
          server hiccoughed would be one nobody could watch. */}
      {grid !== null && error !== null ? <ErrorStrip error={error} /> : null}

      <GridFooter editable={false} busy={loading} onRefresh={refresh}>
        <span>{footer(tab.pane, data, shown.rows.length, picked)}</span>
      </GridFooter>
    </div>
  )
}

/**
 * What either half of the monitor answers with. The process list carries references and the two halves
 * of the status do not, which is the whole of the difference — so one shape covers all three and the
 * grid below does not learn which pane it is drawing.
 */
interface Report {
  columns: TableColumn[]
  rows: (string | null)[][]
  refs?: ProcessRef[]
}

/**
 * The sentence a kill is answered for by. It carries the **statements**, because this app never asks a
 * reader to confirm something it has not shown them, and it says plainly when one of the picked rows
 * is this app's own connection — which is the reason that case is confirmed on a laptop at all.
 */
function killMessage(
  action: ProcessAction,
  picked: ProcessRef[],
  statements: string[],
  label: string,
): string {
  const what =
    action === 'cancel'
      ? `Cancel the statement on ${count(picked.length, label.toLowerCase())}?`
      : `End ${count(picked.length, label.toLowerCase())}? Whatever each one had open is rolled back.`

  const ours = picked.some((ref) => ref.own)
    ? ' One of them is a connection this app itself is reading on, so this window will lose it.'
    : ''

  const shown = statements.length === 0 ? '' : ` This runs: ${statements.join('; ')}.`
  return what + ours + shown
}

function count(many: number, noun: string): string {
  return many === 1 ? `1 ${noun}` : `${many} ${noun}s`
}

function emptyTitle(pane: MonitorPane, narrowed: boolean, label: string): string {
  if (narrowed) return 'Nothing matches'
  if (pane === 'processes') return `No ${label.toLowerCase()}s to show`
  return 'The server reported nothing'
}

function footer(
  pane: MonitorPane,
  data: Report | null,
  shownRows: number,
  picked: ProcessRef[],
): ReactNode {
  if (data === null) return ''

  const held = data.rows.length
  const rows = shownRows === held ? `${held}` : `${shownRows} of ${held}`
  const noun = pane === 'processes' ? 'rows' : 'entries'
  const pick = picked.length === 0 ? '' : ` · ${picked.length} picked`
  return `${rows} ${noun}${pick}`
}
