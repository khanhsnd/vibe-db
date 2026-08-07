import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'

/** Which namespace a backup is being taken of: the connection, and the database or schema. */
export interface BackupTarget {
  profileId: string
  namespace: Namespace
}

/** Which backup is being run back. The file itself is looked up by name — see `RestoreTarget`. */
export interface RestoreTarget extends BackupTarget {
  name: string
}

interface BackupsState {
  /** Where a new backup is being taken, and non-null exactly while that dialog is up. */
  target: BackupTarget | null
  /** Which backup is being restored, and non-null exactly while that dialog is up. */
  restoring: RestoreTarget | null
  ask: (target: BackupTarget) => void
  close: () => void
  askRestore: (target: RestoreTarget) => void
  closeRestore: () => void
}

/**
 * The two backup dialogs' own slots, and nothing else.
 *
 * **The backups themselves are not here.** They are in `stores/connections` beside the tree's other
 * names, because the tree draws them: two copies of one folder would be two things to keep in step,
 * and the one thing worse than a stale list is two lists disagreeing about which files exist. It is
 * also why a restore is asked for **by name**: the row it names — its comment, its size, and which
 * tables it holds — is already in that one list, so the dialog reads it there rather than being
 * handed a copy that could go stale between the click and the answer.
 */
export const useBackupsStore = create<BackupsState>((set) => ({
  target: null,
  restoring: null,
  ask: (target) => set({ target }),
  close: () => set({ target: null }),
  askRestore: (restoring) => set({ restoring }),
  closeRestore: () => set({ restoring: null }),
}))
