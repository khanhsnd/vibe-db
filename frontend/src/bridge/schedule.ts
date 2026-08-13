import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'
import { EventsOn } from '@wailsjs/runtime/runtime'

import type { Namespace } from './connections'
import { call } from './errors'

/**
 * The jobs this app runs on a clock: a backup, an export, one of the engine's own dump programs, or
 * a comparison that watches two tables for drift.
 *
 * **A task is not a fifth way of doing any of those.** What it fires is the very same bound method
 * the reader's own button calls, which is why a scheduled backup writes exactly the file `New
 * Backup` would have written — see `internal/app/schedule_run.go`. Nothing here is put *back*: a
 * restore and an import are not scheduled, because a job that ran while nobody was looking is the
 * wrong place to find out a table was overwritten.
 *
 * The clock runs **only while the app is open**. This is a desktop client and not a service, which
 * is what `catchUp` is the reader's answer to.
 */

export type Task = app.Task
export type TaskRun = app.TaskRun

/** What a job does when it fires. The list is closed and the Go side refuses anything else. */
export type TaskKind = 'backup' | 'export' | 'dump' | 'compare'

/** How often it fires. `minutes` counts a gap; the other three name a moment. */
export type TaskEvery = 'minutes' | 'hourly' | 'daily' | 'weekly'

/** What one firing came to. `differs` is not a failure — it is what a comparison was asked to find. */
export type TaskOutcome = 'ok' | 'failed' | 'differs'

/** The far side of a scheduled comparison, which carries a connection of its own. */
export interface TaskTarget {
  profileId: string
  database: string
  schema: string
  table: string
}

/**
 * One job as the dialog fills it in. Spelled out here rather than taken from `app.TaskInput` for the
 * reason `ProfileInput` is: Wails generates a class for a wire type, and an object literal can never
 * satisfy one.
 *
 * Every kind's answers ride on one shape, because JSON carries no union — the same arrangement
 * `TableCriterionInput` has. What a kind does not read is kept rather than cleared, so switching a
 * job from an export to a backup and back finds the folder still there.
 */
export interface TaskRequest {
  id: string
  name: string
  kind: TaskKind
  enabled: boolean
  /** Whether a firing time that went by while the app was shut should still run. */
  catchUp: boolean
  profileId: string
  namespace: Namespace

  every: TaskEvery
  minutes: number
  hour: number
  minute: number
  /** Sunday is 0, as `time.Weekday` counts. */
  weekday: number

  /** What the job covers, and **empty is every table the namespace holds**. */
  tables: string[]
  withStructure: boolean
  withData: boolean
  comment: string
  /** How many backups to leave in this namespace's folder, oldest deleted first. 0 is all of them. */
  keep: number
  format: string
  folder: string
  maxRows: number
  compress: boolean

  table: string
  right: TaskTarget
}

/**
 * What a new job starts on: a nightly backup at two in the morning, of wherever the reader asked for
 * one, keeping everything. Two o'clock because that is when a database is quiet, and off rather than
 * on because a job the reader has not read yet should not fire tonight.
 */
export function defaultTask(profileId: string, namespace: Namespace): TaskRequest {
  return {
    id: '',
    name: '',
    kind: 'backup',
    enabled: true,
    catchUp: false,
    profileId,
    namespace,
    every: 'daily',
    minutes: 30,
    hour: 2,
    minute: 0,
    weekday: 1,
    tables: [],
    withStructure: true,
    withData: true,
    comment: '',
    keep: 0,
    format: 'csv',
    folder: '',
    maxRows: 0,
    compress: true,
    table: '',
    right: { profileId: '', database: '', schema: '', table: '' },
  }
}

/** A saved job read back into the shape the dialog edits. */
export function toTaskRequest(task: Task): TaskRequest {
  return {
    id: task.id,
    name: task.name,
    kind: task.kind as TaskKind,
    enabled: task.enabled,
    catchUp: task.catchUp,
    profileId: task.profileId,
    namespace: { database: task.database, schema: task.schema },
    every: task.every as TaskEvery,
    minutes: task.minutes,
    hour: task.hour,
    minute: task.minute,
    weekday: task.weekday,
    tables: [...task.tables],
    withStructure: task.withStructure,
    withData: task.withData,
    comment: task.comment,
    keep: task.keep,
    format: task.format,
    folder: task.folder,
    maxRows: task.maxRows,
    compress: task.compress,
    table: task.table,
    right: { ...task.right },
  }
}

const wire = <T>(value: unknown) => value as T

export const tasks = {
  /** Every job, each carrying when it fires next and what its last few firings came to. */
  list: (): Promise<Task[]> => call(() => api.ListTasks()),

  save: (request: TaskRequest): Promise<Task> =>
    call(() =>
      api.SaveTask(
        wire<app.TaskInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),

  remove: (id: string): Promise<void> => call(() => api.DeleteTask(id)),

  /** Turns one on or off. Turning one on **arms** it: it waits a whole cadence rather than firing. */
  enable: (id: string, enabled: boolean): Promise<Task> => call(() => api.EnableTask(id, enabled)),

  /**
   * Fires one now — the same call the clock makes, which is the whole point of the button: a reader
   * setting a nightly backup up finds out this evening whether it works.
   */
  run: (id: string): Promise<TaskRun> => call(() => api.RunTask(id)),
}

const RUN_EVENT = 'task:run'

interface TaskEvent {
  taskId: string
  running: boolean
  run: TaskRun
}

/**
 * Follows every job's firings and answers with how to stop following them.
 *
 * Pushed rather than polled, because a schedule fires when it fires: a reader with the list open at
 * two in the morning is exactly the one who wants to watch it happen. See `app.EventTaskRun`.
 */
export function onTaskRun(
  change: (taskId: string, running: boolean, run: TaskRun) => void,
): () => void {
  return EventsOn(RUN_EVENT, (event: TaskEvent) => {
    change(event.taskId, event.running, event.run)
  })
}

/** The days of the week, in the order a picker lists them, indexed as `time.Weekday` counts. */
export const WEEKDAYS = [
  'Sunday',
  'Monday',
  'Tuesday',
  'Wednesday',
  'Thursday',
  'Friday',
  'Saturday',
] as const

/**
 * How a cadence reads to a person — `Every day at 02:00`. It is written here rather than on the Go
 * side for the reason a file's size and a backup's date are: how a moment reads is the reader's own
 * locale, and only this side knows it.
 */
export function cadenceText(task: {
  every: string
  minutes: number
  hour: number
  minute: number
  weekday: number
}): string {
  const time = `${pad(task.hour)}:${pad(task.minute)}`

  switch (task.every) {
    case 'minutes':
      return task.minutes === 1 ? 'Every minute' : `Every ${task.minutes} minutes`
    case 'hourly':
      return `Every hour at :${pad(task.minute)}`
    case 'weekly':
      return `Every ${WEEKDAYS[task.weekday] ?? 'Sunday'} at ${time}`
    default:
      return `Every day at ${time}`
  }
}

/** What a kind is called wherever one is named to a person. */
export const TASK_NOUN: Record<TaskKind, string> = {
  backup: 'Backup',
  export: 'Export',
  dump: 'Dump',
  compare: 'Compare',
}

/** `2026-08-13 02:00` — sortable, unambiguous, and the same width on every row. */
export function momentText(iso: string): string {
  if (iso === '') return ''

  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return iso

  return (
    `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ` +
    `${pad(at.getHours())}:${pad(at.getMinutes())}`
  )
}

const pad = (value: number) => String(value).padStart(2, '0')
