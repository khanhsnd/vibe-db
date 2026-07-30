import { create } from 'zustand'

import { type AppError, toAppError } from '@/bridge/errors'
import { connections, type Profile, type ProfileInput, type Session } from '@/bridge/connections'
import type { ConnectionStatus, TreeNode } from '@/features/connections/tree'

export const connectionNodeId = (profileId: string) => `conn:${profileId}`
export const databaseNodeId = (profileId: string, database: string) => `db:${profileId}:${database}`
export const tablesNodeId = (profileId: string, database: string) =>
  `tables:${profileId}:${database}`
export const tableNodeId = (profileId: string, database: string, table: string) =>
  `table:${profileId}:${database}:${table}`

const tablesKey = (profileId: string, database: string) => `${profileId}:${database}`

interface ConnectionsState {
  profiles: Profile[]
  sessions: Record<string, Session>
  statuses: Record<string, ConnectionStatus>
  databases: Record<string, string[]>
  tables: Record<string, string[]>
  activeDatabase: Record<string, string>
  expanded: ReadonlySet<string>
  loading: boolean
  error: AppError | null

  loadProfiles: () => Promise<void>
  saveProfile: (input: ProfileInput) => Promise<Profile>
  deleteProfile: (profileId: string) => Promise<void>
  connect: (profileId: string, password: string) => Promise<void>
  disconnect: (profileId: string) => Promise<void>
  selectDatabase: (profileId: string, database: string) => void
  loadTables: (profileId: string, database: string) => Promise<void>
  toggleNode: (nodeId: string) => void
  clearError: () => void
}

export const useConnectionsStore = create<ConnectionsState>((set, get) => ({
  profiles: [],
  sessions: {},
  statuses: {},
  databases: {},
  tables: {},
  activeDatabase: {},
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
        const activeDatabase = { ...s.activeDatabase }
        delete sessions[profileId]
        delete databases[profileId]
        delete activeDatabase[profileId]

        const tables = Object.fromEntries(
          Object.entries(s.tables).filter(([key]) => !key.startsWith(`${profileId}:`)),
        )

        const expanded = new Set(s.expanded)
        expanded.delete(connectionNodeId(profileId))

        return {
          sessions,
          databases,
          tables,
          activeDatabase,
          expanded,
          statuses: { ...s.statuses, [profileId]: 'idle' },
        }
      })
    }
  },

  selectDatabase: (profileId, database) =>
    set((s) => ({
      activeDatabase: { ...s.activeDatabase, [profileId]: database },
      expanded: new Set(s.expanded).add(databaseNodeId(profileId, database)),
    })),

  loadTables: async (profileId, database) => {
    const { sessions, tables } = get()
    const session = sessions[profileId]
    const key = tablesKey(profileId, database)
    if (!session || tables[key]) return

    try {
      const names = await connections.listTables(session.id, database)
      set((s) => ({ tables: { ...s.tables, [key]: names } }))
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
  tables: Record<string, string[]>
  activeDatabase: Record<string, string>
}

export function buildTree({
  profiles,
  statuses,
  databases,
  tables,
  activeDatabase,
}: TreeInput): TreeNode[] {
  return profiles.map((profile) => ({
    id: connectionNodeId(profile.id),
    kind: 'connection',
    label: profile.name,
    profileId: profile.id,
    status: statuses[profile.id] ?? 'idle',
    children: (databases[profile.id] ?? []).map((database) =>
      databaseNode(profile.id, database, tables, activeDatabase[profile.id] === database),
    ),
  }))
}

function databaseNode(
  profileId: string,
  database: string,
  tables: Record<string, string[]>,
  active: boolean,
): TreeNode {
  const names = tables[tablesKey(profileId, database)]

  return {
    id: databaseNodeId(profileId, database),
    kind: 'database',
    label: database,
    profileId,
    database,
    active,
    children: [
      {
        id: tablesNodeId(profileId, database),
        kind: 'folder',
        label: 'Tables',
        profileId,
        database,
        ...(names ? { badge: String(names.length) } : {}),
        children: (names ?? []).map((table) => ({
          id: tableNodeId(profileId, database, table),
          kind: 'table',
          label: table,
          profileId,
          database,
        })),
      },
    ],
  }
}
