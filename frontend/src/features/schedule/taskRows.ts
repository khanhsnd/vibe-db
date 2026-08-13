import {
  cadenceText,
  momentText,
  TASK_NOUN,
  type Task,
  type TaskKind,
  type TaskRun,
} from '@/bridge/schedule'
import { namespaceLabel, type Profile } from '@/bridge/connections'
import type { GridData, ResultColumn } from '@/features/results/resultSet'

/**
 * The two grids of the Automation tab: the jobs, and one job's firings.
 *
 * The columns are the **app's own** rather than an engine's, exactly as a backup list's are: a job
 * on a clock is this machine's business and no catalogue has ever heard of one. Every moment and
 * every duration is turned into words here rather than on the Go side, because how either reads is
 * the reader's own locale and only this side knows it.
 */

const TASK_COLUMNS: ResultColumn[] = [
  { name: 'Name', type: 'text', fullType: 'text', width: 200 },
  { name: 'Job', type: 'text', fullType: 'text', width: 90 },
  { name: 'Status', type: 'text', fullType: 'text', width: 90 },
  { name: 'Schedule', type: 'text', fullType: 'text', width: 175 },
  { name: 'Connection', type: 'text', fullType: 'text', width: 150 },
  { name: 'Target', type: 'text', fullType: 'text', width: 170 },
  { name: 'Next Run', type: 'text', fullType: 'text', width: 145 },
  { name: 'Last Run', type: 'text', fullType: 'text', width: 145 },
  { name: 'Last Result', type: 'text', fullType: 'text', width: 320 },
]

const RUN_COLUMNS: ResultColumn[] = [
  { name: 'Ran At', type: 'text', fullType: 'text', width: 145 },
  { name: 'Result', type: 'text', fullType: 'text', width: 90 },
  { name: 'Elapsed', type: 'text', fullType: 'text', width: 90 },
  { name: 'Summary', type: 'text', fullType: 'text', width: 320 },
  { name: 'Detail', type: 'text', fullType: 'text', width: 460 },
]

export function taskGrid(tasks: Task[], profiles: Profile[]): GridData {
  return {
    columns: TASK_COLUMNS,
    rows: tasks.map((task) => [
      task.name,
      TASK_NOUN[task.kind as TaskKind] ?? task.kind,
      statusText(task),
      cadenceText(task),
      profiles.find((profile) => profile.id === task.profileId)?.name ?? '(deleted connection)',
      targetText(task),
      momentText(task.nextRunAt),
      momentText(task.lastRunAt),
      lastResultText(task),
    ]),
  }
}

export function runGrid(runs: TaskRun[]): GridData {
  return {
    columns: RUN_COLUMNS,
    rows: runs.map((run) => [
      momentText(run.at),
      outcomeText(run.outcome),
      elapsedText(run.elapsedMs),
      run.summary,
      run.detail,
    ]),
  }
}

/**
 * What a job is doing, in one word. `Running` wins over both the others, because it is the only one
 * of the three that will not be true in a minute — and it is what a reader who has just pressed Run
 * is looking for.
 */
function statusText(task: Task): string {
  if (task.running) return 'Running'
  return task.enabled ? 'On' : 'Off'
}

/** Where the job points: the namespace, and the pair of tables where it compares two. */
function targetText(task: Task): string {
  const where = namespaceLabel({ database: task.database, schema: task.schema })
  if (task.kind !== 'compare') return where

  return `${where}.${task.table} ⇄ ${task.right.table}`
}

/** The last firing in one line: what it came to, and what it wrote. */
function lastResultText(task: Task): string {
  const last = task.runs[0]
  if (!last) return 'Never run'

  const detail = last.detail === '' ? last.summary : `${last.summary}: ${last.detail}`
  return `${outcomeText(last.outcome)} · ${detail}`
}

/**
 * A firing's outcome as a word. `Differs` is deliberately not `Failed`: a comparison that found two
 * sides apart did exactly what it was asked to — see `schedule.Outcome`.
 */
export function outcomeText(outcome: string): string {
  switch (outcome) {
    case 'ok':
      return 'OK'
    case 'differs':
      return 'Differs'
    case 'failed':
      return 'Failed'
    default:
      return outcome
  }
}

/** `1.4 s`, or `840 ms` for a job that was over before a second passed. */
export function elapsedText(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`

  const minutes = Math.floor(ms / 60_000)
  const seconds = Math.round((ms % 60_000) / 1000)
  return `${minutes} m ${seconds} s`
}
