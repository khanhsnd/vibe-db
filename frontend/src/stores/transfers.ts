import { create } from 'zustand'

import { finished, objects, onTransfers, type TransferReport } from '@/bridge/objects'

/**
 * Every transfer this app has been asked for. It belongs to the **window** and not to a tab, which
 * is the first thing a reader finds out by closing the tab they started a download from: the bytes
 * are still moving, so the panel that watches them cannot be the tab's.
 *
 * Nothing here polls. The report is pushed while anything is moving (`onTransfers`) and read once on
 * open, so a panel nobody is looking at costs nothing at all — see `docs/features/object-download.md`.
 */
interface TransfersState {
  report: TransferReport
  /** Whether the dock at the foot of the work area is open. */
  shown: boolean
  /** How many transfers have finished since the panel was last looked at, drawn as its badge. */
  unseen: number
  listening: boolean

  /** Starts listening, and is idempotent: every surface that shows a transfer calls it. */
  watch: () => void
  /**
   * What to do when one transfer ends, run **once**. It is here rather than in whoever asked for the
   * transfer because the end of one is something only the report knows — and a bucket whose upload has
   * landed is holding an object the grid has not read.
   */
  whenFinished: (jobId: string, then: () => void) => void
  refresh: () => Promise<void>
  show: (shown: boolean) => void
  cancel: (jobId: string) => Promise<void>
  cancelAll: () => Promise<void>
  clear: () => Promise<void>
}

const NOTHING: TransferReport = { jobs: [], files: [], active: 0 }

/** The waiting `whenFinished` callbacks, by job. It is module state rather than a field of the store
 *  because nothing draws it, and a closure in a store is a closure nothing can render. */
const waiting = new Map<string, () => void>()

export const useTransfersStore = create<TransfersState>((set, get) => ({
  report: NOTHING,
  shown: false,
  unseen: 0,
  listening: false,

  watch: () => {
    if (get().listening) return
    set({ listening: true })

    onTransfers((report) => {
      const before = get().report.jobs
      // A job that has just finished is what the badge counts, and it is counted here rather than
      // in the panel because the panel is usually shut when it happens.
      const settled = report.jobs.filter(
        (job) =>
          finished(job.state) &&
          !before.some((was) => was.id === job.id && finished(was.state)),
      )

      set((s) => ({
        report,
        unseen: s.shown ? 0 : s.unseen + settled.length,
      }))

      // The same moment, told to whoever asked to hear it. A job is dropped from the list as it is
      // fired, so a report that names a finished job twice cannot run it twice.
      for (const job of settled) {
        const then = waiting.get(job.id)
        if (then === undefined) continue
        waiting.delete(job.id)
        then()
      }
    })
  },

  whenFinished: (jobId, then) => {
    waiting.set(jobId, then)
  },

  refresh: async () => {
    const report = await objects.transfers()
    set({ report })
  },

  show: (shown) => set({ shown, unseen: shown ? 0 : 0 }),

  cancel: async (jobId) => {
    await objects.cancelTransfer(jobId)
    await get().refresh()
  },

  cancelAll: async () => {
    await objects.cancelTransfers()
    await get().refresh()
  },

  clear: async () => {
    await objects.clearTransfers()
    await get().refresh()
  },
}))
