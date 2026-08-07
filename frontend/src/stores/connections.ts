import { create } from 'zustand'

import { backups as backupFiles, type Backup } from '@/bridge/backup'
import { type AppError, toAppError } from '@/bridge/errors'
import {
  connections,
  engineInfo,
  engineWords,
  namespaceOf,
  toProfileInput,
  NO_NAMESPACE,
  type EngineInfo,
  type Namespace,
  type Objects,
  type Profile,
  type ProfileInput,
  type Session,
  type TableSchema,
} from '@/bridge/connections'
import { objectDump, type DumpRequest } from '@/bridge/export'
import { schema, type TableStructure } from '@/bridge/schema'
import {
  filterDatabases,
  loadDatabaseFilters,
  saveDatabaseFilters,
} from '@/features/connections/databaseFilter'
import type { ConnectionStatus, NodeKind, TreeNode } from '@/features/connections/tree'

export const connectionNodeId = (profileId: string) => `conn:${profileId}`
export const databaseNodeId = (profileId: string, database: string) => `db:${profileId}:${database}`
export const schemaNodeId = (profileId: string, ns: Namespace) =>
  `schema:${namespaceKey(profileId, ns)}`
export const folderNodeId = (kind: NodeKind, profileId: string, ns: Namespace) =>
  `${kind}:${namespaceKey(profileId, ns)}`
export const tableNodeId = (profileId: string, ns: Namespace, table: string) =>
  `table:${namespaceKey(profileId, ns)}:${table}`

export const databaseKey = (profileId: string, database: string) => `${profileId}:${database}`
export const namespaceKey = (profileId: string, ns: Namespace) =>
  `${profileId}:${ns.database}:${ns.schema}`
export const tableKey = (profileId: string, ns: Namespace, table: string) =>
  `${namespaceKey(profileId, ns)}:${table}`

interface ConnectionsState {
  profiles: Profile[]
  sessions: Record<string, Session>
  statuses: Record<string, ConnectionStatus>
  databases: Record<string, string[]>
  /** Keyed by connection and database: the level below a database, empty for MySQL. */
  schemas: Record<string, string[]>
  /** Both keyed by connection and namespace, since a table name only means something there. */
  objects: Record<string, Objects>
  tableSchemas: Record<string, TableSchema[]>
  /** Keyed by connection, namespace and table: the fields, indexes and keys of one table. */
  structures: Record<string, TableStructure>
  /**
   * The backups kept for a namespace, keyed like its objects are. They are files on this machine
   * rather than anything a catalogue answered with, and they live here anyway because **the tree
   * draws them**: the Backups folder opens onto the names, and a second copy of the folder held
   * somewhere else would be two lists to keep in step.
   */
  backups: Record<string, Backup[]>
  activeNamespace: Record<string, Namespace>
  /**
   * Which databases each connection draws, keyed by connection: the part of a big server a reader
   * works on. It outlives the app rather than the session — see `databaseFilter.ts`.
   */
  databaseFilters: Record<string, string>
  expanded: ReadonlySet<string>
  loading: boolean
  error: AppError | null

  loadProfiles: () => Promise<void>
  saveProfile: (input: ProfileInput) => Promise<Profile>
  /** Marks a connection with a colour, or clears it. Nothing else about the profile changes. */
  setColour: (profileId: string, colour: string) => Promise<void>
  deleteProfile: (profileId: string) => Promise<void>
  connect: (profileId: string, password: string) => Promise<void>
  disconnect: (profileId: string) => Promise<void>
  selectNamespace: (profileId: string, ns: Namespace) => void
  /**
   * Narrows what a connection's branch draws to the databases whose name holds `pattern`, or draws
   * every one of them again where nothing was typed. It narrows the **tree** and nothing else: the
   * list itself is untouched, so a namespace picker, a restore and a dump still offer the whole of it.
   */
  setDatabaseFilter: (profileId: string, pattern: string) => void
  loadSchemas: (profileId: string, database: string) => Promise<void>
  loadObjects: (profileId: string, ns: Namespace) => Promise<void>
  loadTableSchemas: (profileId: string, ns: Namespace) => Promise<void>
  loadStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  /** Re-reads a table's shape whether or not it is already cached, which Refresh needs. */
  refreshStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  loadBackups: (profileId: string, ns: Namespace) => Promise<void>
  refreshObjects: (profileId: string, ns: Namespace) => Promise<void>
  /** Re-reads a namespace's backup folder whether or not it is already cached. */
  refreshBackups: (profileId: string, ns: Namespace) => Promise<void>
  /**
   * Runs one change to the backup folder — a backup taken, deleted, copied or renamed — and
   * re-reads it. It is `changeSchema`'s sibling and throws for the same reason: what asked is a
   * dialog, and a rejection belongs in the dialog rather than in the status bar.
   */
  changeBackups: (profileId: string, ns: Namespace, change: ChangeSchema) => Promise<void>
  refreshSchemas: (profileId: string, database: string) => Promise<void>
  /** Re-reads the database list and forgets everything under a database it no longer carries. */
  refreshDatabases: (profileId: string) => Promise<void>
  /**
   * Runs one schema change and re-reads whatever it invalidated. The change itself is the
   * caller's — a bridge call bound to a session — so the store owns the caches and nothing else.
   */
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
  /**
   * The same for a change to the database list itself, which invalidates a level higher up than
   * any namespace: creating or dropping a database.
   */
  changeDatabases: (profileId: string, change: ChangeSchema) => Promise<void>
  /**
   * Writes a dump of a table, or of every table a namespace holds, to a file the reader picks in
   * the host's own save dialog. It changes nothing on the server, so there is no cache to
   * invalidate — and no dialog waiting on the answer, which is why a failure is parked in `error`
   * the way a read's is rather than thrown the way `changeSchema` throws one.
   */
  dump: (profileId: string, request: DumpRequest) => Promise<void>
  /**
   * Shows a namespace's backup folder in the host's own file manager. It is a dump's sibling here
   * for the same reason: nothing on the server changes, and nothing is waiting on the answer — so
   * a failure is parked in `error` rather than thrown at a dialog that is not there.
   */
  openBackupFolder: (profileId: string, namespace: Namespace) => Promise<void>
  toggleNode: (nodeId: string) => void
  /**
   * Puts a failure in the bar. It is what an action raised from a *menu* has instead of a dialog:
   * nothing is waiting on the answer, so there is nowhere else for the reader to be told.
   */
  reportError: (thrown: unknown) => void
  clearError: () => void
}

/** What a schema change needs from the store: the session it runs on. */
export type ChangeSchema = (sessionId: string) => Promise<void>

/**
 * What a schema change touched. `namespace` is the one whose table list is now stale, and
 * `table` the one whose shape is — a table the change dropped or renamed away names none, and
 * is forgotten because the fresh list no longer carries it.
 */
export interface SchemaTarget {
  namespace: Namespace
  table?: string
}

export const useConnectionsStore = create<ConnectionsState>((set, get) => ({
  profiles: [],
  sessions: {},
  statuses: {},
  databases: {},
  schemas: {},
  objects: {},
  backups: {},
  tableSchemas: {},
  structures: {},
  activeNamespace: {},
  databaseFilters: loadDatabaseFilters(),
  expanded: new Set<string>(),
  loading: false,
  error: null,

  loadProfiles: async () => {
    set({ loading: true })
    try {
      const profiles = await connections.listProfiles()
      set({ profiles, loading: false, error: null })
    } catch (thrown) {
      set({ loading: false, error: toAppError(thrown) })
    }
  },

  saveProfile: async (input) => {
    const saved = await connections.saveProfile(input)
    await get().loadProfiles()
    return saved
  },

  // A blank password in a saved profile keeps the stored one, so re-saving the profile it was
  // read from is the whole of what marking one costs — see ConnectionAPI.SaveProfile.
  setColour: async (profileId, colour) => {
    const profile = get().profiles.find((one) => one.id === profileId)
    if (!profile) return

    await get().saveProfile({ ...toProfileInput(profile), colour })
  },

  deleteProfile: async (profileId) => {
    if (get().sessions[profileId]) await get().disconnect(profileId)
    await connections.deleteProfile(profileId)
    // A connection that is gone takes its narrowing with it, or a connection saved later under the
    // same id would open already filtered by a pattern nobody typed for it.
    get().setDatabaseFilter(profileId, '')
    await get().loadProfiles()
  },

  connect: async (profileId, password) => {
    set((s) => ({
      statuses: { ...s.statuses, [profileId]: 'connecting' },
      error: null,
    }))

    try {
      const session = await connections.connect(profileId, password)
      const names = await connections.listDatabases(session.id)

      set((s) => ({
        sessions: { ...s.sessions, [profileId]: session },
        statuses: { ...s.statuses, [profileId]: 'connected' },
        databases: { ...s.databases, [profileId]: names },
        expanded: new Set(s.expanded).add(connectionNodeId(profileId)),
      }))
    } catch (thrown) {
      set((s) => ({
        statuses: { ...s.statuses, [profileId]: 'error' },
        error: toAppError(thrown),
      }))
      throw thrown
    }
  },

  disconnect: async (profileId) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      await connections.disconnect(session.id)
    } finally {
      set((s) => {
        const sessions = { ...s.sessions }
        const databases = { ...s.databases }
        const activeNamespace = { ...s.activeNamespace }
        delete sessions[profileId]
        delete databases[profileId]
        delete activeNamespace[profileId]

        const own = (key: string) => !key.startsWith(`${profileId}:`)
        const drop = <T>(source: Record<string, T>) =>
          Object.fromEntries(Object.entries(source).filter(([key]) => own(key)))

        const expanded = new Set(s.expanded)
        expanded.delete(connectionNodeId(profileId))

        return {
          sessions,
          databases,
          activeNamespace,
          expanded,
          schemas: drop(s.schemas),
          objects: drop(s.objects),
          backups: drop(s.backups),
          tableSchemas: drop(s.tableSchemas),
          structures: drop(s.structures),
          statuses: { ...s.statuses, [profileId]: 'idle' },
        }
      })
    }
  },

  // Names the active namespace and nothing else. Expanding is `toggleNode`'s alone, or the two
  // fight: the tree's caret both selects and toggles, and an expand hidden in here would undo it.
  selectNamespace: (profileId, ns) =>
    set((s) => ({ activeNamespace: { ...s.activeNamespace, [profileId]: ns } })),

  setDatabaseFilter: (profileId, pattern) =>
    set((s) => {
      const databaseFilters = { ...s.databaseFilters }
      // An emptied box is a connection with no filter, not one filtered by nothing: a stored empty
      // pattern would read as a narrowing in the entry and light the funnel for ever.
      if (pattern.trim() === '') delete databaseFilters[profileId]
      else databaseFilters[profileId] = pattern

      saveDatabaseFilters(databaseFilters)
      return { databaseFilters }
    }),

  loadSchemas: async (profileId, database) => {
    const { sessions, schemas } = get()
    const session = sessions[profileId]
    const key = databaseKey(profileId, database)
    if (!session || schemas[key]) return

    try {
      const names = await connections.listSchemas(session.id, database)
      set((s) => ({ schemas: { ...s.schemas, [key]: names } }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  loadObjects: async (profileId, ns) => {
    if (get().objects[namespaceKey(profileId, ns)]) return
    await get().refreshObjects(profileId, ns)
  },

  loadBackups: async (profileId, ns) => {
    if (get().backups[namespaceKey(profileId, ns)]) return
    await get().refreshBackups(profileId, ns)
  },

  // Cached until disconnect, like tables: a schema read costs one catalogue scan, and the
  // editor asks for it on every keystroke.
  loadTableSchemas: async (profileId, ns) => {
    const { sessions, tableSchemas } = get()
    const session = sessions[profileId]
    const key = namespaceKey(profileId, ns)
    if (!session || tableSchemas[key]) return

    try {
      const tables = await connections.loadTableSchemas(session.id, ns)
      set((s) => ({ tableSchemas: { ...s.tableSchemas, [key]: tables } }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  // A table's shape is cached like its rows are not: the tree draws it, so it is read once
  // per table and re-read only when a schema change or a Refresh says it moved.
  loadStructure: async (profileId, ns, table) => {
    if (get().structures[tableKey(profileId, ns, table)]) return
    await get().refreshStructure(profileId, ns, table)
  },

  refreshStructure: async (profileId, ns, table) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const structure = await schema.loadStructure(session.id, ns, table)
      set((s) => ({
        structures: { ...s.structures, [tableKey(profileId, ns, table)]: structure },
      }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  refreshObjects: async (profileId, ns) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const objects = await connections.loadObjects(session.id, ns)
      set((s) => ({
        objects: { ...s.objects, [namespaceKey(profileId, ns)]: objects },
        tableSchemas: without(s.tableSchemas, namespaceKey(profileId, ns)),
      }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  refreshBackups: async (profileId, ns) => {
    const session = get().sessions[profileId]
    if (!session) return

    // A folder that could not be read is recorded as **empty, with the reason in the bar** rather
    // than left unset: unset is what "nobody has looked yet" means here, and a list still waiting
    // on a read that already failed would spin for ever.
    try {
      const files = await backupFiles.list(session.id, ns)
      set((s) => ({ backups: { ...s.backups, [namespaceKey(profileId, ns)]: files } }))
    } catch (thrown) {
      set((s) => ({
        backups: { ...s.backups, [namespaceKey(profileId, ns)]: [] },
        error: toAppError(thrown),
      }))
    }
  },

  changeBackups: async (profileId, ns, change) => {
    const session = get().sessions[profileId]
    if (!session) return

    await change(session.id)
    await get().refreshBackups(profileId, ns)
  },

  refreshSchemas: async (profileId, database) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const names = await connections.listSchemas(session.id, database)
      set((s) => ({ schemas: { ...s.schemas, [databaseKey(profileId, database)]: names } }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  refreshDatabases: async (profileId) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const names = await connections.listDatabases(session.id)
      set((s) => {
        // Every cache below a database is keyed `profileId:database:…`, so a database the fresh
        // list no longer carries takes its schemas, tables and shapes with it.
        const listed = new Set(names.map((database) => databaseKey(profileId, database)))
        const held = (key: string) =>
          !key.startsWith(`${profileId}:`) || listed.has(key.split(':').slice(0, 2).join(':'))
        const keep = <T,>(source: Record<string, T>) =>
          Object.fromEntries(Object.entries(source).filter(([key]) => held(key)))

        return {
          databases: { ...s.databases, [profileId]: names },
          schemas: keep(s.schemas),
          objects: keep(s.objects),
          tableSchemas: keep(s.tableSchemas),
          structures: keep(s.structures),
        }
      })
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  // The failure is re-thrown rather than parked in `error`, because a schema change is asked
  // for from a dialog and that dialog is where it has to be answered.
  changeSchema: async (profileId, target, change) => {
    const session = get().sessions[profileId]
    if (!session) return

    await change(session.id)
    await get().refreshObjects(profileId, target.namespace)

    // Whatever the change did to the table list decides which shapes are still worth holding,
    // so a dropped or renamed table needs no telling.
    set((s) => {
      const listed = new Set(
        (s.objects[namespaceKey(profileId, target.namespace)]?.tables ?? []).map((table) =>
          tableKey(profileId, target.namespace, table),
        ),
      )
      const prefix = `${namespaceKey(profileId, target.namespace)}:`
      const structures = Object.fromEntries(
        Object.entries(s.structures).filter(
          ([key]) => !key.startsWith(prefix) || listed.has(key),
        ),
      )
      return { structures }
    })

    if (target.table) await get().refreshStructure(profileId, target.namespace, target.table)
  },

  changeDatabases: async (profileId, change) => {
    const session = get().sessions[profileId]
    if (!session) return

    await change(session.id)
    await get().refreshDatabases(profileId)
  },

  dump: async (profileId, request) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      await objectDump.run(session.id, request)
      // The bar holds one failure, so it holds the last outcome: a dump that worked has to take a
      // previous one down or it would read as this dump's.
      set({ error: null })
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  openBackupFolder: async (profileId, namespace) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      await backupFiles.openFolder(session.id, namespace)
      set({ error: null })
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  toggleNode: (nodeId) =>
    set((s) => {
      const expanded = new Set(s.expanded)
      if (expanded.has(nodeId)) expanded.delete(nodeId)
      else expanded.add(nodeId)
      return { expanded }
    }),

  reportError: (thrown) => set({ error: toAppError(thrown) }),

  clearError: () => set({ error: null }),
}))

function without<T>(source: Record<string, T>, key: string): Record<string, T> {
  return Object.fromEntries(Object.entries(source).filter(([held]) => held !== key))
}

interface TreeInput {
  profiles: Profile[]
  statuses: Record<string, ConnectionStatus>
  databases: Record<string, string[]>
  schemas: Record<string, string[]>
  objects: Record<string, Objects>
  backups: Record<string, Backup[]>
  structures: Record<string, TableStructure>
  activeNamespace: Record<string, Namespace>
  databaseFilters: Record<string, string>
}

export function buildTree(input: TreeInput): TreeNode[] {
  return input.profiles.map((profile) => {
    const info = engineInfo(profile.engine)

    return {
      id: connectionNodeId(profile.id),
      kind: 'connection',
      label: profile.name,
      profileId: profile.id,
      engine: info.value,
      environment: profile.environment,
      colour: profile.colour,
      status: input.statuses[profile.id] ?? 'idle',
      // Two connections to one server differ only in their name, and two connections *named*
      // alike differ only in their server — so the row that carries the name carries the server
      // too, and the reader never has to open a dialog to tell them apart.
      badge: `${profile.host}:${profile.port}`,
      // The reader's own narrowing, applied where the branch is built and nowhere else: what the
      // *tree* draws of a big server is a reading preference, and the list the store holds is still
      // the whole one — see `setDatabaseFilter`.
      children: filterDatabases(
        input.databases[profile.id] ?? [],
        input.databaseFilters[profile.id] ?? '',
      ).map((database) =>
        info.hasSchemas
          ? schemaParentNode(input, info, profile.id, database)
          : namespaceNode(input, info, profile.id, 'database', namespaceOf(database), database),
      ),
    }
  })
}

// A PostgreSQL database holds no tables of its own — its schemas do — so it only carries the
// level below it, and the schema list arrives on first expand.
function schemaParentNode(
  input: TreeInput,
  info: EngineInfo,
  profileId: string,
  database: string,
): TreeNode {
  const names = input.schemas[databaseKey(profileId, database)]

  return {
    id: databaseNodeId(profileId, database),
    kind: 'database',
    label: database,
    profileId,
    namespace: namespaceOf(database),
    ...(names ? { badge: String(names.length) } : {}),
    children: (names ?? []).map((schema) =>
      namespaceNode(input, info, profileId, 'schema', namespaceOf(database, schema), schema),
    ),
  }
}

// The node that owns tables, whichever level of the tree that turns out to be. Its folders are drawn
// whether or not they have been read: they are what the caret is for, and one read fills all of
// them, so a folder with a badge is a folder whose namespace has been opened. A folder for something
// the engine does not have is not drawn at all — see `hasRoutines`.
function namespaceNode(
  input: TreeInput,
  info: EngineInfo,
  profileId: string,
  kind: 'database' | 'schema',
  ns: Namespace,
  label: string,
): TreeNode {
  const held = input.objects[namespaceKey(profileId, ns)]
  const kept = input.backups[namespaceKey(profileId, ns)]
  const active = input.activeNamespace[profileId] ?? NO_NAMESPACE

  // `read` is whether this folder's own source has been looked at, which is what decides if it
  // carries a number: a folder nobody has opened has no badge, not a zero. It defaults to the
  // catalogue read, which fills three of the four; Backups is read separately and says so.
  const folder = (
    folderKind: NodeKind,
    name: string,
    children: TreeNode[],
    read = held !== undefined,
  ): TreeNode => ({
    id: folderNodeId(folderKind, profileId, ns),
    kind: folderKind,
    label: name,
    profileId,
    namespace: ns,
    ...(read ? { badge: String(children.length) } : {}),
    children,
  })

  const objectNode = (objectKind: NodeKind, name: string, extra: Partial<TreeNode> = {}) => ({
    id: `${objectKind}:${namespaceKey(profileId, ns)}:${name}`,
    kind: objectKind,
    label: name,
    profileId,
    namespace: ns,
    ...extra,
  })

  return {
    id: kind === 'schema' ? schemaNodeId(profileId, ns) : databaseNodeId(profileId, ns.database),
    kind,
    label,
    profileId,
    namespace: ns,
    active: active.database === ns.database && active.schema === ns.schema,
    // How much is in here, once it has been read — the one number that says whether a namespace
    // is worth opening. A namespace nobody has opened yet has no number, not a zero.
    ...(held ? { badge: String(namespaceCount(info, held)) } : {}),
    children: [
      folder(
        'tables',
        engineWords(info.value).tables,
        (held?.tables ?? []).map((table) => tableNode(input, info, profileId, ns, table)),
      ),
      // Views is left out for an engine that has none at all, for the reason Functions is: a
      // folder that could only ever be empty is a folder that says nothing — see `hasViews`.
      ...(info.hasViews
        ? [
            folder(
              'views',
              'Views',
              (held?.views ?? []).map((view) => objectNode('view', view)),
            ),
          ]
        : []),
      ...(info.hasRoutines
        ? [
            folder(
              'routines',
              'Functions',
              (held?.routines ?? []).map((routine) =>
                objectNode(routine.procedure ? 'procedure' : 'function', routine.name, {
                  routine,
                  // Two functions of one name differ only in what they take, so the row says which.
                  ...(routine.arguments ? { badge: routine.arguments } : {}),
                }),
              ),
            ),
          ]
        : []),
      // Under the catalogue's own folders, because it is not one of them: what is in here is files
      // on this machine. It is filled by a read of its own rather than by the one the other three
      // share — a folder on disk is not something the server was asked about — which is why it is
      // the one folder whose badge can be there while theirs are not, and the other way round.
      // See `hasBackups`.
      ...(info.hasBackups
        ? [
            folder(
              'backups',
              'Backups',
              (kept ?? []).map((file) => objectNode('backup', file.name)),
              kept !== undefined,
            ),
          ]
        : []),
    ],
  }
}

// How much a namespace holds, which is not the same question on every engine. A table store is
// counted by what it lists; a key space lists one fixed table and always will, so the number worth
// drawing there is how many keys are under it — the driver's own count, since only the server has it.
function namespaceCount(info: EngineInfo, held: Objects): number {
  if (info.keys) return held.keyCount
  return held.tables.length + held.views.length + (held.routines?.length ?? 0)
}

// A table carries its folders whether or not its shape has been read: they are what the caret is
// for, and the read happens when one of them is opened. Foreign Keys is left out for an engine that
// has no referential constraints of any kind — see `hasForeignKeys`.
function tableNode(
  input: TreeInput,
  info: EngineInfo,
  profileId: string,
  ns: Namespace,
  table: string,
): TreeNode {
  const structure = input.structures[tableKey(profileId, ns, table)]
  const at = (kind: NodeKind) => objectNodeId(kind, profileId, ns, table)

  return {
    id: tableNodeId(profileId, ns, table),
    kind: 'table',
    label: table,
    profileId,
    namespace: ns,
    table,
    children: [
      {
        id: at('fields'),
        kind: 'fields',
        label: 'Fields',
        profileId,
        namespace: ns,
        table,
        ...(structure ? { badge: String(structure.columns.length) } : {}),
        children: (structure?.columns ?? []).map((column) => ({
          id: objectNodeId('field', profileId, ns, table, column.name),
          kind: 'field',
          label: column.name,
          profileId,
          namespace: ns,
          table,
          primaryKey: column.primaryKey,
          // The bare type, not the declared one: six levels of indent leave no room for
          // `character varying(255)`, and the designer is where a field's length is read.
          badge: column.type,
        })),
      },
      {
        id: at('indexes'),
        kind: 'indexes',
        label: 'Indexes',
        profileId,
        namespace: ns,
        table,
        ...(structure ? { badge: String(structure.indexes.length) } : {}),
        children: (structure?.indexes ?? []).map((index) => ({
          id: objectNodeId('index', profileId, ns, table, index.name),
          kind: 'index',
          label: index.name,
          profileId,
          namespace: ns,
          table,
          primaryKey: index.primary,
          badge: index.columns.join(', '),
        })),
      },
      ...(info.hasForeignKeys
        ? [
            {
              id: at('foreignKeys'),
              kind: 'foreignKeys' as NodeKind,
              label: 'Foreign Keys',
              profileId,
              namespace: ns,
              table,
              ...(structure ? { badge: String(structure.foreignKeys.length) } : {}),
              children: (structure?.foreignKeys ?? []).map((key) => ({
                id: objectNodeId('foreignKey', profileId, ns, table, key.name),
                kind: 'foreignKey' as NodeKind,
                label: key.name,
                profileId,
                namespace: ns,
                table,
                badge: `→ ${key.referencedTable}`,
              })),
            },
          ]
        : []),
    ],
  }
}

const objectNodeId = (
  kind: NodeKind,
  profileId: string,
  ns: Namespace,
  table: string,
  name = '',
) => `${kind}:${tableKey(profileId, ns, table)}${name === '' ? '' : `:${name}`}`
