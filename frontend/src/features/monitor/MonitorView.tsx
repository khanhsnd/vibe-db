import { useCallback, useMemo, useState, type ReactNode } from 'react'
import {
  Activity,
  ChartSpline,
  CirclePause,
  Database,
  FlaskConical,
  Gauge,
  Search,
  SlidersHorizontal,
  Timer,
  TimerReset,
  type LucideIcon,
} from 'lucide-react'

import { engineInfo, namespaceOf, NO_NAMESPACE } from '@/bridge/connections'
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
import { loadSlowQueries, type SlowQueryRef } from '@/bridge/profiler'
import type { TableColumn } from '@/bridge/tables'
import { CellEditor, EDITOR_HEIGHT, EDITOR_MIN_HEIGHT } from '@/features/results/CellEditor'
import { CellEditorButton } from '@/features/results/CellEditorButton'
import { FilterSortButton } from '@/features/results/FilterSortButton'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import { countConditions, NO_FILTER, type Group } from '@/features/results/filter'
import {
  focusOf,
  NO_SORT,
  oneRange,
  selectedRows,
  type CellRef,
  type GridData,
  type Selection,
  type SortKey,
} from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { CopyButton } from '@/shared/ui/CopyButton'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { IconButton } from '@/shared/ui/IconButton'
import { Loading } from '@/shared/ui/Loading'
import { MenuButton } from '@/shared/ui/MenuButton'
import { ICON } from '@/shared/ui/menuIcons'
import { Segmented } from '@/shared/ui/Segmented'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore, type MonitorPane, type MonitorTab } from '@/stores/tabs'

import { arrange } from './arrange'
import { HISTORY, WINDOW, type Reading } from './metrics'
import { MetricsPane } from './MetricsPane'
import { ProfileDialog } from './ProfileDialog'
import { useMetricHistory } from './useMetricHistory'
import { useServerReport } from './useServerReport'

/** How often the pane may re-read itself, in seconds. `0` is off, and is what a pick holds it at. */
const INTERVALS: readonly number[] = [0, 2, 5, 10, 30]

/**
 * How many slow statements are read at once. It is a ceiling rather than a page: every engine here
 * keeps a bounded record of its own — a capped collection, a ring, a plan cache — and a reader
 * hunting a slow statement is looking at the top of the list rather than paging to the bottom of it.
 */
const SLOW_ROWS = 200

/**
 * What each pane is called and what it wears, in the order the toolbar draws them: what the server is
 * *doing now*, what it has been doing *over the last few minutes*, what it has already *done* slowly,
 * what it has *counted*, and what it was *set to*.
 *
 * Charts come first because it is the one that answers "is this server all right" without the reader
 * knowing what to look for. The four beside it are lists, and a list is read by somebody who already
 * has a question.
 */
const PANES: ReadonlyArray<{ id: MonitorPane; label: string; icon: LucideIcon }> = [
  { id: 'charts', label: 'Charts', icon: ChartSpline },
  { id: 'processes', label: 'Processes', icon: Activity },
  { id: 'slow', label: 'Slow Queries', icon: TimerReset },
  { id: 'status', label: 'Status', icon: Gauge },
  { id: 'variables', label: 'Variables', icon: SlidersHorizontal },
]

/**
 * What a server is doing and what it was started with — what the `Server Monitor` node opens. Four
 * panes over one `ResultGrid`, in whatever columns that engine's own catalogue keeps like every other
 * list in this app, and one that is not a list at all: the [charts](./MetricsPane.tsx), which are the
 * same servers read for what is *moving* rather than for what is there.
 *
 * There is **one of these per connection**, and asking for a second pane moves this tab rather than
 * opening another: a process belongs to the server, and the server is what the tab is.
 *
 * See [docs/features/monitor.md](../../../../docs/features/monitor.md).
 */
export function MonitorView({ tab, sessionId }: { tab: MonitorTab; sessionId: string }) {
  const [selection, setSelection] = useState<Selection | null>(null)
  const [find, setFind] = useState('')
  const [configuring, setConfiguring] = useState(false)
  const [valueShown, setValueShown] = useState(false)
  // How the pane is narrowed and ordered. It is this view's own state rather than the tab's, which is
  // where `find` already lives and for the same reason: it is about the *columns being read*, and
  // every one of them changes the moment the reader switches pane.
  const [criteria, setCriteria] = useState<Group>(NO_FILTER)
  const [filterShown, setFilterShown] = useState(false)
  const [sort, setSort] = useState<readonly SortKey[]>(NO_SORT)

  const editMonitor = useTabsStore((s) => s.editMonitor)
  const ask = useSchemaStore((s) => s.ask)
  const reportError = useConnectionsStore((s) => s.reportError)
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === tab.profileId)?.engine ?? '',
  )
  // The databases this connection draws, for the one pane that is about a database rather than about
  // the server. A selector is drawn only where the engine keeps the recording per database.
  const databases = useConnectionsStore((s) => s.databases[tab.profileId] ?? EMPTY_DATABASES)
  const info = engineInfo(engine)

  // Which database the Slow Queries pane is about. Empty is the connection's own, which is what the
  // four server-wide engines ignore entirely.
  //
  // It is read through a fallback rather than trusted, because a tab is a **stored** object: one
  // opened before this field existed carries no `database` at all, and `namespaceOf(undefined)` would
  // put an undefined where the wire expects a string. Every other tab kind takes the same care.
  const database = tab.database ?? ''
  const namespace = useMemo(
    () => (database === '' ? NO_NAMESPACE : namespaceOf(database)),
    [database],
  )

  const charts = tab.pane === 'charts'

  const read = useCallback((): Promise<Report> => {
    // The charts pane is not a list, so there is nothing here for it to ask. It reads its own numbers
    // on its own clock — see `useMetricHistory` — and leaving this one to fall through to the process
    // list would have a busy server answering a question nobody on screen was looking at.
    if (charts) return Promise.resolve(NO_REPORT)
    if (tab.pane === 'status') return loadServerStatus(sessionId)
    if (tab.pane === 'variables') return loadServerVariables(sessionId)
    if (tab.pane === 'slow') return loadSlowQueries(sessionId, namespace, SLOW_ROWS)
    return listProcesses(sessionId)
  }, [sessionId, tab.pane, namespace, charts])

  // The clock is **held while anything is picked**. A list that moved under a pick would leave the
  // reader's rows pointing at somebody else's process, and the next press would end that one — which
  // is the one mistake a monitor must not make, and is why this is not merely a convenience.
  const picking = selectedRows(selection).length > 0
  const { data, loading, error, refresh } = useServerReport(
    read,
    picking || charts ? 0 : tab.every,
    // What is being asked, which is what the pane re-reads itself on. The database is part of it
    // because on two engines the recording belongs to one: switching either is a different question,
    // and a reader who asked it should see the answer rather than have to press Refresh.
    `${tab.pane}:${database}`,
  )

  // The readings behind the charts. They are kept in a store keyed by this tab rather than in any
  // component, because only the tab in front is mounted: minutes of collected history are the one
  // thing on this tab that is worse for being thrown away and read again, and looking at a query tab
  // would otherwise do it. The clock is still this view's — see `useMetricHistory`.
  const metrics = useMetricHistory(tab.id, sessionId, tab.every, charts)

  const shown = useMemo(
    () =>
      arrange(data?.rows ?? [], data?.refs ?? [], data?.columns ?? [], find, criteria, sort),
    [data, find, criteria, sort],
  )

  // Whether the reader has narrowed the pane themselves, by the box or by a condition. It decides
  // what an empty pane says, which is the one place the difference matters.
  const narrowed = find.trim() !== '' || countConditions(criteria) > 0

  const grid: GridData | null = data
    ? {
        columns: data.columns.map((column) => ({ ...column, readOnly: true })),
        rows: shown.rows,
      }
    : null

  /**
   * The processes a pick names. Only the process list carries references, so only its rows can be
   * *acted* on: a pick anywhere else moves the value panel and nothing else, which is what the other
   * three panes are picked for.
   */
  const picked = useMemo(
    () => selectedRows(selection).flatMap((row) => {
      const ref = shown.refs[row]
      return ref && isProcess(ref) ? [ref] : []
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

  /**
   * The cell the value panel is on — the focus of the selection, read against the rows as drawn.
   * The panel is over a **narrowed** list, so its coordinates are the narrowed ones: what is under
   * the pointer is what is read.
   */
  const focus = focusOf(selection)
  const focusedRow = focus ? shown.rows[focus.row] : undefined
  const focusedColumn = focus && data ? data.columns[focus.column] : undefined
  const focused =
    focus && focusedRow && focusedColumn
      ? {
          ref: focus,
          column: { ...focusedColumn, readOnly: true },
          value: focusedRow[focus.column] ?? null,
        }
      : null

  /** The panel reads the focused cell, so the menu's own cell becomes the selection first. */
  const showInPanel = (ref: CellRef) => {
    setSelection(oneRange({ anchor: ref, focus: ref }))
    setValueShown(true)
  }

  /**
   * Reads the recording of a different database. The pick goes with it: a selection is a row of the
   * database being left, and carrying it across would leave the value panel reading a row that is no
   * longer there — the same reason switching panes clears one.
   *
   * Nothing else happens here. The read follows from the tab having changed, because that is what
   * `useServerReport` is asked to watch.
   */
  const pickDatabase = (database: string) => {
    editMonitor(tab.id, { database })
    setSelection(null)
  }

  // The panel is closed while there is nothing to read under it, so a failed read does not leave it
  // stranded open over an empty pane — the arrangement a query result's has. A read **in flight** is
  // not that case: closing the panel while the reader switches database would take the pane apart and
  // put it back for the length of one query.
  const valueOpen = valueShown && (grid !== null || loading)
  const panels = useMemo(() => (valueOpen ? ['rows', 'value'] : ['rows']), [valueOpen])

  const processes = tab.pane === 'processes'
  const slow = tab.pane === 'slow'

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      {/* The toolbar **scrolls sideways** rather than letting its controls run off the edge. This
          pane carries more of them than any other monitor surface — four segments, the switch, a
          database, the clock and the box — and on a narrow window the ones on the right were simply
          lost. Every control keeps its own width (`Button` is already `shrink-0`), so nothing is
          squeezed to fit; what does not fit is reached by scrolling. */}
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 overflow-x-auto border-b border-line bg-bar px-1.5"
      >
        {/* Which pane is being read is a `Segmented` and not four held-down buttons: on a toolbar a
            pressed button reads as a shade beside its neighbours, and four of those read as four
            buttons with one shaded rather than as "this is the pane you are on". It is the control a
            comparison's two halves already use.

            The Slow Queries pane is dropped where the engine has no recording — a SQLite database is
            a file and keeps none — rather than being drawn over an `UNSUPPORTED`. */}
        <Segmented
          label="Which quarter of the server is being read"
          options={info.hasProfiler ? PANES : PANES.filter((pane) => pane.id !== 'slow')}
          value={tab.pane}
          onChange={(pane) => {
            editMonitor(tab.id, { pane })
            // The pane's columns are all different ones, so everything said *about* columns starts
            // over: a condition on `Duration` means nothing over a variable list, and a sort key
            // naming a column that is not there would be a sort button with nothing behind it.
            setSelection(null)
            setFind('')
            setCriteria(NO_FILTER)
            setSort(NO_SORT)
            setFilterShown(false)
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

        {/* The switch, drawn only where the engine has one. SQL Server and Oracle record whether or
            not anybody asked, so there is nothing here to open — see `profilerConfigurable`.

            It is the only button this pane adds. Nothing in a slow list is *acted* on: a row is read
            here and in the panel below, and what a reader takes away they copy. */}
        {slow && info.profilerConfigurable ? (
          <Button
            variant="subtle"
            icon={<Timer size={13} />}
            onClick={() => setConfiguring(true)}
          >
            Profiling…
          </Button>
        ) : null}

        {/* Which database the recording is being read from, on the two engines that keep it per
            database. Everywhere else this is a server fact and the selector would be a control that
            changed nothing. */}
        {slow && info.profilerPerDatabase && databases.length > 0 ? (
          <MenuButton
            items={[
              { label: 'This connection’s own', onSelect: () => pickDatabase('') },
              ...databases.map((name) => ({
                label: name,
                onSelect: () => pickDatabase(name),
              })),
            ]}
          >
            <Button variant="subtle" icon={<Database size={13} />}>
              {database === '' ? 'Default database' : database}
            </Button>
          </MenuButton>
        ) : null}

        <Button
          variant="subtle"
          icon={ICON.refresh}
          onClick={charts ? metrics.refresh : refresh}
        >
          Refresh
        </Button>

        {/* The two panel toggles. They are here rather than in the head strip the grid draws for
            itself, because this surface has a toolbar of its own and one table must never carry two
            strips — see docs/grid.md. The count on the first is what says the grid is still narrowed
            after the panel is closed. */}
        {charts ? null : (
          <>
            <FilterSortButton
              shown={filterShown}
              onToggle={() => setFilterShown(!filterShown)}
              count={countConditions(criteria) + sort.length}
            />

            <CellEditorButton
              shown={valueShown}
              onToggle={() => setValueShown(!valueShown)}
              label="Value"
            />
          </>
        )}

        {/* How often the pane asks again. It is a menu rather than a row of buttons because it is set
            once and then read as a label — and the label is what says the pane is live. */}
        <MenuButton
          items={INTERVALS.map((seconds) => ({
            label: seconds === 0 ? 'Off' : `Every ${seconds}s`,
            onSelect: () => editMonitor(tab.id, { every: seconds }),
          }))}
        >
          {/* `Auto: 5s` rather than `Auto Refresh: Every 5s`. It is read as a state and not as a
              sentence, and the longer wording was the widest thing on a crowded bar. */}
          <Button variant="subtle" icon={<Timer size={13} />}>
            {tab.every === 0 ? 'Auto: Off' : `Auto: ${tab.every}s`}
          </Button>
        </MenuButton>

        <div className="ml-auto flex shrink-0 items-center gap-2 pl-1">
          {/* Why the pane has stopped moving, said where the reader is looking. It is the one thing
              here that gives way on a narrow window: the box beside it is what the reader is
              reaching for, and this sentence is an explanation rather than a control. */}
          {picking && !charts && tab.every > 0 ? (
            <span className="hidden text-2xs text-faint xl:inline">Held while a row is picked</span>
          ) : null}

          {/* The box narrows a **list**, and the charts pane has none: what would it hide, a line?
              The panels are already the four questions, so there is nothing here to narrow to. */}
          {charts ? null : (
            <div className="flex h-6 shrink-0 items-center gap-1.5 rounded-md border border-line-strong bg-elevated px-2">
              <Search size={12} className="shrink-0 text-faint" />
              <input
                value={find}
                onChange={(event) => {
                  setFind(event.target.value)
                  setSelection(null)
                }}
                placeholder="Find"
                aria-label="Narrow the list"
                className="w-24 bg-transparent text-xs text-fg outline-none placeholder:text-faint lg:w-40"
              />
            </div>
          )}
        </div>
      </div>

      {charts ? (
        <MetricsPane history={metrics.history} reading={metrics.reading} error={metrics.error} />
      ) : (
        <>
          {/* The rows, and under them the value of whichever cell is focused. The panel is the reason
              this is a `Split` at all: every pane here holds cells far longer than a column is wide — a
              statement, a `currentOp` document, a `CLIENT LIST` line — and reading one used to mean
              copying it out of the app. It is `CellEditor` in its read-only shape, the same panel a
              table view and a query result put under their own grids. */}
          <Split
            id="vibe-db.monitor.rows"
            orientation="vertical"
            panelIds={panels}
            className="min-h-0 flex-1"
          >
            <SplitPanel id="rows" minSize="25%">
              {grid === null ? (
                error ? (
                  <Empty title={error.message} hint={error.detail} />
                ) : (
                  <Loading label="Asking the server…" />
                )
              ) : grid.rows.length === 0 ? (
                <Empty
                  title={emptyTitle(tab.pane, narrowed, info.processLabel)}
                  hint={narrowed ? 'Widen the box or the conditions to see the whole list.' : undefined}
                />
              ) : (
                <ResultGrid
                  result={grid}
                  selection={selection}
                  onSelectionChange={setSelection}
                  onRefresh={refresh}
                  headStrip={false}
                  cellPanel={{ shown: valueShown, onToggle: () => setValueShown(!valueShown) }}
                  onEditInPanel={showInPanel}
                  // Both narrow **what is already in memory** — see `arrange`. There is no `suggest`,
                  // because offering the values a column holds would mean asking the server a second
                  // question about a list it has already answered with in full.
                  sort={sort}
                  onSortChange={(next) => {
                    setSort(next)
                    setSelection(null)
                  }}
                  filtering={{
                    criteria,
                    shown: filterShown,
                    onShownChange: setFilterShown,
                    onChange: (next) => {
                      setCriteria(next)
                      setSelection(null)
                    },
                  }}
                  // What a reader *does* with a recorded statement, pinned to the right edge where it
                  // stays reachable however wide the statement column runs. Only the slow list has one:
                  // the other three panes are watched rather than taken away from.
                  rowActions={
                    slow
                      ? {
                          label: 'Take',
                          width: ROW_ACTIONS_WIDTH,
                          render: (row) => <SlowRowActions statement={statementAt(shown.refs, row)} />,
                        }
                      : undefined
                  }
                />
              )}
            </SplitPanel>

            {valueOpen ? (
              <>
                <SplitHandle orientation="vertical" />

                <SplitPanel
                  id="value"
                  defaultSize={EDITOR_HEIGHT}
                  minSize={EDITOR_MIN_HEIGHT}
                  maxSize="70%"
                >
                  {focused === null ? (
                    <Empty title="No cell selected" hint="Click a cell in the grid to read its value." />
                  ) : (
                    <CellEditor
                      key={`${focused.ref.row}:${focused.ref.column}`}
                      column={focused.column}
                      value={focused.value}
                      // Nothing on any of these four panes is written back: a process list, a slow log
                      // and a server's own counters are all reads. The panel says so rather than taking
                      // an edit it would then throw away.
                      readOnly="This is what the server reported, and there is nothing here to write back to."
                      busy={false}
                      onApply={NO_APPLY}
                    />
                  )}
                </SplitPanel>
              </>
            ) : null}
          </Split>

          {/* A failed re-read leaves the list already drawn where it is and says so under it, which is
              what a table view does with the same failure — a monitor that blanked itself the moment a
              server hiccoughed would be one nobody could watch. */}
          {grid !== null && error !== null ? <ErrorStrip error={error} /> : null}
        </>
      )}

      <GridFooter
        editable={false}
        busy={charts ? metrics.reading : loading}
        onRefresh={charts ? metrics.refresh : refresh}
      >
        <span>
          {charts
            ? watched(metrics.history, tab.every)
            : footer(tab.pane, data, shown.rows.length, picked.length)}
        </span>
      </GridFooter>

      {/* The switch, over the pane it is about. It reads the server itself when it opens and applies
          nothing until Save — see `ProfileDialog`. */}
      <ProfileDialog
        sessionId={sessionId}
        namespace={namespace}
        open={configuring}
        onClose={() => setConfiguring(false)}
        onApplied={refresh}
      />
    </div>
  )
}

/**
 * What each quarter of the monitor answers with. The process list carries references and the other
 * three do not — which is the whole of the difference, so one shape covers all four and the grid
 * below does not learn which pane it is drawing.
 *
 * The slow list carries none because nothing there is *acted on*: a row is read, in the grid and in
 * the panel under it, and a reference is what a row would be acted on **by**.
 */
interface Report {
  columns: TableColumn[]
  rows: (string | null)[][]
  refs?: MonitorRef[]
}

/**
 * What a row of a pane carries beside its cells. Two of the four have one, and they mean different
 * things by it: a process is **ended** by its reference, and a slow statement is **taken away** by
 * its reference. Which kind a row holds is read off the reference rather than off the pane, so the
 * two cannot be confused while a pane is being switched.
 */
type MonitorRef = ProcessRef | SlowQueryRef

function isProcess(ref: MonitorRef): ref is ProcessRef {
  return 'id' in ref
}

function isSlowQuery(ref: MonitorRef): ref is SlowQueryRef {
  return 'runnable' in ref
}

/** A read-only panel applies nothing, and says so by having nothing to call. */
const NO_APPLY = () => {}

/**
 * How wide the pinned column of buttons is: two 24 px `IconButton`s, the gap between them, and the
 * padding that keeps them clear of the edge.
 */
const ROW_ACTIONS_WIDTH = 62

/**
 * The statement one row of the shown list holds, or `null` where there is none to take away — a row
 * of a process list, or a slow row whose statement the server did not record. It reads the
 * **narrowed** references, so a filtered list hands over the row it drew.
 */
function statementAt(refs: MonitorRef[], row: number): SlowQueryRef | null {
  const ref = refs[row]
  if (ref === undefined || !isSlowQuery(ref) || ref.runnable === '') return null
  return ref
}

/**
 * What a reader does with one recorded statement. Two buttons, in the row's own pinned column.
 *
 * **Copy** puts the statement on the clipboard as something that would *run somewhere else* — the
 * driver's own wording, which is why a MySQL row arrives with its `USE` in front and a MongoDB one
 * with `getSiblingDB`. Where the engine records only what it normalized, the copied text opens with
 * two comment lines saying so, because a reader who pastes `$1` into a terminal and is refused has
 * been told nothing by the refusal. It is the app's one `CopyButton`, so the tick that confirms it
 * is the same tick everywhere else.
 *
 * **Test** is a place kept and nothing more. It is drawn disabled and says so, rather than being
 * left out and moving the Copy button the day it arrives — a button that changes place between
 * releases is worse than a button that is plainly not ready.
 */
function SlowRowActions({ statement }: { statement: SlowQueryRef | null }) {
  return (
    <>
      <CopyButton
        text={statement?.runnable ?? ''}
        label={
          statement === null
            ? 'Nothing was recorded for this row'
            : statement.complete
              ? 'Copy as a statement that will run'
              : 'Copy — the values were replaced by the server, so fill them in first'
        }
      />
      <IconButton
        label="Test — not wired up yet"
        icon={<FlaskConical size={12} />}
        disabled
        onClick={undefined}
      />
    </>
  )
}

/** A connection whose databases have not been drawn yet, held still so the selector does not loop. */
const EMPTY_DATABASES: readonly string[] = []

/**
 * What the list read answers with on the pane that has no list. It is a value rather than a `null`
 * so the grid below is drawn as an *empty* answer and never as one still in flight — and it is held
 * still because it is a dependency of the read.
 */
const NO_REPORT: Report = { columns: [], rows: [] }

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

/**
 * What an empty pane says. **Whether the reader narrowed it is the first question**: a list emptied
 * by a condition and a server with nothing to report are opposite answers, and telling a reader that
 * nothing has run slowly when their own filter hid it would send them looking at the wrong thing.
 */
function emptyTitle(pane: MonitorPane, narrowed: boolean, label: string): string {
  if (narrowed) return 'Nothing matches'
  // The charts pane draws no grid and so reaches none of this — it says its own kind of empty, one
  // panel at a time, because a server that counts sessions and not bytes is empty in one place only.
  if (pane === 'charts') return 'Nothing to show'
  if (pane === 'processes') return `No ${label.toLowerCase()}s to show`
  // "Nothing has run slowly" is a happy answer, and it is a different one from "this server is not
  // recording" — which the dialog says, having read the setting rather than the list.
  if (pane === 'slow') return 'Nothing slow has been recorded'
  return 'The server reported nothing'
}

/**
 * What the charts pane says it is showing: how much of the window has been collected, and whether it
 * is still collecting. It counts **readings** rather than minutes because that is the honest number —
 * a pane opened five minutes ago with the clock off holds one — and it names the ceiling, so a reader
 * watching a long stretch can see it is about to start losing the front of it.
 *
 * There are **two** ceilings and either will do, the readings having outlived the pane: a count, and
 * the two hours past which the front is dropped however few of them there are. The second is read
 * against the clock's own step, the front going when the *next* reading is more than that far past it.
 */
function watched(history: readonly Reading[], every: number): string {
  if (history.length === 0) return ''

  const from = history[0]?.readAt ?? 0
  const to = history[history.length - 1]?.readAt ?? 0
  const span = clock(from, to)

  const full = history.length === HISTORY || to - from + every * 1000 >= WINDOW
  const held = `${history.length} readings${full ? ' (the most it keeps)' : ''}`
  return every === 0 ? `${held} · ${span} · not refreshing` : `${held} · ${span} · every ${every}s`
}

/** How long the readings kept so far cover, written the way a person says a stretch of time. */
function clock(from: number, to: number): string {
  const seconds = Math.max(0, Math.round((to - from) / 1000))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
}

function footer(
  pane: MonitorPane,
  data: Report | null,
  shownRows: number,
  picked: number,
): ReactNode {
  if (data === null) return ''

  const held = data.rows.length
  const rows = shownRows === held ? `${held}` : `${shownRows} of ${held}`
  const noun = pane === 'processes' || pane === 'slow' ? 'rows' : 'entries'
  const pick = picked === 0 ? '' : ` · ${picked} picked`
  return `${rows} ${noun}${pick}`
}
