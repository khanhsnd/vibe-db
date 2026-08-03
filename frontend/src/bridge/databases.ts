import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

/** What the New Database dialog fills its lists from. An engine answers `[]` for a list it has no concept of. */
export type DatabaseOptions = app.DatabaseOptions

/**
 * A database as the dialog declares it. Every engine's fields are here because one dialog fills
 * it for both; which of them reach the statement is the driver's decision, not this side's.
 */
export type DatabaseSpec = app.DatabaseSpecInput

export const NO_DATABASE_OPTIONS: DatabaseOptions = {
  owners: [],
  templates: [],
  encodings: [],
  collations: [],
  tablespaces: [],
  extensions: [],
} as DatabaseOptions

/** `-1` is PostgreSQL's own "no limit", which is what the box shows before anyone touches it. */
export const NO_CONNECTION_LIMIT = -1

export function emptyDatabaseSpec(): DatabaseSpec {
  return {
    name: '',
    owner: '',
    template: '',
    encoding: '',
    collation: '',
    ctype: '',
    tablespace: '',
    connectionLimit: NO_CONNECTION_LIMIT,
    allowConnections: true,
    isTemplate: false,
    comment: '',
    extensions: [],
  } as DatabaseSpec
}

export const databases = {
  options: (sessionId: string): Promise<DatabaseOptions> =>
    call(() => api.LoadDatabaseOptions(sessionId)),

  /** The statements `create` would run, which is what the dialog's SQL Preview shows. */
  preview: (sessionId: string, spec: DatabaseSpec): Promise<string[]> =>
    call(() => api.PreviewCreateDatabase(sessionId, spec)),

  create: (sessionId: string, spec: DatabaseSpec): Promise<void> =>
    call(() => api.CreateDatabase(sessionId, spec)),

  drop: (sessionId: string, name: string): Promise<void> =>
    call(() => api.DropDatabase(sessionId, name)),
}
