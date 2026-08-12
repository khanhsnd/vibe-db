import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'
import type { TableColumn } from './tables'

/**
 * One thing a server is doing, named. The two fields beside the id are each one engine's way of
 * saying *which*: `serial` is Oracle's, where a session **is** the pair `sid,serial#` — a sid is
 * handed to the next session the moment this one ends — and `own` is not the server's fact at all.
 *
 * `own` says the row is a connection **this app is reading on**. It is the driver's mark and never
 * derived here: only the adapter can ask a server which connection is asking, and what it answers is
 * "one of ours" rather than "the only one", a session holding a small pool.
 */
export interface ProcessRef {
  id: string
  serial: string
  own: boolean
}

/**
 * One row per thing the server is doing, in whatever columns this engine's catalogue keeps — the
 * arrangement the account list has, and for the same reason: no two engines report the same facts
 * about a connection.
 *
 * `refs` runs parallel to `rows` and is what a row is *acted on* by, so nothing here reads an id back
 * out of a column and nothing depends on which column it landed in.
 */
export interface ProcessList {
  columns: TableColumn[]
  rows: (string | null)[][]
  refs: ProcessRef[]
}

/**
 * What a kill does. `cancel` stops the statement and leaves the connection open; `close` ends the
 * connection, and with it whatever transaction was on it.
 *
 * Not every engine has both — see `EngineInfo.cancelsStatement` and `closesConnection` — and one it
 * has not got answers `UNSUPPORTED` naming it rather than doing the other one instead.
 */
export type ProcessAction = 'cancel' | 'close'

export interface ProcessKill {
  process: ProcessRef
  action: ProcessAction
}

/**
 * Exactly what the kills will run, and whether the reader has to answer for it first. There is no
 * Changes pane here, so this **is** what the confirmation shows.
 *
 * `confirm` is a `domain` rule already applied against this connection's environment — see
 * `domain.ProcessHazard`. Nothing here re-derives it from `hazard`.
 */
export interface ProcessPreview {
  statements: string[]
  hazard: string
  confirm: boolean
}

/**
 * What a server says about itself: one row per setting it was started with, or per number it has been
 * counting. Both halves of the monitor answer with this shape, and the columns are the engine's own —
 * MySQL's status is a name and a value, PostgreSQL's settings carry a unit and a category, and Redis's
 * `INFO` carries the section it groups them under.
 */
export interface ServerFacts {
  columns: TableColumn[]
  rows: (string | null)[][]
}

/** A kill of one process, which is what every call site here builds. */
export function processKill(process: ProcessRef, action: ProcessAction): ProcessKill {
  return { process, action }
}

export function listProcesses(sessionId: string): Promise<ProcessList> {
  return call(() => api.ListProcesses(sessionId)) as Promise<ProcessList>
}

export function previewProcessKills(
  sessionId: string,
  kills: ProcessKill[],
): Promise<ProcessPreview> {
  return call(() => api.PreviewProcessKills(sessionId, wire(kills))) as Promise<ProcessPreview>
}

export function killProcesses(sessionId: string, kills: ProcessKill[]): Promise<void> {
  return call(() => api.KillProcesses(sessionId, wire(kills)))
}

export function loadServerStatus(sessionId: string): Promise<ServerFacts> {
  return call(() => api.LoadServerStatus(sessionId)) as Promise<ServerFacts>
}

export function loadServerVariables(sessionId: string): Promise<ServerFacts> {
  return call(() => api.LoadServerVariables(sessionId)) as Promise<ServerFacts>
}

/**
 * Wails generates a *class* for a wire type holding a nested struct, so an object literal can never
 * satisfy it — the same boundary cast every other module here makes, and for the same reason.
 */
const wire = (kills: ProcessKill[]) => kills as unknown as app.ProcessKillInput[]
