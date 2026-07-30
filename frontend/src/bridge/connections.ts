import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

export type Profile = app.Profile
export type ProfileInput = app.ProfileInput
export type Session = app.Session
export type TestResult = app.TestResult
export type TableSchema = app.TableSchema

/**
 * Where a table lives. `schema` is empty on MySQL, whose database is its schema, and on a
 * PostgreSQL database node, which names no schema yet and so addresses no tables.
 */
export type Namespace = app.Namespace

export const NO_NAMESPACE: Namespace = { database: '', schema: '' }

export function namespaceOf(database: string, schema = ''): Namespace {
  return { database, schema }
}

/** How a namespace reads to a person: `shop`, or `shop.public` where there is a schema. */
export function namespaceLabel(ns: Namespace): string {
  return ns.schema === '' ? ns.database : `${ns.database}.${ns.schema}`
}

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
  // Whether a table is qualified with a schema inside its database. MySQL's database *is*
  // its schema, so its tree has one level fewer.
  hasSchemas: boolean
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
    hasSchemas: false,
  },
  {
    value: 'postgres',
    label: 'PostgreSQL',
    port: 5432,
    user: 'postgres',
    needsDatabase: true,
    hasSchemas: true,
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
  listSchemas: (sessionId: string, database: string) =>
    call(() => api.ListSchemas(sessionId, database)),
  listTables: (sessionId: string, ns: Namespace) => call(() => api.ListTables(sessionId, ns)),
  loadTableSchemas: (sessionId: string, ns: Namespace) =>
    call(() => api.LoadTableSchemas(sessionId, ns)),
}
