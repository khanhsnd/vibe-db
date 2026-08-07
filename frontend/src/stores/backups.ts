import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'

/** Which namespace a backup is being taken of: the connection, and the database or schema. */
export interface BackupTarget {
  profileId: string
  namespace: Namespace
}

interface BackupsState {
  /** Where a new backup is being taken, and non-null exactly while that dialog is up. */
  target: BackupTarget | null
  ask: (target: BackupTarget) => void
  close: () => void
}

/**
 * The New Backup dialog's own slot, and nothing else.
 *
 * **The backups themselves are not here.** They are in `stores/connections` beside the tree's other
 * names, because the tree draws them: two copies of one folder would be two things to keep in step,
 * and the one thing worse than a stale list is two lists disagreeing about which files exist.
 */
export const useBackupsStore = create<BackupsState>((set) => ({
  target: null,
  ask: (target) => set({ target }),
  close: () => set({ target: null }),
}))
