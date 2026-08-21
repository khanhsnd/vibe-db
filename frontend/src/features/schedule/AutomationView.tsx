import { useEffect, useMemo, useState } from 'react'
import { CalendarClock, Loader2, Pause, Play } from 'lucide-react'

import { defaultTask, onTaskRun, toTaskRequest, type Task } from '@/bridge/schedule'
import { NO_NAMESPACE } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import { focusOf, type Selection } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { useSchedulesStore } from '@/stores/schedules'
import { useSchemaStore } from '@/stores/schema'

import { runGrid, taskGrid } from './taskRows'

/**
 * Every job this app runs on a clock, and what the picked one has been doing — what
 * `Tools ▸ Automation` opens.
 *
 * The list above is a [`ResultGrid`](../../../../docs/grid.md) and so is the history below it, both
 * read-only: a job is changed in its own dialog, and a firing that has happened is not a thing to
 * type over. The toolbar acts on whichever row the selection names.
 *
 * It belongs to **no connection**, because a job names the connection it dials and the list holds
 * jobs on all of them — the arrangement the History Log has, and for the same reason.
 */
export function AutomationView() {
  const tasks = useSchedulesStore((s) => s.tasks)
  const loading = useSchedulesStore((s) => s.loading)
  const storeError = useSchedulesStore((s) => s.error)
  const load = useSchedulesStore((s) => s.load)
  const edit = useSchedulesStore((s) => s.edit)
  const remove = useSchedulesStore((s) => s.remove)
  const enable = useSchedulesStore((s) => s.enable)
  const run = useSchedulesStore((s) => s.run)

  const profiles = useConnectionsStore((s) => s.profiles)
  const sessions = useConnectionsStore((s) => s.sessions)
  const activeNamespace = useConnectionsStore((s) => s.activeNamespace)
  const ask = useSchemaStore((s) => s.ask)

  const [selection, setSelection] = useState<Selection | null>(null)
  const [failure, setFailure] = useState<AppError | null>(null)

  useEffect(() => {
    void load()
  }, [load])

  // A job fires when it fires, so the list follows the clock rather than waiting to be re-read: a
  // reader watching at two in the morning sees it start and sees it finish. See `app.EventTaskRun`.
  useEffect(() => onTaskRun(() => void load()), [load])

  const focus = focusOf(selection)
  const picked: Task | null = focus ? (tasks[focus.row] ?? null) : null

  const data = useMemo(() => taskGrid(tasks, profiles), [tasks, profiles])
  const runs = useMemo(() => runGrid(picked?.runs ?? []), [picked])

  const act = (work: () => Promise<void>) => () => {
    setFailure(null)
    void work().catch((thrown: unknown) => setFailure(toAppError(thrown)))
  }

  /**
   * A new job starts on wherever the reader already is — the first open connection and the
   * namespace selected on it — for the reason `New Query` does. Where nothing is open it starts on
   * the first connection there is, and the dialog's own pickers are what move it.
   */
  const newTask = () => {
    const profileId = Object.keys(sessions)[0] ?? profiles[0]?.id ?? ''
    setFailure(null)
    edit(defaultTask(profileId, activeNamespace[profileId] ?? NO_NAMESPACE))
  }

  const askDelete = (task: Task) =>
    ask({
      kind: 'confirm',
      title: 'Delete Job',
      message:
        `Delete “${task.name}”? It stops firing, and what it has already written stays where ` +
        `it is — the backups it took are still in the backup folder.`,
      submitLabel: 'Delete',
      danger: true,
      onSubmit: async () => {
        await remove(task.id)
        setSelection(null)
      },
    })

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button variant="subtle" icon={ICON.add} onClick={newTask}>
          New Job
        </Button>
        <Button
          variant="subtle"
          icon={ICON.editConnection}
          disabled={picked === null}
          onClick={() => {
            if (picked) edit(toTaskRequest(picked))
          }}
        >
          Edit
        </Button>

        <span className="mx-1 h-4 w-px bg-line" />

        {/* Run is the same call the clock makes, which is the whole point of it: a nightly backup
            set up this afternoon is one the reader can watch work this afternoon. */}
        <Button
          variant="subtle"
          icon={
            picked?.running ? (
              <Loader2 size={13} className="animate-spin" />
            ) : (
              <Play size={13} fill="currentColor" />
            )
          }
          disabled={picked === null || picked.running}
          onClick={act(async () => {
            if (picked) await run(picked.id)
          })}
        >
          Run Now
        </Button>
        <Button
          variant="subtle"
          icon={picked?.enabled ? <Pause size={13} /> : <Play size={13} />}
          disabled={picked === null}
          onClick={act(async () => {
            if (picked) await enable(picked.id, !picked.enabled)
          })}
        >
          {picked?.enabled ? 'Disable' : 'Enable'}
        </Button>

        <span className="mx-1 h-4 w-px bg-line" />

        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={picked === null}
          onClick={() => {
            setFailure(null)
            if (picked) askDelete(picked)
          }}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete
        </Button>
        <Button variant="subtle" icon={ICON.refresh} onClick={() => void load()}>
          Refresh
        </Button>

        {/* The one thing about a schedule a reader has to know, said where they cannot miss it. */}
        <span className="ml-auto pr-1 text-2xs text-faint">
          Jobs fire only while Vibe DB is running
        </span>
      </div>

      <div className="min-h-0 flex-1">
        {loading && tasks.length === 0 ? (
          <Loading label="Reading the schedule…" />
        ) : tasks.length === 0 ? (
          <Empty
            icon={<CalendarClock size={26} strokeWidth={1.5} />}
            title="Nothing is scheduled"
            hint="New Job backs a namespace up, exports it, dumps it with the engine's own program, or watches two tables for drift."
            action={
              <Button variant="primary" onClick={newTask}>
                New Job
              </Button>
            }
          />
        ) : (
          <Split
            id="vibe-db.automation"
            orientation="vertical"
            panelIds={['jobs', 'runs']}
            className="h-full"
          >
            <SplitPanel id="jobs" defaultSize={55} minSize={20}>
              <ResultGrid
                result={data}
                selection={selection}
                onSelectionChange={setSelection}
                onActivateRow={(row) => {
                  const task = tasks[row]
                  if (task) edit(toTaskRequest(task))
                }}
                onRefresh={() => void load()}
              />
            </SplitPanel>

            <SplitHandle orientation="vertical" />

            <SplitPanel id="runs" minSize={15}>
              {picked === null ? (
                <Empty title="No job picked" hint="Pick a row to read what it has been doing." />
              ) : picked.runs.length === 0 ? (
                <Empty
                  title={`${picked.name} has not run yet`}
                  hint="Run Now fires it exactly as the clock would."
                />
              ) : (
                <ResultGrid result={runs} />
              )}
            </SplitPanel>
          </Split>
        )}
      </div>

      {failure !== null ? <ErrorStrip error={failure} /> : null}
      {failure === null && storeError !== null ? <ErrorStrip error={storeError} /> : null}

      <GridFooter editable={false} busy={loading} onRefresh={() => void load()}>
        <span>{tasks.length === 1 ? '1 job' : `${tasks.length} jobs`}</span>
      </GridFooter>
    </div>
  )
}
