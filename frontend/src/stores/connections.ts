import { create } from 'zustand'

import { type AppError, toAppError } from '@/bridge/errors'
import {
  connections,
  engineInfo,
  namespaceOf,
  NO_NAMESPACE,
  type Namespace,
  type Profile,
  type ProfileInput,
  type Session,
  type TableSchema,
} from '@/bridge/connections'
import { schema, type TableStructure } from '@/bridge/schema'
import type { ConnectionStatus, NodeKind, TreeNode } from '@/features/connections/tree'

export const connectionNodeId = (profileId: string) => `conn:${profileId}`
export const databaseNodeId = (profileId: string, database: string) => `db:${profileId}:${database}`
export const schemaNodeId = (profileId: string, ns: Namespace) =>
  `schema:${namespaceKey(profileId, ns)}`
export const tablesNodeId = (profileId: string, ns: Namespace) =>
  `tables:${namespaceKey(profileId, ns)}`
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
  tables: Record<string, string[]>
  tableSchemas: Record<string, TableSchema[]>
  /** Keyed by connection, namespace and table: the fields, indexes and keys of one table. */
  structures: Record<string, TableStructure>
  activeNamespace: Record<string, Namespace>
  expanded: ReadonlySet<string>
  loading: boolean
  error: AppError | null

  loadProfiles: () => Promise<void>
  saveProfile: (input: ProfileInput) => Promise<Profile>
  deleteProfile: (profileId: string) => Promise<void>
  connect: (profileId: string, password: string) => Promise<void>
  disconnect: (profileId: string) => Promise<void>
  selectNamespace: (profileId: string, ns: Namespace) => void
  loadSchemas: (profileId: string, database: string) => Promise<void>
  loadTables: (profileId: string, ns: Namespace) => Promise<void>
  loadTableSchemas: (profileId: string, ns: Namespace) => Promise<void>
  loadStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  /** Re-reads a table's shape whether or not it is already cached, which Refresh needs. */
  refreshStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  refreshTables: (profileId: string, ns: Namespace) => Promise<void>
  refreshSchemas: (profileId: string, database: string) => Promise<void>
  /**
   * Runs one schema change and re-reads whatever it invalidated. The change itself is the
   * caller's — a bridge call bound to a session — so the store owns the caches and nothing else.
   */
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
  toggleNode: (nodeId: string) => void
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
  tables: {},
  tableSchemas: {},
  structures: {},
  activeNamespace: {},
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

  deleteProfile: async (profileId) => {
    if (get().sessions[profileId]) await get().disconnect(profileId)
    await connections.deleteProfile(profileId)
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
          tables: drop(s.tables),
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

  loadTables: async (profileId, ns) => {
    const { sessions, tables } = get()
    const session = sessions[profileId]
    const key = namespaceKey(profileId, ns)
    if (!session || tables[key]) return

    try {
      const names = await connections.listTables(session.id, ns)
      set((s) => ({ tables: { ...s.tables, [key]: names } }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
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

  refreshTables: async (profileId, ns) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const names = await connections.listTables(session.id, ns)
      set((s) => ({
        tables: { ...s.tables, [namespaceKey(profileId, ns)]: names },
        tableSchemas: without(s.tableSchemas, namespaceKey(profileId, ns)),
      }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
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

  // The failure is re-thrown rather than parked in `error`, because a schema change is asked
  // for from a dialog and that dialog is where it has to be answered.
  changeSchema: async (profileId, target, change) => {
    const session = get().sessions[profileId]
    if (!session) return

    await change(session.id)
    await get().refreshTables(profileId, target.namespace)

    // Whatever the change did to the table list decides which shapes are still worth holding,
    // so a dropped or renamed table needs no telling.
    set((s) => {
      const listed = new Set(
        (s.tables[namespaceKey(profileId, target.namespace)] ?? []).map((table) =>
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

  toggleNode: (nodeId) =>
    set((s) => {
      const expanded = new Set(s.expanded)
      if (expanded.has(nodeId)) expanded.delete(nodeId)
      else expanded.add(nodeId)
      return { expanded }
    }),

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
  tables: Record<string, string[]>
  structures: Record<string, TableStructure>
  activeNamespace: Record<string, Namespace>
}

export function buildTree(input: TreeInput): TreeNode[] {
  return input.profiles.map((profile) => {
    const { value: engine, hasSchemas } = engineInfo(profile.engine)

    return {
      id: connectionNodeId(profile.id),
      kind: 'connection',
      label: profile.name,
      profileId: profile.id,
      engine,
      status: input.statuses[profile.id] ?? 'idle',
      children: (input.databases[profile.id] ?? []).map((database) =>
        hasSchemas
          ? schemaParentNode(input, profile.id, database)
          : namespaceNode(input, profile.id, 'database', namespaceOf(database), database),
      ),
    }
  })
}

// A PostgreSQL database holds no tables of its own — its schemas do — so it only carries the
// level below it, and the schema list arrives on first expand.
function schemaParentNode(input: TreeInput, profileId: string, database: string): TreeNode {
  const names = input.schemas[databaseKey(profileId, database)]

  return {
    id: databaseNodeId(profileId, database),
    kind: 'database',
    label: database,
    profileId,
    namespace: namespaceOf(database),
    ...(names ? { badge: String(names.length) } : {}),
    children: (names ?? []).map((schema) =>
      namespaceNode(input, profileId, 'schema', namespaceOf(database, schema), schema),
    ),
  }
}

// The node that owns tables, whichever level of the tree that turns out to be.
function namespaceNode(
  input: TreeInput,
  profileId: string,
  kind: 'database' | 'schema',
  ns: Namespace,
  label: string,
): TreeNode {
  const names = input.tables[namespaceKey(profileId, ns)]
  const active = input.activeNamespace[profileId] ?? NO_NAMESPACE

  return {
    id: kind === 'schema' ? schemaNodeId(profileId, ns) : databaseNodeId(profileId, ns.database),
    kind,
    label,
    profileId,
    namespace: ns,
    active: active.database === ns.database && active.schema === ns.schema,
    children: [
      {
        id: tablesNodeId(profileId, ns),
        kind: 'folder',
        label: 'Tables',
        profileId,
        namespace: ns,
        ...(names ? { badge: String(names.length) } : {}),
        children: (names ?? []).map((table) => tableNode(input, profileId, ns, table)),
      },
    ],
  }
}

// A table carries the three folders whether or not its shape has been read: they are what the
// caret is for, and the read happens when one of them is opened.
function tableNode(
  input: TreeInput,
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
      {
        id: at('foreignKeys'),
        kind: 'foreignKeys',
        label: 'Foreign Keys',
        profileId,
        namespace: ns,
        table,
        ...(structure ? { badge: String(structure.foreignKeys.length) } : {}),
        children: (structure?.foreignKeys ?? []).map((key) => ({
          id: objectNodeId('foreignKey', profileId, ns, table, key.name),
          kind: 'foreignKey',
          label: key.name,
          profileId,
          namespace: ns,
          table,
          badge: `→ ${key.referencedTable}`,
        })),
      },
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
