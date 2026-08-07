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
  /** The tint the sidebar washes this connection and everything under it with. */
  colour: string
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
  colour: string
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
 * The colour a connection is marked with, which is the one thing on a profile no server ever hears
 * about: it is how a reader tells two connections apart when the names are alike and the
 * environments are the same. The palette is closed — see `domain.Colour`, which this mirrors —
 * because a colour picked off a wheel is a colour that reads on one theme and vanishes on the other.
 *
 * `''` is a connection that has not picked one, and it wears the sidebar's own background.
 */
export type ConnectionColour =
  | ''
  | 'red'
  | 'orange'
  | 'yellow'
  | 'green'
  | 'teal'
  | 'blue'
  | 'purple'
  | 'pink'

export interface ColourInfo {
  value: ConnectionColour
  label: string
  /** The token behind the colour, and `''` for none. Never a hex — see `shared/theme/tokens.css`. */
  colour: string
}

export const CONNECTION_COLOURS = [
  { value: '', label: 'No Colour', colour: '' },
  { value: 'red', label: 'Red', colour: 'var(--conn-red)' },
  { value: 'orange', label: 'Orange', colour: 'var(--conn-orange)' },
  { value: 'yellow', label: 'Yellow', colour: 'var(--conn-yellow)' },
  { value: 'green', label: 'Green', colour: 'var(--conn-green)' },
  { value: 'teal', label: 'Teal', colour: 'var(--conn-teal)' },
  { value: 'blue', label: 'Blue', colour: 'var(--conn-blue)' },
  { value: 'purple', label: 'Purple', colour: 'var(--conn-purple)' },
  { value: 'pink', label: 'Pink', colour: 'var(--conn-pink)' },
] as const satisfies readonly ColourInfo[]

export function colourInfo(colour: string): ColourInfo {
  return CONNECTION_COLOURS.find((info) => info.value === colour) ?? CONNECTION_COLOURS[0]
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

export type Engine = 'mysql' | 'postgres' | 'mongo' | 'redis'

export interface EngineInfo {
  value: Engine
  label: string
  port: number
  user: string
  // Whether every connection to this engine is made *as* somebody. A MongoDB server started
  // without access control and a Redis server with no ACL both take no user name at all, which is
  // what greys the box rather than refusing the profile.
  needsUser: boolean
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
  // Whether this engine stores documents rather than rows. It is the one engine fact the UI has
  // to know about without asking a driver, because it changes what the *words* mean: a table is a
  // collection, a field is not declared anywhere, and the editor is not writing SQL.
  documents: boolean
  // Whether this engine stores values under keys and nothing else. A Redis database holds one flat
  // key space rather than tables, so what the reader browses is that key space, and the editor is
  // writing commands rather than either SQL or a shell.
  keys: boolean
  // Whether a database of this engine is something the app can make and drop. A Redis server is
  // started with a fixed number of them and they all exist from that moment, so there is nothing
  // for a New Database dialog to make and nothing for a delete to remove.
  createsDatabases: boolean
  // Whether a namespace's tables are the reader's to declare. A Redis database holds exactly one
  // key space and always has, so there is nothing to create, rename, duplicate, design or drop.
  createsTables: boolean
  // Whether this engine has views at all, which is what decides if the tree draws a folder that
  // could only ever be empty.
  hasViews: boolean
  // Whether a new database of this engine has to be given its first collection. MongoDB has no
  // CREATE DATABASE — a database exists exactly as long as it holds a collection — so making one
  // is making that collection, and the dialog asks which.
  databaseNeedsCollection: boolean
  // Whether a table of this engine can carry referential constraints. MongoDB has none of any
  // kind, so the designer does not offer a tab for declaring one it could never enforce.
  hasForeignKeys: boolean
  // Whether this engine has stored functions and procedures at all. Server-side JavaScript was
  // removed from MongoDB, so there is no folder for the tree to draw.
  hasRoutines: boolean
  // Whether emptying and truncating are two different things. They are on a table store, where one
  // deletes the rows and the other resets the storage; a collection has no storage to reset, so
  // offering both would be offering the same command twice under two names.
  hasTruncate: boolean
  // Whether a database of this engine can be backed up. A backup here is a file of the statements
  // that would put the database back, so the engines that have one are the engines whose statements
  // declare a table and write a row — see `domain.Engine.Backups`, which is where the rule lives.
  hasBackups: boolean
}

/**
 * What this engine calls the things the app shows. A collection is not a table and a document is not
 * a row, and a menu that said otherwise would be telling the reader something untrue about the
 * server they are looking at — the truncate wording especially, which on a table store warns about
 * a foreign key that a document store cannot have.
 *
 * It is words alone. **Nothing here changes what anything does**: a `NodeKind` is still `tables` and
 * a bound method is still `DropTable`, so one name means one thing on the wire whatever it is called
 * on the screen.
 */
export interface EngineWords {
  /** Title case, for a menu item or a dialog's title: `Delete Collection`. */
  table: string
  /** Title case and plural, for the tree's own folder. */
  tables: string
  /** Lower case, for the middle of a sentence. */
  row: string
  rows: string
  /** A file of statements, for the item that runs one: a document store holds no SQL. */
  scriptFile: string
}

const SQL_WORDS: EngineWords = {
  table: 'Table',
  tables: 'Tables',
  row: 'row',
  rows: 'rows',
  scriptFile: 'SQL File',
}

const DOCUMENT_WORDS: EngineWords = {
  table: 'Collection',
  tables: 'Collections',
  row: 'document',
  rows: 'documents',
  scriptFile: 'Script File',
}

/**
 * A key-value store has one table to a database and it is not called a table. `Data` is the folder
 * every other Redis client draws, and a row in it is a *key* — which is the word that matters most
 * here, since it is also what identifies one.
 */
const KEY_WORDS: EngineWords = {
  table: 'Key Space',
  tables: 'Data',
  row: 'key',
  rows: 'keys',
  scriptFile: 'Command File',
}

export function engineWords(engine: string): EngineWords {
  const info = engineInfo(engine)
  if (info.keys) return KEY_WORDS
  return info.documents ? DOCUMENT_WORDS : SQL_WORDS
}

// The order here is the order of the dialog's Engine list, and the first entry is what an
// unnamed engine falls back to — the same default the Go side applies.
export const ENGINES = [
  {
    value: 'mysql',
    label: 'MySQL',
    port: 3306,
    user: 'root',
    needsUser: true,
    needsDatabase: false,
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTruncate: true,
    hasBackups: true,
  },
  {
    value: 'postgres',
    label: 'PostgreSQL',
    port: 5432,
    user: 'postgres',
    needsUser: true,
    needsDatabase: true,
    hasSchemas: true,
    databaseHasOwner: true,
    documents: false,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTruncate: true,
    hasBackups: true,
  },
  {
    // A MongoDB server started without access control takes no credentials at all, which is why
    // this is the one engine whose default user is blank rather than an administrator's name.
    value: 'mongo',
    label: 'MongoDB',
    port: 27017,
    user: '',
    needsUser: false,
    needsDatabase: false,
    hasSchemas: false,
    databaseHasOwner: false,
    documents: true,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    databaseNeedsCollection: true,
    hasForeignKeys: false,
    hasRoutines: false,
    hasTruncate: false,
    hasBackups: false,
  },
  {
    // A Redis database is *numbered* rather than named — `0` to `15` on a default install — so the
    // Database box takes an index, and leaving it blank is database 0. Nothing else about this
    // engine is declared: one key space to a database, and no shape to any of it.
    value: 'redis',
    label: 'Redis',
    port: 6379,
    user: '',
    needsUser: false,
    needsDatabase: false,
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: true,
    createsDatabases: false,
    createsTables: false,
    hasViews: false,
    databaseNeedsCollection: false,
    hasForeignKeys: false,
    hasRoutines: false,
    hasTruncate: false,
    hasBackups: false,
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
    colour: '',
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
    colour: profile.colour,
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
