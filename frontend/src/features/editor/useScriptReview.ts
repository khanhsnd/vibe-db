import { useEffect, useState } from 'react'

import { NO_REVIEW, queries, type ScriptReview } from '@/bridge/query'

/**
 * How long the editor waits after a keystroke before asking what the text now says. It is short
 * enough that a reader who stops typing sees the answer immediately, and long enough that typing a
 * word is one call and not five. The call itself is local — no server is reached — so this is
 * pacing the *render*, not the network.
 */
const SETTLE_MS = 250

/**
 * What the script in the editor says about itself: which statements will have to be answered for,
 * and what the lexer can already prove is wrong with it.
 *
 * It needs a session because where a statement ends is the *engine's* — a semicolon inside a
 * `$body$` ends nothing on PostgreSQL — and because the environment that decides what is confirmed
 * belongs to the connection. A tab with nothing dialled therefore reviews nothing, which is the
 * same tab whose Execute is already disabled.
 */
export function useScriptReview(sessionId: string | null, sql: string): ScriptReview {
  const [review, setReview] = useState<ScriptReview>(NO_REVIEW)

  useEffect(() => {
    if (sessionId === null || sql.trim() === '') {
      setReview(NO_REVIEW)
      return
    }

    let live = true
    const timer = setTimeout(() => {
      void queries
        .review(sessionId, sql)
        .then((read) => live && setReview(read))
        // A review that fails says nothing rather than saying something wrong: the marks are an
        // aid, and a stale one over text that has moved on is worse than none.
        .catch(() => live && setReview(NO_REVIEW))
    }, SETTLE_MS)

    return () => {
      live = false
      clearTimeout(timer)
    }
  }, [sessionId, sql])

  return review
}
