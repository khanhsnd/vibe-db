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

/**
 * The outer hop: the SOCKS5 or HTTP proxy the connection is carried through, the bastion included.
 * Its password is not here — it travels in `ProfileInput.proxySecret` and never comes back out.
 */
export interface ProxySettings {
  enabled: boolean
  kind: string
  host: string
  port: number
  user: string
}

export interface Profile {
  id: string
  name: string
  engine: string
  environment: string
  /** The tint the sidebar washes this connection and everything under it with. */
  colour: string
  /** What this connection will be let do at all — see `ACCESS_MODES`. */
  access: string
  /** Whether an AI agent may read this connection through `vibe-db mcp` — never write to it. */
  mcp: boolean
  host: string
  port: number
  user: string
  database: string
  auth: string
  region: string
  tls: TLSSettings
  ssh: SSHSettings
  proxy: ProxySettings
  savePassword: boolean
  /** Whether the credential store already holds one — never the secret itself. */
  hasPassword: boolean
  hasSshSecret: boolean
  hasProxySecret: boolean
}

export interface ProfileInput {
  id: string
  name: string
  engine: string
  environment: string
  colour: string
  access: string
  mcp: boolean
  host: string
  port: number
  user: string
  password: string
  database: string
  auth: string
  region: string
  tls: TLSSettings
  ssh: SSHSettings
  proxy: ProxySettings
  /** The bastion's password, or the pass phrase of its key. Blank keeps whatever is stored. */
  sshSecret: string
  /** The proxy's own password. Blank keeps whatever is stored, the same rule. */
  proxySecret: string
  savePassword: boolean
}

export type Session = app.Session
export type TestResult = app.TestResult
export type TableSchema = app.TableSchema

/** One stored function or procedure, as the tree lists it. */
export type Routine = app.RoutineInfo

/** One trigger, named beside the table it fires on — which is half of what names one. */
export type Trigger = app.TriggerInfo

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

/**
 * Which protocol the proxy speaks. There is no SOCKS4: it cannot carry a name, so every host would
 * be resolved on *this* machine — which is the thing a proxy in front of a private network exists to
 * avoid. The port beside each is what the dialog fills in when the kind is picked, since one default
 * would be wrong half the time.
 */
export type ProxyKind = 'socks5' | 'http'

export interface ProxyKindInfo {
  value: ProxyKind
  label: string
  /** The port this kind is usually on, which is what the dialog fills in when it is picked. */
  port: number
}

export const PROXY_KINDS = [
  { value: 'socks5', label: 'SOCKS5', port: 1080 },
  { value: 'http', label: 'HTTP CONNECT', port: 8080 },
] as const satisfies readonly ProxyKindInfo[]

export function proxyPort(kind: string): number {
  return PROXY_KINDS.find((one) => one.value === kind)?.port ?? PROXY_KINDS[0].port
}

/**
 * What a connection is let do at all. It is the second fact about a profile that changes how the app
 * *behaves* rather than how it connects, and it is the harder of the two: an environment turns a
 * hazard into a **question**, and this turns one into a **refusal**.
 *
 * It mirrors `domain.Access`, which is where the rule lives. Nothing here decides what is refused —
 * a session says whether it is read-only and the Go side says what it held back.
 */
export type Access = '' | 'read-only'

export interface AccessInfo {
  value: Access
  label: string
  hint: string
}

export const ACCESS_MODES = [
  { value: '', label: 'Read & write', hint: 'Nothing is held back.' },
  {
    value: 'read-only',
    label: 'Read-only',
    hint:
      'Anything that would write is refused before it is sent — no edits, no schema changes, no ' +
      'INSERT, UPDATE, DELETE or DROP. It is a guard on this app, not on the server: a read-only ' +
      'account is what protects the database from every other client.',
  },
] as const satisfies readonly AccessInfo[]

export function accessInfo(access: string): AccessInfo {
  return ACCESS_MODES.find((info) => info.value === access) ?? ACCESS_MODES[0]
}

export type Engine =
  | 'mysql'
  | 'postgres'
  | 'sqlite'
  | 'sqlserver'
  | 'oracle'
  | 'mongo'
  | 'redis'

export interface EngineInfo {
  value: Engine
  label: string
  port: number
  user: string
  // Whether the whole database is one file on this machine rather than a server somewhere. There
  // is nothing to dial and nobody to be, so the dialog asks for a *path* instead of a host, a port,
  // a user and a password — see `domain.Engine.File`, which is where the rule lives.
  isFile: boolean
  // Whether every connection to this engine is made *as* somebody. A MongoDB server started
  // without access control and a Redis server with no ACL both take no user name at all, which is
  // what greys the box rather than refusing the profile.
  needsUser: boolean
  // PostgreSQL binds a session to one database for its whole life, so a profile that names
  // none has nothing to connect to. MySQL can be browsed without one.
  needsDatabase: boolean
  // What the Database box is called and what it takes, in this engine's own words. They are fields
  // rather than a comparison in the dialog because what a box *holds* is a fact about the engine:
  // Oracle's is not a database at all but the service its listener is asked for, Redis's is a
  // number, and SQLite's is a path to a file.
  databaseLabel: string
  databaseHint: string
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
  // Whether a view or a routine of this engine is declared by a *statement* the app can read back
  // and put again. It is what decides whether the definition editor is offered at all: MongoDB's
  // view is a pipeline over a collection rather than a statement, and a Redis key space is not
  // declared, so both answer `domain.Unsupported` and neither is offered a New or a Design that
  // could only fail.
  hasSource: boolean
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
  // Whether this engine has triggers at all. Every SQL engine here does; a document store's change
  // stream is something a *client* watches rather than a program the server keeps, and a key space
  // has nothing to fire on — so neither has a folder.
  hasTriggers: boolean
  // Whether this engine has views whose rows are *stored* rather than recomputed. PostgreSQL and
  // Oracle do, and both refresh one with a statement of its own; SQL Server's indexed view is a
  // view with an index on it rather than an object of its own kind, and MongoDB's on-demand
  // materialized view is a pipeline ending in `$merge` — an aggregation a reader runs, not an
  // object with a definition.
  hasMaterializedViews: boolean
  // Whether a number generator here is an object of the schema. PostgreSQL, Oracle and SQL Server
  // have one; MySQL's `AUTO_INCREMENT` is a column's own fact and so is not a folder.
  hasSequences: boolean
  // Whether the server runs statements on a schedule of its own, kept as a *declaration* this app
  // can read back and put again. MySQL alone does. Every other scheduler here is a procedure called
  // with arguments — Oracle's DBMS_SCHEDULER, SQL Server Agent's `sp_add_job` — which is not a
  // definition an editor could save.
  hasEvents: boolean
  // Whether a table's rows can be stored in named pieces this app can address. MySQL, PostgreSQL
  // and Oracle name one; SQL Server's are numbered pieces of a partition *scheme* rather than named
  // objects, and SQLite has none. It is what decides whether a table draws a Partitions folder.
  hasPartitions: boolean
  // Whether a column can carry a description on the server. MySQL keeps one as a clause of the
  // column and PostgreSQL and Oracle as a `COMMENT ON` statement; SQL Server's is an extended
  // property written by a stored procedure rather than a comment, and SQLite has none — so the
  // designer draws no Comment column there.
  hasColumnComments: boolean
  // Whether emptying and truncating are two different things. They are on a table store, where one
  // deletes the rows and the other resets the storage; a collection has no storage to reset, so
  // offering both would be offering the same command twice under two names.
  hasTruncate: boolean
  // Whether a database of this engine can be backed up. A backup here is a file of the statements
  // that would put the database back, so the engines that have one are the engines whose statements
  // declare a table and write a row — see `domain.Engine.Backups`, which is where the rule lives.
  hasBackups: boolean
  // Whether a reader may turn autocommit off and decide for themselves when what they have done
  // becomes true. Every SQL engine here has one; MongoDB's needs a replica set and a client session
  // it carries through every command, and Redis's MULTI shows the reader nothing until it has
  // already run — so both answer `domain.Unsupported` and neither is offered the button.
  hasTransactions: boolean
  // What this engine's vendor calls its own dump and restore programs, and `''` where it ships no
  // such pair. They are *names* rather than a boolean because the menu says them: a reader who has
  // installed MongoDB Database Tools is looking for the word `mongodump`. Whether either is on
  // this machine is not knowable here — that is `tools.toolkit`, which asks the Go side.
  //
  // These mirror `domain.Engine.Toolkit`, which is where the rule lives.
  dumpTool: string
  restoreTool: string
  // Whether this engine has accounts at all. A SQLite database is a file: whoever can open it can
  // do everything to it and whoever cannot can do nothing, which is the operating system's answer
  // rather than the engine's — so there is no `Users` folder under one and no tab to open.
  hasAccounts: boolean
  // What this engine calls one, in its own words. PostgreSQL has no users: `CREATE USER` there is
  // `CREATE ROLE … LOGIN` and nothing else, so a menu saying `New User…` would be naming something
  // the server does not have. It is a field for the reason `databaseLabel` is one — never a
  // comparison against an engine's name in a component.
  accountLabel: string
  // Whether an account here is granted **roles** rather than privileges. On MongoDB a role is the
  // only unit there is — a privilege belongs to the role and not to the account — and on Redis
  // there are no roles at all. Both facts reach the editor as the driver's own `AccountOptions`,
  // and this is only what keeps a menu item off an engine that has neither.
  hasRoles: boolean
  // Whether there is a server to monitor at all. A SQLite database is a file: nobody is connected to
  // it, nothing is in flight, and a pragma is a fact about this program's own handle rather than a
  // server's state — so there is no `Server Monitor` node under one and no tab to open.
  hasMonitor: boolean
  // Whether one statement can be stopped while its connection is left open. MySQL has `KILL QUERY`,
  // PostgreSQL `pg_cancel_backend` and MongoDB `killOp`; SQL Server has only `KILL`, Oracle's cancel
  // arrived in 18c and is addressed by the statement rather than the session, and a Redis server runs
  // one command at a time and to completion. An engine without it is not offered the button.
  cancelsStatement: boolean
  // Whether a connection can be ended by the server. Every SQL engine here can; MongoDB cannot — a
  // client there disconnects itself — which is the one engine with the cancel and not this.
  closesConnection: boolean
  // What this engine calls one of the things in that list, in its own words: a MySQL process, a
  // PostgreSQL backend, an Oracle or SQL Server session, a MongoDB operation, a Redis client. It is a
  // field for the reason `accountLabel` is one — never a comparison against an engine's name in a
  // component — and `''` where there is nothing to name.
  processLabel: string
  // Whether this engine records the statements that ran **slowly**. Every server here does in some
  // form — a slow log, an extension, a plan cache, a shared pool, a profile collection, a ring in
  // memory — and a SQLite database does not, being a file that keeps no record of what was asked of
  // it. An engine without it is drawn no `Slow Queries` pane.
  hasProfiler: boolean
  // Whether that recording can be turned on, off or thrown away from this app. SQL Server and Oracle
  // record whether or not anybody asked and have no switch, so the dialog is not offered rather than
  // being offered with every control dead. It is a separate fact from `hasProfiler` because "always
  // recording" and "not recording" are opposite answers that would otherwise share one field.
  profilerConfigurable: boolean
  // Whether that recording belongs to a **database** rather than to the server. A MongoDB profile is
  // a collection of one database and a PostgreSQL extension is installed into one, so on those two
  // the pane has to say which; MySQL's slow log, a plan cache, a shared pool and a Redis ring are all
  // the server's, and a database selector over them would be a control that changed nothing.
  profilerPerDatabase: boolean
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
    isFile: false,
    needsUser: true,
    needsDatabase: false,
    databaseLabel: 'Default database',
    databaseHint: 'Optional',
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    hasSource: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTriggers: true,
    hasMaterializedViews: false,
    hasSequences: false,
    hasEvents: true,
    hasPartitions: true,
    hasColumnComments: true,
    hasTruncate: true,
    hasBackups: true,
    hasTransactions: true,
    dumpTool: 'mysqldump',
    // The ordinary client, because mysqldump writes a plain script and running one is all it takes
    // to put it back. It is the one of the three whose restore is not a program of its own.
    restoreTool: 'mysql',
    hasAccounts: true,
    accountLabel: 'User',
    hasRoles: true,
    hasMonitor: true,
    cancelsStatement: true,
    closesConnection: true,
    processLabel: 'Process',
    // The slow log is a **table** here once `log_output` says so, which is why this app can read
    // it at all and why turning the recording on sets two things.
    hasProfiler: true,
    profilerConfigurable: true,
    profilerPerDatabase: false,
  },
  {
    value: 'postgres',
    label: 'PostgreSQL',
    port: 5432,
    user: 'postgres',
    isFile: false,
    needsUser: true,
    needsDatabase: true,
    databaseLabel: 'Database',
    databaseHint: 'Required',
    hasSchemas: true,
    databaseHasOwner: true,
    documents: false,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    hasSource: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTriggers: true,
    hasMaterializedViews: true,
    hasSequences: true,
    hasEvents: false,
    hasPartitions: true,
    hasColumnComments: true,
    hasTruncate: true,
    hasBackups: true,
    hasTransactions: true,
    dumpTool: 'pg_dump',
    restoreTool: 'pg_restore',
    hasAccounts: true,
    // A role and a user are one object here: `CREATE USER` is `CREATE ROLE … LOGIN`.
    accountLabel: 'Role',
    hasRoles: true,
    hasMonitor: true,
    cancelsStatement: true,
    closesConnection: true,
    // A backend is an operating-system process of its own here, which is what the server calls one
    // everywhere it reports them — `pg_stat_activity`, `pg_terminate_backend`.
    processLabel: 'Backend',
    // The recording is an **extension** rather than a setting, and it is installed per database.
    hasProfiler: true,
    profilerConfigurable: true,
    profilerPerDatabase: true,
  },
  {
    // A SQLite database is a file, so there is no host, no port and nobody to be — the Database box
    // holds a path and the dialog draws a Browse beside it. It has no second database to make, no
    // TRUNCATE, and no stored routine: a function there belongs to the program that opened the file.
    value: 'sqlite',
    label: 'SQLite',
    port: 0,
    user: '',
    isFile: true,
    needsUser: false,
    needsDatabase: true,
    databaseLabel: 'Database file',
    databaseHint: 'The file has to be there already — this never makes an empty one.',
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: false,
    createsDatabases: false,
    createsTables: true,
    hasViews: true,
    hasSource: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: false,
    hasTriggers: false,
    hasMaterializedViews: false,
    hasSequences: false,
    hasEvents: false,
    hasPartitions: false,
    hasColumnComments: false,
    hasTruncate: false,
    hasBackups: true,
    hasTransactions: true,
    // A SQLite database is one file, so copying it *is* the dump. There is no client program to
    // run and nothing for one to do that Explorer does not already.
    dumpTool: '',
    restoreTool: '',
    hasAccounts: false,
    accountLabel: '',
    hasRoles: false,
    // Nobody is connected to a file, so there is nothing to watch and nothing to end.
    hasMonitor: false,
    cancelsStatement: false,
    closesConnection: false,
    processLabel: '',
    // A file keeps no record of what was asked of it. What this engine has instead is
    // `EXPLAIN QUERY PLAN` on a statement the reader types.
    hasProfiler: false,
    profilerConfigurable: false,
    profilerPerDatabase: false,
  },
  {
    // SQL Server names a table by its database *and* its schema, so the tree has both levels the
    // way PostgreSQL's does. A database of its own has a collation and nothing else the dialog
    // asks for, which is MySQL's arrangement rather than PostgreSQL's.
    value: 'sqlserver',
    label: 'SQL Server',
    port: 1433,
    user: 'sa',
    isFile: false,
    needsUser: true,
    needsDatabase: false,
    databaseLabel: 'Default database',
    databaseHint: 'Optional',
    hasSchemas: true,
    databaseHasOwner: false,
    documents: false,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    hasSource: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTriggers: true,
    hasMaterializedViews: false,
    hasSequences: true,
    hasEvents: false,
    hasPartitions: false,
    hasColumnComments: false,
    hasTruncate: true,
    hasBackups: true,
    hasTransactions: true,
    // SQL Server's own backup is a statement the *server* runs, writing to a path the server can
    // see rather than one this machine has. There is no client program that carries a database out.
    dumpTool: '',
    restoreTool: '',
    hasAccounts: true,
    accountLabel: 'User',
    hasRoles: true,
    hasMonitor: true,
    // `KILL` ends the session; there is no statement that stops a request and leaves it.
    cancelsStatement: false,
    closesConnection: true,
    processLabel: 'Session',
    // The plan cache holds a statistic per plan whether or not anybody asked, so there is
    // nothing to turn on — only the whole cache to empty.
    hasProfiler: true,
    profilerConfigurable: false,
    profilerPerDatabase: false,
  },
  {
    // Oracle has no level between a connection and a schema: the Database box holds the *service*
    // the listener is asked for, and what the tree lists below the connection are schemas. Making
    // one is making a user with a password, which is administration rather than this dialog's
    // business — see `createsDatabases`.
    value: 'oracle',
    label: 'Oracle',
    port: 1521,
    user: 'system',
    isFile: false,
    needsUser: true,
    needsDatabase: true,
    databaseLabel: 'Service name',
    databaseHint: 'The service the listener is asked for, such as XEPDB1',
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: false,
    createsDatabases: false,
    createsTables: true,
    hasViews: true,
    hasSource: true,
    databaseNeedsCollection: false,
    hasForeignKeys: true,
    hasRoutines: true,
    hasTriggers: true,
    hasMaterializedViews: true,
    hasSequences: true,
    hasEvents: false,
    hasPartitions: true,
    hasColumnComments: true,
    hasTruncate: true,
    hasBackups: true,
    hasTransactions: true,
    // Oracle's Data Pump runs *inside* the instance and writes to a directory object the database
    // owns, so `expdp` is not a program that carries anything to this machine.
    dumpTool: '',
    restoreTool: '',
    hasAccounts: true,
    accountLabel: 'User',
    hasRoles: true,
    hasMonitor: true,
    // `ALTER SYSTEM CANCEL SQL` arrived in 18c and names the statement rather than the session, so
    // this app does not claim it — what it offers is ending the session.
    cancelsStatement: false,
    closesConnection: true,
    processLabel: 'Session',
    // `v$sqlarea` is there whether or not anybody asked, and the only thing that would empty it
    // would reparse every statement on the instance — so this engine offers no change at all.
    hasProfiler: true,
    profilerConfigurable: false,
    profilerPerDatabase: false,
  },
  {
    // A MongoDB server started without access control takes no credentials at all, which is why
    // this is the one engine whose default user is blank rather than an administrator's name.
    value: 'mongo',
    label: 'MongoDB',
    port: 27017,
    user: '',
    isFile: false,
    needsUser: false,
    needsDatabase: false,
    databaseLabel: 'Default database',
    databaseHint: 'Optional',
    hasSchemas: false,
    databaseHasOwner: false,
    documents: true,
    keys: false,
    createsDatabases: true,
    createsTables: true,
    hasViews: true,
    // A MongoDB view is a collection declared by a *document* — the collection it reads and the
    // pipeline over it — rather than by a statement, so there is no definition to read or put back.
    hasSource: false,
    databaseNeedsCollection: true,
    hasForeignKeys: false,
    hasRoutines: false,
    hasTriggers: false,
    hasMaterializedViews: false,
    hasSequences: false,
    hasEvents: false,
    hasPartitions: false,
    hasColumnComments: false,
    hasTruncate: false,
    // A backup here is a file of statements, and a document store's are commands rather than a
    // declaration and a row — but its *vendor* ships a pair of programs that carry the BSON
    // itself, which is what these two are and why this engine has the second without the first.
    hasBackups: false,
    hasTransactions: false,
    dumpTool: 'mongodump',
    restoreTool: 'mongorestore',
    hasAccounts: true,
    accountLabel: 'User',
    hasRoles: true,
    hasMonitor: true,
    // The one engine with the cancel and not the close: `killOp` stops an operation, and a client
    // disconnects itself.
    cancelsStatement: true,
    closesConnection: false,
    processLabel: 'Operation',
    // The engine the word profiling comes from: `setProfilingLevel`, per database, written to a
    // capped collection of that database.
    hasProfiler: true,
    profilerConfigurable: true,
    profilerPerDatabase: true,
  },
  {
    // A Redis database is *numbered* rather than named — `0` to `15` on a default install — so the
    // Database box takes an index, and leaving it blank is database 0. Nothing else about this
    // engine is declared: one key space to a database, and no shape to any of it.
    value: 'redis',
    label: 'Redis',
    port: 6379,
    user: '',
    isFile: false,
    needsUser: false,
    needsDatabase: false,
    databaseLabel: 'Default database',
    databaseHint: 'A number, counting from 0',
    hasSchemas: false,
    databaseHasOwner: false,
    documents: false,
    keys: true,
    createsDatabases: false,
    createsTables: false,
    hasViews: false,
    hasSource: false,
    databaseNeedsCollection: false,
    hasForeignKeys: false,
    hasRoutines: false,
    hasTriggers: false,
    hasMaterializedViews: false,
    hasSequences: false,
    hasEvents: false,
    hasPartitions: false,
    hasColumnComments: false,
    hasTruncate: false,
    hasBackups: false,
    hasTransactions: false,
    // Redis persists itself — an RDB file the server writes where the server is — and there is no
    // client program that pulls one back to this machine.
    dumpTool: '',
    restoreTool: '',
    hasAccounts: true,
    accountLabel: 'User',
    // A Redis ACL grants command categories and has nothing that is granted in turn.
    hasRoles: false,
    hasMonitor: true,
    // A Redis server runs one command at a time and to completion, so there is nothing in flight for
    // a second connection to cancel. What can be ended is the client.
    cancelsStatement: false,
    closesConnection: true,
    processLabel: 'Client',
    // `SLOWLOG` is a ring in memory, bounded by the server, and both its settings are a
    // `CONFIG SET` away.
    hasProfiler: true,
    profilerConfigurable: true,
    profilerPerDatabase: false,
  },
] as const satisfies readonly EngineInfo[]

export function engineInfo(engine: string): EngineInfo {
  return ENGINES.find((info) => info.value === engine) ?? ENGINES[0]
}

/**
 * Whether this engine's vendor ships a dump and a restore program of its own. It is what decides
 * whether the tree draws the two items at all — never whether they are *usable*, which is a fact
 * about this machine and comes from `tools.toolkit`.
 */
export function hasToolkit(info: EngineInfo): boolean {
  return info.dumpTool !== ''
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
    access: '',
    mcp: false,
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
    // A proxy starts off, and starts SOCKS5 — which is what a database client is nearly always put
    // behind, an HTTP proxy being built for requests rather than for sockets.
    proxy: {
      enabled: false,
      kind: PROXY_KINDS[0].value,
      host: '',
      port: PROXY_KINDS[0].port,
      user: '',
    },
    proxySecret: '',
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
    access: profile.access,
    mcp: profile.mcp,
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
    proxy: { ...profile.proxy },
    sshSecret: '',
    proxySecret: '',
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
