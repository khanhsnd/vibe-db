import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

/**
 * A profile is spelled out here rather than taken from `app.Profile`, which is the same rule
 * `QueryRequest` follows: Wails generates a *class* for any wire type holding a nested struct —
 * with a `convertValues` method on it — and an object literal can then never satisfy it. The
 * shapes below are the same fields, and the two calls that send one cast at the boundary.
 */
export interface TLSSettings {
  mode: string
  rootCert: string
  clientCert: string
  clientKey: string
  serverName: string
}

export interface SSHSettings {
  enabled: boolean
  host: string
  port: number
  user: string
  auth: string
  keyPath: string
  knownHosts: string
  strictHostKey: boolean
}

export interface Profile {
  id: string
  name: string
  engine: string
  environment: string
  host: string
  port: number
  user: string
  database: string
  auth: string
  region: string
  tls: TLSSettings
  ssh: SSHSettings
  savePassword: boolean
  /** Whether the credential store already holds one — never the secret itself. */
  hasPassword: boolean
  hasSshSecret: boolean
}

export interface ProfileInput {
  id: string
  name: string
  engine: string
  environment: string
  host: string
  port: number
  user: string
  password: string
  database: string
  auth: string
  region: string
  tls: TLSSettings
  ssh: SSHSettings
  /** The bastion's password, or the pass phrase of its key. Blank keeps whatever is stored. */
  sshSecret: string
  savePassword: boolean
}

export type Session = app.Session
export type TestResult = app.TestResult
export type TableSchema = app.TableSchema

/** One stored function or procedure. It travels back into a drop unchanged — see `schema.dropRoutine`. */
export type Routine = app.RoutineInfo

/** Everything the tree draws a folder for under a namespace, read in one call. */
export type Objects = app.Objects

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

export type TLSMode = 'disabled' | 'preferred' | 'required' | 'verify-ca' | 'verify-full'

export const TLS_MODES: ReadonlyArray<{ value: TLSMode; label: string; hint: string }> = [
  { value: 'disabled', label: 'Disabled', hint: 'Plain text.' },
  { value: 'preferred', label: 'Preferred', hint: 'TLS where the server offers it, plain text where it does not.' },
  { value: 'required', label: 'Required', hint: 'Encrypted, and the certificate is not checked.' },
  { value: 'verify-ca', label: 'Verify CA', hint: 'The certificate chain is checked; the host name is not.' },
  { value: 'verify-full', label: 'Verify Full', hint: 'The chain and the host name are both checked.' },
]

/** Whether a mode opens a certificate file at all, which is what greys the file boxes out. */
export function tlsUsesFiles(mode: string): boolean {
  return mode === 'required' || mode === 'verify-ca' || mode === 'verify-full'
}

/**
 * What a server is *for*. It is the one fact about a connection that changes how the app behaves
 * rather than how it connects: every surface wearing the connection's name wears its colour, and
 * a statement that cannot be taken back is confirmed before it runs on a guarded one.
 *
 * `''` is a connection that has not said, which is every connection saved before this existed.
 */
export type Environment = '' | 'development' | 'staging' | 'production'

export interface EnvironmentInfo {
  value: Environment
  label: string
  /** The token behind the colour. Never a hex — see `shared/theme/tokens.css`. */
  colour: string
  /** Whether anything that writes is answered for before it runs here. Production alone is. */
  guarded: boolean
}

export const ENVIRONMENTS = [
  { value: '', label: 'None', colour: 'var(--env-none)', guarded: false },
  { value: 'development', label: 'Development', colour: 'var(--env-development)', guarded: false },
  { value: 'staging', label: 'Staging', colour: 'var(--env-staging)', guarded: false },
  { value: 'production', label: 'Production', colour: 'var(--env-production)', guarded: true },
] as const satisfies readonly EnvironmentInfo[]

export function environmentInfo(environment: string): EnvironmentInfo {
  return ENVIRONMENTS.find((info) => info.value === environment) ?? ENVIRONMENTS[0]
}

/**
 * How the *database* authenticates the reader. The two cloud methods mint a token per connection
 * from an identity the machine already holds, so neither has a password to type or to save.
 */
export type AuthMethod = 'password' | 'aws-iam' | 'google-iam'

export interface AuthInfo {
  value: AuthMethod
  label: string
  hint: string
  /** Whether the password is made at connect time rather than known, and so never stored. */
  minted: boolean
}

export const AUTH_METHODS = [
  { value: 'password', label: 'Password', hint: '', minted: false },
  {
    value: 'aws-iam',
    label: 'AWS IAM',
    hint:
      'The password is an RDS authentication token signed with this machine’s AWS credentials, ' +
      'good for 15 minutes. Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or run ' +
      '`aws configure export-credentials --format env`. TLS must be on.',
    minted: true,
  },
  {
    value: 'google-iam',
    label: 'Google Cloud IAM',
    hint:
      'The password is an OAuth access token from this machine’s application default credentials. ' +
      'Run `gcloud auth application-default login`, or point GOOGLE_APPLICATION_CREDENTIALS at a ' +
      'service-account key file.',
    minted: true,
  },
] as const satisfies readonly AuthInfo[]

export function authInfo(auth: string): AuthInfo {
  return AUTH_METHODS.find((info) => info.value === auth) ?? AUTH_METHODS[0]
}

/** How the *bastion* authenticates the reader, for a connection carried over SSH. */
export type SSHAuth = 'key' | 'password' | 'agent'

export const SSH_AUTHS: ReadonlyArray<{ value: SSHAuth; label: string }> = [
  { value: 'key', label: 'Private Key' },
  { value: 'password', label: 'Password' },
  { value: 'agent', label: 'SSH Agent' },
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
  // Whether a database of this engine is an object in its own right — with an owner, a
  // template, a locale, a tablespace and a connection limit — rather than a namespace with a
  // character set. It is what decides how much of the New Database dialog there is to fill in,
  // and whether that database can carry extensions and a comment.
  databaseHasOwner: boolean
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
    databaseHasOwner: false,
  },
  {
    value: 'postgres',
    label: 'PostgreSQL',
    port: 5432,
    user: 'postgres',
    needsDatabase: true,
    hasSchemas: true,
    databaseHasOwner: true,
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
    environment: '',
    host: '127.0.0.1',
    port,
    user,
    password: '',
    database: '',
    auth: 'password',
    region: '',
    tls: { mode: 'preferred', rootCert: '', clientCert: '', clientKey: '', serverName: '' },
    // A tunnel starts off, and starts *strict*: a bastion that cannot be identified protects the
    // credentials it carries from nobody, so turning that off has to be a deliberate act.
    ssh: {
      enabled: false,
      host: '',
      port: 22,
      user: '',
      auth: 'key',
      keyPath: '',
      knownHosts: '',
      strictHostKey: true,
    },
    sshSecret: '',
    savePassword: true,
  }
}

export function toProfileInput(profile: Profile): ProfileInput {
  return {
    id: profile.id,
    name: profile.name,
    engine: profile.engine,
    environment: profile.environment,
    host: profile.host,
    port: profile.port,
    user: profile.user,
    // Neither secret ever comes back out of the store — see `hasPassword` / `hasSshSecret`, which
    // are how the dialog says one is already kept without showing it.
    password: '',
    database: profile.database,
    auth: profile.auth,
    region: profile.region,
    tls: { ...profile.tls },
    ssh: { ...profile.ssh },
    sshSecret: '',
    savePassword: profile.savePassword,
  }
}

/** The one place a profile crosses into the generated bindings — see the note on ProfileInput. */
const wire = (input: ProfileInput) => input as unknown as app.ProfileInput

export const connections = {
  listProfiles: (): Promise<Profile[]> =>
    call(() => api.ListProfiles()) as unknown as Promise<Profile[]>,
  saveProfile: (input: ProfileInput): Promise<Profile> =>
    call(() => api.SaveProfile(wire(input))) as unknown as Promise<Profile>,
  deleteProfile: (id: string) => call(() => api.DeleteProfile(id)),
  testConnection: (input: ProfileInput) => call(() => api.TestConnection(wire(input))),
  connect: (profileId: string, password: string) => call(() => api.Connect(profileId, password)),
  disconnect: (sessionId: string) => call(() => api.Disconnect(sessionId)),
  listDatabases: (sessionId: string) => call(() => api.ListDatabases(sessionId)),
  listSchemas: (sessionId: string, database: string) =>
    call(() => api.ListSchemas(sessionId, database)),
  loadObjects: (sessionId: string, ns: Namespace) => call(() => api.LoadObjects(sessionId, ns)),
  loadTableSchemas: (sessionId: string, ns: Namespace) =>
    call(() => api.LoadTableSchemas(sessionId, ns)),
}
