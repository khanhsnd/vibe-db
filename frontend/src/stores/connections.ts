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
import type { ConnectionStatus, TreeNode } from '@/features/connections/tree'

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
  toggleNode: (nodeId: string) => void
  clearError: () => void
}

export const useConnectionsStore = create<ConnectionsState>((set, get) => ({
  profiles: [],
  sessions: {},
  statuses: {},
  databases: {},
  schemas: {},
  tables: {},
  tableSchemas: {},
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

  toggleNode: (nodeId) =>
    set((s) => {
      const expanded = new Set(s.expanded)
      if (expanded.has(nodeId)) expanded.delete(nodeId)
      else expanded.add(nodeId)
      return { expanded }
    }),

  clearError: () => set({ error: null }),
}))

interface TreeInput {
  profiles: Profile[]
  statuses: Record<string, ConnectionStatus>
  databases: Record<string, string[]>
  schemas: Record<string, string[]>
  tables: Record<string, string[]>
  activeNamespace: Record<string, Namespace>
}

export function buildTree({
  profiles,
  statuses,
  databases,
  schemas,
  tables,
  activeNamespace,
}: TreeInput): TreeNode[] {
  return profiles.map((profile) => {
    const { label, hasSchemas } = engineInfo(profile.engine)
    const active = activeNamespace[profile.id] ?? NO_NAMESPACE

    return {
      id: connectionNodeId(profile.id),
      kind: 'connection',
      label: profile.name,
      profileId: profile.id,
      status: statuses[profile.id] ?? 'idle',
      badge: label,
      children: (databases[profile.id] ?? []).map((database) =>
        hasSchemas
          ? schemaParentNode(profile.id, database, schemas, tables, active)
          : namespaceNode(profile.id, 'database', namespaceOf(database), database, tables, active),
      ),
    }
  })
}

// A PostgreSQL database holds no tables of its own — its schemas do — so it only carries the
// level below it, and the schema list arrives on first expand.
function schemaParentNode(
  profileId: string,
  database: string,
  schemas: Record<string, string[]>,
  tables: Record<string, string[]>,
  active: Namespace,
): TreeNode {
  const names = schemas[databaseKey(profileId, database)]

  return {
    id: databaseNodeId(profileId, database),
    kind: 'database',
    label: database,
    profileId,
    namespace: namespaceOf(database),
    ...(names ? { badge: String(names.length) } : {}),
    children: (names ?? []).map((schema) =>
      namespaceNode(profileId, 'schema', namespaceOf(database, schema), schema, tables, active),
    ),
  }
}

// The node that owns tables, whichever level of the tree that turns out to be.
function namespaceNode(
  profileId: string,
  kind: 'database' | 'schema',
  ns: Namespace,
  label: string,
  tables: Record<string, string[]>,
  active: Namespace,
): TreeNode {
  const names = tables[namespaceKey(profileId, ns)]

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
        children: (names ?? []).map((table) => ({
          id: tableNodeId(profileId, ns, table),
          kind: 'table',
          label: table,
          profileId,
          namespace: ns,
        })),
      },
    ],
  }
}
