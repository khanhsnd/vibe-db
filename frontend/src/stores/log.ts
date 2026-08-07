import { create } from 'zustand'

import { statementLog, type LogEntry } from '@/bridge/log'

/**
 * The History Log, which is one log for the whole window rather than one per tab: it says what
 * *this app* has been doing, across every connection, so there is nothing for a second copy of it
 * to show.
 *
 * The entries are held here rather than in the view because only the tab in front is mounted, and a
 * log the reader had to re-read every time they looked at another tab would lose the place they had
 * scrolled to. What is held is exactly what the backend holds — see `bridge/log.ts`; this only
 * remembers how far it has read.
 */
interface LogState {
  entries: LogEntry[]
  /** The highest `seq` already held, and the only thing the next read has to be told. */
  read: number
  /**
   * Stops the reading rather than the running. A paused log is the window standing still while the
   * app carries on; resuming asks for everything since, which is why it is a read and not a filter.
   */
  paused: boolean
  wordWrap: boolean
  /** Draws only the statements a server refused. */
  errorsOnly: boolean
  /** Narrows the log to the entries holding this text, matched however it is cased. */
  find: string

  /** Reads whatever has run since the last time, and appends it. */
  poll: () => Promise<void>
  clear: () => Promise<void>
  setPaused: (paused: boolean) => void
  setWordWrap: (wordWrap: boolean) => void
  setErrorsOnly: (errorsOnly: boolean) => void
  setFind: (find: string) => void
}

export const useLogStore = create<LogState>((set, get) => ({
  entries: [],
  read: 0,
  paused: false,
  wordWrap: true,
  errorsOnly: false,
  find: '',

  poll: async () => {
    if (get().paused) return

    // A failed read leaves the log where it is and says nothing. It is a window on what has
    // already happened, and a strip of red over it would be reporting on the reader's own log
    // rather than on anything they asked for.
    const fresh = await statementLog.since(get().read).catch(() => [])

    // What has already been drawn is settled here rather than before the read, so two reads that
    // overlap cannot both append the same entries: an answer is a *claim* about a moment that has
    // passed, and only the numbers still unread are taken from it.
    set((s) => {
      const unread = fresh.filter((entry) => entry.seq > s.read)
      const last = unread[unread.length - 1]
      if (last === undefined) return s

      return { entries: [...s.entries, ...unread], read: last.seq }
    })
  },

  clear: async () => {
    await statementLog.clear()
    // The numbering starts again with the log, so what has been read starts again with it.
    set({ entries: [], read: 0 })
  },

  setPaused: (paused) => set({ paused }),
  setWordWrap: (wordWrap) => set({ wordWrap }),
  setErrorsOnly: (errorsOnly) => set({ errorsOnly }),
  setFind: (find) => set({ find }),
}))
