import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { transactions } from '@/bridge/transactions'

interface TransactionState {
  /**
   * Which connections are holding a transaction open, by session id. It is keyed by *session* and
   * not by tab because that is what a transaction is — one connection — so two tabs open on the
   * same connection are inside the same transaction and both say so.
   */
  open: Record<string, boolean>
  /** Whether one of the three is still in flight, so the menu cannot be pressed twice. */
  busy: Record<string, boolean>
  /**
   * What the last Begin, Commit or Rollback failed with. It is held here rather than thrown at the
   * caller because the surfaces already draw a failure the same way — see `ErrorStrip` — and a
   * transaction belongs to the connection rather than to whichever toolbar was pressed.
   */
  errors: Record<string, AppError | null>

  begin: (sessionId: string, namespace: Namespace) => Promise<void>
  commit: (sessionId: string) => Promise<void>
  rollback: (sessionId: string) => Promise<void>
  /** Forgets a connection that has been closed, so a session id reused later starts clean. */
  forget: (sessionId: string) => void
}

/**
 * What the transaction menu draws itself from. The Go side is the authority — it is the one holding
 * the connection — so every call writes back exactly what it answered, **including beside a
 * failure**: a commit the server refused has still ended the transaction, and a menu that went on
 * offering Rollback would be offering something that is no longer there.
 */
export const useTransactionsStore = create<TransactionState>((set) => {
  const settle = async (sessionId: string, action: () => Promise<boolean>) => {
    set((s) => ({ busy: { ...s.busy, [sessionId]: true } }))

    let open = false
    let failure: AppError | null = null
    try {
      open = await action()
    } catch (thrown) {
      failure = toAppError(thrown)
      open = await stillOpen(sessionId)
    }

    set((s) => ({
      open: { ...s.open, [sessionId]: open },
      errors: { ...s.errors, [sessionId]: failure },
      busy: { ...s.busy, [sessionId]: false },
    }))
  }

  return {
    open: {},
    busy: {},
    errors: {},

    begin: (sessionId, namespace) =>
      settle(sessionId, () => transactions.begin(sessionId, namespace)),
    commit: (sessionId) => settle(sessionId, () => transactions.commit(sessionId)),
    rollback: (sessionId) => settle(sessionId, () => transactions.rollback(sessionId)),

    forget: (sessionId) =>
      set((s) => ({
        open: without(s.open, sessionId),
        busy: without(s.busy, sessionId),
        errors: without(s.errors, sessionId),
      })),
  }
})

/**
 * Asks the Go side again after a failure, because only it knows what is still held. A connection
 * that has gone away answers `false` rather than throwing, so a session nobody can reach is drawn
 * as holding nothing — which is true.
 */
async function stillOpen(sessionId: string): Promise<boolean> {
  try {
    return await transactions.isOpen(sessionId)
  } catch {
    return false
  }
}

function without<T>(held: Record<string, T>, key: string): Record<string, T> {
  if (!(key in held)) return held
  const next = { ...held }
  delete next[key]
  return next
}
