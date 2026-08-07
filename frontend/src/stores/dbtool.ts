import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'

/** Which of the two programs the dialog is open on. */
export type ToolMode = 'dump' | 'restore'

/** What is being carried out or put back: the connection, and the database or schema. */
export interface ToolTarget {
  profileId: string
  namespace: Namespace
  mode: ToolMode
}

interface ToolsState {
  /** What the dialog is open on, and non-null exactly while it is up. */
  target: ToolTarget | null
  ask: (target: ToolTarget) => void
  close: () => void
}

/**
 * The tool dialog's own slot, and nothing else.
 *
 * There is **one** dialog for both directions rather than two, because they ask the same four
 * questions — which server, which file, which tables, and what to do about what is already there —
 * and a second component would be the same four boxes with a different verb on the button.
 *
 * Nothing about the *programs* is kept here. Whether mongodump is on this machine is a fact about
 * the machine and is read fresh every time the dialog opens, so a reader who installs the tools
 * while it is open presses Locate and carries on rather than restarting the app.
 */
export const useToolsStore = create<ToolsState>((set) => ({
  target: null,
  ask: (target) => set({ target }),
  close: () => set({ target: null }),
}))
