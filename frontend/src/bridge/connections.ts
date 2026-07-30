import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

export type Profile = app.Profile
export type ProfileInput = app.ProfileInput
export type Session = app.Session
export type TestResult = app.TestResult

export type TLSMode = 'disabled' | 'preferred' | 'required' | 'verify-ca'

export const TLS_MODES: ReadonlyArray<{ value: TLSMode; label: string }> = [
  { value: 'disabled', label: 'Disabled' },
  { value: 'preferred', label: 'Preferred' },
  { value: 'required', label: 'Required' },
  { value: 'verify-ca', label: 'Verify CA' },
]

export const DEFAULT_MYSQL_PORT = 3306

export function emptyProfileInput(): ProfileInput {
  return {
    id: '',
    name: '',
    host: '127.0.0.1',
    port: DEFAULT_MYSQL_PORT,
    user: 'root',
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
}
