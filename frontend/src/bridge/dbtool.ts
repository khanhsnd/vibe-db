import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'
import { EventsOn } from '@wailsjs/runtime/runtime'

import type { Namespace } from './connections'
import { call } from './errors'

/**
 * The engine's **own** dump and restore programs — mongodump and mongorestore, mysqldump and the
 * mysql client, pg_dump and pg_restore.
 *
 * It is not `backup.ts`. A backup is a file of statements this app spelled, kept on a shelf the app
 * owns, and restoring one is running a script. These write the *vendor's* format to a file the
 * reader names, and nothing in this app ever reads a byte of it: a MongoDB archive is BSON, every
 * field of every document, which is precisely what a script of ours could not carry.
 *
 * Neither is it `transfer.ts`, which carries a table to a CSV for something that is not a database
 * to read. These files go back into the same engine and nowhere else.
 */

export type Toolkit = app.Toolkit
export type ToolProgram = app.ToolProgram
export type ToolResult = app.ToolResult

/** Which of the pair a call means — the words `dbtool.DumpProgram` and `RestoreProgram` carry. */
export type ToolWhich = 'dump' | 'restore'

/** What a dump covers and how much of it. An empty `tables` is every table the namespace holds. */
export interface ToolDumpRequest {
  namespace: Namespace
  runId: string
  path: string
  tables: string[]
  withStructure: boolean
  withData: boolean
  compress: boolean
}

/** Which file to put back, and how. An empty `tables` is everything the file holds. */
export interface ToolRestoreRequest {
  namespace: Namespace
  runId: string
  path: string
  tables: string[]
  drop: boolean
  dryRun: boolean
  stopOnError: boolean
}

export function defaultDumpRequest(namespace: Namespace, runId: string): ToolDumpRequest {
  return {
    namespace,
    runId,
    path: '',
    tables: [],
    withStructure: true,
    withData: true,
    compress: true,
  }
}

/**
 * What a restore starts on. **Nothing is dropped and nothing is skipped**: a restore that removed
 * what was there by default would be a wrong click away from emptying a database, and the box that
 * does it is on the Advanced tab where it has room for the sentence that says so.
 */
export function defaultRestoreRequest(namespace: Namespace, runId: string): ToolRestoreRequest {
  return {
    namespace,
    runId,
    path: '',
    tables: [],
    drop: false,
    dryRun: false,
    stopOnError: true,
  }
}

const wire = <T>(value: unknown) => value as T

export const tools = {
  /** What this engine ships and whether this machine has it. Looked up fresh on every call. */
  toolkit: (sessionId: string): Promise<Toolkit> => call(() => api.LoadToolkit(sessionId)),

  /**
   * Asks for **the program file itself** and answers with the kit as it stands afterwards. Picking
   * one still finds the other where it is sitting beside it, which is why the dialog needs only one
   * trip for a pair that arrived as one download.
   */
  chooseProgram: (sessionId: string, which: ToolWhich): Promise<Toolkit> =>
    call(() => api.ChooseToolProgram(sessionId, which)),

  /** Drops the file that was picked, so this program goes back to whatever is on the PATH. */
  forgetProgram: (sessionId: string, which: ToolWhich): Promise<Toolkit> =>
    call(() => api.ForgetToolProgram(sessionId, which)),

  /** Answers with the path picked, or `''` for a dialog the reader dismissed. */
  chooseFile: (sessionId: string, saving: boolean, suggested: string): Promise<string> =>
    call(() => api.ChooseToolFile(sessionId, saving, suggested)),

  dump: (sessionId: string, request: ToolDumpRequest): Promise<ToolResult> =>
    call(() =>
      api.RunToolDump(
        sessionId,
        wire<app.ToolDumpInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),

  restore: (sessionId: string, request: ToolRestoreRequest): Promise<ToolResult> =>
    call(() =>
      api.RunToolRestore(
        sessionId,
        wire<app.ToolRestoreInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),

  /** Kills the program. An id that has already finished is ignored. */
  stop: (runId: string): Promise<void> => call(() => api.StopTool(runId)),
}

const LINE_EVENT = 'tool:line'

interface ToolLine {
  runId: string
  text: string
}

/**
 * Follows one run's output and answers with how to stop following it.
 *
 * The call itself comes back with the whole log as well, so nothing is lost by not listening — but
 * a dump of a hundred gigabytes says nothing for an hour if nobody is, and that is exactly the run
 * a reader stands and watches. See `app.EventToolLine`.
 */
export function onToolLine(runId: string, line: (text: string) => void): () => void {
  return EventsOn(LINE_EVENT, (printed: ToolLine) => {
    if (printed.runId === runId) line(printed.text)
  })
}
