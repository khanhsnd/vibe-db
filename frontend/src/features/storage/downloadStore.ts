import { create } from 'zustand'

import { DEFAULT_COLLISION, type DownloadObject } from '@/bridge/objects'

/**
 * What a download is waiting to be told: where to put it, and what to do about a file that is
 * already there. It is a store rather than a prop because the dialog is mounted once, beside the
 * others in `App` — the arrangement `stores/backups.ts` has with New Backup.
 */
export interface DownloadRequest {
  sessionId: string
  bucket: string
  /** The prefix the reader was standing in, taken off the front of every key. */
  under: string
  /** The objects that were ticked, each carrying what the listing already said about it. */
  keys: DownloadObject[]
  prefixes: string[]
  /** What is being brought down, in the reader's own words, so the dialog can say it back. */
  label: string
}

interface DownloadState {
  request: DownloadRequest | null
  /** The folder the last download went to, offered as the next one's — a reader downloads twice. */
  into: string
  collision: string
  ask: (request: DownloadRequest) => void
  dismiss: () => void
  setInto: (into: string) => void
  setCollision: (collision: string) => void
}

export const useDownloadStore = create<DownloadState>((set) => ({
  request: null,
  into: '',
  collision: DEFAULT_COLLISION,
  ask: (request) => set({ request }),
  dismiss: () => set({ request: null }),
  setInto: (into) => set({ into }),
  setCollision: (collision) => set({ collision }),
}))
