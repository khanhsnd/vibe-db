import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

export type Profile = app.Profile
export type ProfileInput = app.ProfileInput
export type Session = app.Session
export type TestResult = app.TestResult
export type TableSchema = app.TableSchema

export type TLSMode = 'disabled' | 'preferred' | 'required' | 'verify-ca'

export const TLS_MODES: ReadonlyArray<{ value: TLSMode; label: string }> = [
  { value: 'disabled', label: 'Disabled' },
  { value: 'preferred', label: 'Preferred' },
  { value: 'required', label: 'Required' },
  { value: 'verify-ca', label: 'Verify CA' },
]

export type Engine = 'mysql' | 'postgres'

export interface EngineInfo {
  value: Engine
  label: string
  port: number
  user: string
  // PostgreSQL binds a session to one database for its whole life, so a profile that names
  // none has nothing to connect to. MySQL can be browsed without one.
  needsDatabase: boolean
  // What the tree and the query toolbar are listing under a connection: MySQL qualifies a
  // table with a database, PostgreSQL with a schema of the one database it is bound to.
  namespace: string
}

// The order here is the order of the dialog's Engine list, and the first entry is what an
// unnamed engine falls back to — the same default the Go side applies.
export const ENGINES = [
  {
    value: 'mysql',
    label: 'MySQL',
    port: 3306,
    user: 'root',
    needsDatabase: false,
    namespace: 'Database',
  },
  {
    value: 'postgres',
    label: 'PostgreSQL',
    port: 5432,
    user: 'postgres',
    needsDatabase: true,
    namespace: 'Schema',
  },
] as const satisfies readonly EngineInfo[]

export function engineInfo(engine: string): EngineInfo {
  return ENGINES.find((info) => info.value === engine) ?? ENGINES[0]
}

// Switching engine brings the defaults of the new one with it, but only where the field
// still holds the old engine's default: a port or user that was typed is the user's.
export function withEngine(form: ProfileInput, engine: string): ProfileInput {
  const from = engineInfo(form.engine)
  const to = engineInfo(engine)

  return {
    ...form,
    engine: to.value,
    port: form.port === from.port ? to.port : form.port,
    user: form.user === from.user ? to.user : form.user,
  }
}

export function emptyProfileInput(): ProfileInput {
  const [{ value, port, user }] = ENGINES

  return {
    id: '',
    name: '',
    engine: value,
    host: '127.0.0.1',
    port,
    user,
    password: '',
    database: '',
    tlsMode: 'preferred',
    savePassword: true,
  }
}

export function toProfileInput(profile: Profile): ProfileInput {
  return {
    id: profile.id,
    name: profile.name,
    engine: profile.engine,
    host: profile.host,
    port: profile.port,
    user: profile.user,
    password: '',
    database: profile.database,
    tlsMode: profile.tlsMode,
    savePassword: profile.savePassword,
  }
}

export const connections = {
  listProfiles: () => call(() => api.ListProfiles()),
  saveProfile: (input: ProfileInput) => call(() => api.SaveProfile(input)),
  deleteProfile: (id: string) => call(() => api.DeleteProfile(id)),
  testConnection: (input: ProfileInput) => call(() => api.TestConnection(input)),
  connect: (profileId: string, password: string) => call(() => api.Connect(profileId, password)),
  disconnect: (sessionId: string) => call(() => api.Disconnect(sessionId)),
  listDatabases: (sessionId: string) => call(() => api.ListDatabases(sessionId)),
  listTables: (sessionId: string, database: string) =>
    call(() => api.ListTables(sessionId, database)),
  loadSchema: (sessionId: string, database: string) =>
    call(() => api.LoadSchema(sessionId, database)),
}
