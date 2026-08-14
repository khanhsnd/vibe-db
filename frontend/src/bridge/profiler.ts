import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'
import type { TableColumn } from './tables'

/**
 * What one row of a slow list is **taken away** by, kept beside the row rather than read back out of
 * whichever column it landed in — `ProcessRef`'s arrangement, and for the same reason.
 *
 * `runnable` is the recorded statement written out as something that would run *somewhere else*: in
 * another client, in a script, in a colleague's terminal. It is not the cell's text. What has to be
 * put around that text is the engine's own knowledge — a MySQL row arrives with its `USE` in front,
 * a SQL Server one with `GO` after it, a MongoDB one wrapped in `getSiblingDB` — so it is spelled by
 * the driver, like every other statement in this app.
 *
 * `complete` says it would run as it stands. Two engines record only what they normalized, and for
 * those the text opens with comment lines saying which placeholders to fill in. It is the driver's
 * own answer about the catalogue it read and is never guessed from the text here.
 */
export interface SlowQueryRef {
  runnable: string
  complete: boolean
}

/**
 * One row per statement the server recorded as slow, in whatever columns this engine's catalogue
 * keeps — the arrangement `ProcessList` has, and for the reason it has one: MySQL records a start
 * time and a lock time, PostgreSQL a call count and a mean, Redis microseconds and the arguments it
 * was given.
 *
 * `refs` runs parallel to `rows`, so nothing here reads a statement back out of a column and nothing
 * depends on which column it landed in.
 */
export interface SlowQueries {
  columns: TableColumn[]
  rows: (string | null)[][]
  refs: SlowQueryRef[]
}

/** One thing an engine says about its own recording that no other engine has got. */
export interface ProfileFact {
  label: string
  value: string
}

/**
 * How the recording is currently set, **read from the server** rather than remembered from the last
 * change this app made: another client may have moved it, and a switch drawn from what this app did
 * last would be a switch that lies.
 *
 * `ready` false with a `reason` is machinery that exists but has not been set up — a PostgreSQL
 * server without the extension installed. It is a different answer from a list holding nothing,
 * which means the happier "nothing has run slowly", and the two are never conflated.
 */
export interface ProfileConfig {
  enabled: boolean
  thresholdMs: number
  configurable: boolean
  ready: boolean
  reason: string
  detail: ProfileFact[]
}

/**
 * What a change does to the recording. `enable` also carries a new threshold — an engine is told
 * "record anything slower than this" in one breath — and `reset` throws away what was recorded.
 *
 * Not every engine has all three, and one it has not got answers `UNSUPPORTED` naming it rather than
 * doing a near enough thing instead — see `EngineInfo.profilerConfigurable`.
 */
export type ProfileAction = 'enable' | 'disable' | 'reset'

export interface ProfileChange {
  action: ProfileAction
  thresholdMs: number
}

/**
 * Exactly what the changes will run, and whether the reader has to answer for it first. There is no
 * Changes pane here, so this **is** what the confirmation shows.
 *
 * `confirm` is a `domain` rule already applied against this connection's environment — see
 * `domain.ProfileHazard`. Nothing here re-derives it from `hazard`.
 */
export interface ProfilePreview {
  statements: string[]
  hazard: string
  confirm: boolean
}

/** A change of one setting, which is what every call site here builds. */
export function profileChange(action: ProfileAction, thresholdMs = 0): ProfileChange {
  return { action, thresholdMs }
}

export function loadSlowQueries(
  sessionId: string,
  ns: Namespace,
  limit: number,
): Promise<SlowQueries> {
  return call(() => api.LoadSlowQueries(sessionId, ns, limit)) as Promise<SlowQueries>
}

export function loadProfileConfig(sessionId: string, ns: Namespace): Promise<ProfileConfig> {
  return call(() => api.LoadProfileConfig(sessionId, ns)) as Promise<ProfileConfig>
}

export function previewProfileChanges(
  sessionId: string,
  changes: ProfileChange[],
): Promise<ProfilePreview> {
  return call(() => api.PreviewProfileChanges(sessionId, wire(changes))) as Promise<ProfilePreview>
}

export function applyProfileChanges(
  sessionId: string,
  ns: Namespace,
  changes: ProfileChange[],
): Promise<void> {
  return call(() => api.ApplyProfileChanges(sessionId, ns, wire(changes)))
}

/**
 * Wails generates a *class* for a wire type holding a nested struct, so an object literal can never
 * satisfy it — the same boundary cast every other module here makes, and for the same reason.
 */
const wire = (changes: ProfileChange[]) => changes as unknown as app.ProfileChangeInput[]
