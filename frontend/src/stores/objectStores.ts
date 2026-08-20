import { create } from 'zustand'

import { objects, toStoreInput, type BucketRow, type Store, type StoreConnection, type StoreInput } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import type { ConnectionStatus } from '@/features/connections/tree'

/**
 * The object-store connections, and it is `stores/connections.ts`'s counterpart rather than a slice
 * of it: a store has no databases, no schemas and no tables, so every one of that store's fifteen
 * maps would have been empty here.
 *
 * What the two **do** share is the shape a reader sees — a list of profiles, a status per profile, a
 * session per profile — because that is what the sidebar draws either kind of connection as.
 */
interface ObjectStoresState {
  profiles: Store[]
  /** By profile id, exactly as a database connection's is. */
  statuses: Record<string, ConnectionStatus>
  sessions: Record<string, StoreConnection>
  /** The buckets a connection could list, by profile id. */
  buckets: Record<string, BucketRow[]>
  loading: boolean
  error: AppError | null

  load: () => Promise<void>
  save: (input: StoreInput) => Promise<Store>
  setColour: (profileId: string, colour: string) => Promise<void>
  rename: (profileId: string, name: string) => Promise<void>
  remove: (profileId: string) => Promise<void>
  connect: (profileId: string, secretKey: string) => Promise<void>
  disconnect: (profileId: string) => Promise<void>
  loadBuckets: (profileId: string) => Promise<void>
  clearError: () => void
}

export const useObjectStoresStore = create<ObjectStoresState>((set, get) => ({
  profiles: [],
  statuses: {},
  sessions: {},
  buckets: {},
  loading: false,
  error: null,

  load: async () => {
    set({ loading: true })
    try {
      const profiles = await objects.listStores()
      set({ profiles, loading: false, error: null })
    } catch (thrown) {
      set({ loading: false, error: toAppError(thrown) })
    }
  },

  save: async (input) => {
    const saved = await objects.saveStore(input)
    await get().load()
    return saved
  },

  // A blank secret in a saved profile keeps the stored one, so re-saving the profile a colour was
  // picked on is the whole of what marking one costs — see StoreAPI.SaveStore.
  setColour: async (profileId, colour) => {
    const profile = get().profiles.find((one) => one.id === profileId)
    if (!profile) return
    await get().save({ ...toStoreInput(profile), colour })
  },

  rename: async (profileId, name) => {
    const profile = get().profiles.find((one) => one.id === profileId)
    if (!profile) return
    await get().save({ ...toStoreInput(profile), name })
  },

  remove: async (profileId) => {
    if (get().sessions[profileId]) await get().disconnect(profileId)
    await objects.deleteStore(profileId)
    await get().load()
  },

  /**
   * Opening a connection **reads its buckets in the same breath**, which is the one thing worth
   * saying about it: opening a store answers nothing by itself — every call to the service is a
   * request of its own — so a connection that could not reach anything would otherwise sit in the
   * tree looking open.
   *
   * A profile that names one bucket lists that bucket alone, because a key that may read one bucket
   * and not list them is the ordinary arrangement in an organisation.
   */
  connect: async (profileId, secretKey) => {
    set((s) => ({ statuses: { ...s.statuses, [profileId]: 'connecting' }, error: null }))

    try {
      const session = await objects.connect(profileId, secretKey)
      const listed = session.bucket
        ? [{ name: session.bucket, createdAt: '', region: '' }]
        : await objects.listBuckets(session.sessionId)

      set((s) => ({
        sessions: { ...s.sessions, [profileId]: session },
        statuses: { ...s.statuses, [profileId]: 'connected' },
        buckets: { ...s.buckets, [profileId]: listed },
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
      await objects.disconnect(session.sessionId)
    } finally {
      set((s) => {
        const sessions = { ...s.sessions }
        const buckets = { ...s.buckets }
        const statuses = { ...s.statuses }
        delete sessions[profileId]
        delete buckets[profileId]
        delete statuses[profileId]
        return { sessions, buckets, statuses }
      })
    }
  },

  loadBuckets: async (profileId) => {
    const session = get().sessions[profileId]
    if (!session) return

    try {
      const listed = await objects.listBuckets(session.sessionId)
      set((s) => ({ buckets: { ...s.buckets, [profileId]: listed }, error: null }))
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    }
  },

  clearError: () => set({ error: null }),
}))

/** The id of a store connection's row in the tree, which is what an expand is remembered under. */
export function storeNodeId(profileId: string): string {
  return `store:${profileId}`
}

export function bucketNodeId(profileId: string, bucket: string): string {
  return `store:${profileId}:${bucket}`
}
