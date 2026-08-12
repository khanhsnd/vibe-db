import * as api from '@wailsjs/go/app/ConnectionAPI'

import type { Namespace } from './connections'
import { call } from './errors'

/**
 * The reader's own transaction: what they have done becomes true when they say so, rather than the
 * moment each statement runs.
 *
 * It belongs to the **connection** and therefore to every tab open on it, which is the only honest
 * way to hold one: uncommitted rows exist on one connection and nowhere else, so a second tab
 * reading the same table over the same session is reading them too. Every call answers with whether
 * one is now open, so nothing here ever has to ask twice — see `stores/transactions.ts`.
 */
export const transactions = {
  /**
   * Opens one, in the namespace the reader is looking at, so an unqualified name inside it resolves
   * where they expect.
   */
  begin: (sessionId: string, namespace: Namespace): Promise<boolean> =>
    call(() => api.BeginTransaction(sessionId, namespace)),

  commit: (sessionId: string): Promise<boolean> => call(() => api.CommitTransaction(sessionId)),

  rollback: (sessionId: string): Promise<boolean> => call(() => api.RollbackTransaction(sessionId)),

  /** What a reader returning to a tab is drawn from. It asks no server anything. */
  isOpen: (sessionId: string): Promise<boolean> => call(() => api.InTransaction(sessionId)),
}
