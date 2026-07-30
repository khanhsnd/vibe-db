import { create } from 'zustand'

import { type AppError, toAppError } from '@/bridge/errors'
import { connections, type Profile, type ProfileInput, type Session } from '@/bridge/connections'
import type { ConnectionStatus, TreeNode } from '@/features/connections/tree'

export const connectionNodeId = (profileId: string) => `conn:${profileId}`
export const databaseNodeId = (profileId: string, database: string) => `db:${profileId}:${database}`

interface ConnectionsState {
  profiles: Profile[]
  sessions: Record<string, Session>
  statuses: Record<string, ConnectionStatus>
  databases: Record<string, string[]>
  expanded: ReadonlySet<string>
  loading: boolean
  error: AppError | null

  loadProfiles: () => Promise<void>
  saveProfile: (input: ProfileInput) => Promise<Profile>
  deleteProfile: (profileId: string) => Promise<void>
  connect: (profileId: string, password: string) => Promise<void>
  disconnect: (profileId: string) => Promise<void>
  toggleNode: (nodeId: string) => void
  clearError: () => void
}

export const useConnectionsStore = create<ConnectionsState>((set, get) => ({
  profiles: [],
  sessions: {},
  statuses: {},
  databases: {},
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
        delete sessions[profileId]
        delete databases[profileId]

        const expanded = new Set(s.expanded)
        expanded.delete(connectionNodeId(profileId))

        return {
          sessions,
          databases,
          expanded,
          statuses: { ...s.statuses, [profileId]: 'idle' },
        }
      })
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

export function buildTree(
  profiles: Profile[],
  statuses: Record<string, ConnectionStatus>,
  databases: Record<string, string[]>,
): TreeNode[] {
  return profiles.map((profile) => ({
    id: connectionNodeId(profile.id),
    kind: 'connection',
    label: profile.name,
    status: statuses[profile.id] ?? 'idle',
    children: (databases[profile.id] ?? []).map((database) => ({
      id: databaseNodeId(profile.id, database),
      kind: 'database',
      label: database,
      children: [],
    })),
  }))
}

export function profileIdFromNode(nodeId: string): string | null {
  const [kind, profileId] = nodeId.split(':')
  return kind === 'conn' && profileId ? profileId : null
}
