import { useCallback, useEffect, useState } from 'react'

import { compare } from '@/bridge/compare'
import { toAppError, type AppError } from '@/bridge/errors'
import { useConnectionsStore } from '@/stores/connections'
import type { TableTarget } from '@/stores/tabs'

import { dataWrites, structureChanges, type Direction, type Ends } from './migrate'
import type { Comparison } from './useCompare'

export interface Migration {
  /** Exactly what Apply will run, spelled by the engine — see `CompareScript`. */
  statements: readonly string[]
  applying: boolean
  error: AppError | null
  apply: () => Promise<void>
  /** The two tables' own declarations, read once per pair and shown in the DDL Diff. */
  ddl: { left: string[] | null; right: string[] | null; loading: boolean }
}

/**
 * What the migration would do, and the doing of it.
 *
 * **The script is asked for and never spelled here.** The ticked pairs become a list of changes,
 * and that same list goes to `previewChanges` to be read and to `applyChanges` to be run — so the
 * statements the reader copied are the statements that ran. Nothing on this side has ever spelled
 * one, which is the rule that makes that true.
 */
export function useMigration(
  comparison: Comparison,
  ends: Ends,
  taken: ReadonlySet<string>,
  into: TableTarget,
  direction: Direction,
): Migration {
  const session = useConnectionsStore((s) => s.sessions[into.profileId]?.id ?? '')
  const leftSession = useConnectionsStore((s) => s.sessions[comparison.left.profileId]?.id ?? '')
  const rightSession = useConnectionsStore((s) => s.sessions[comparison.right.profileId]?.id ?? '')

  const [statements, setStatements] = useState<readonly string[]>([])
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const { structure, data } = comparison

  // What Apply will run. It is rebuilt whenever a tick or the direction moves, because that is
  // exactly when the answer changes — and a preview that lagged behind the ticks would be the one
  // thing this pane must never be.
  useEffect(() => {
    if (session === '' || taken.size === 0) {
      setStatements([])
      return
    }

    let live = true
    const read = structure
      ? compare.previewChanges(
          session,
          into.namespace,
          into.table,
          structureChanges(structure, ends, taken),
        )
      : data
        ? Promise.all(
            dataWrites(data, ends, taken, into).map((write) => compare.previewRows(session, write)),
          ).then((lists) => lists.flat())
        : Promise.resolve([])

    void read
      .then((spelled) => {
        if (live) setStatements(spelled)
      })
      .catch((thrown) => {
        if (live) {
          setStatements([])
          setError(toAppError(thrown))
        }
      })

    return () => {
      live = false
    }
  }, [session, structure, data, taken, direction, into.namespace, into.table, ends])

  const apply = useCallback(async () => {
    if (session === '') return

    setApplying(true)
    setError(null)
    try {
      if (structure) {
        await compare.applyChanges(session, into.namespace, into.table, structureChanges(structure, ends, taken))
      } else if (data) {
        for (const write of dataWrites(data, ends, taken, into)) {
          await compare.applyRows(session, write)
        }
      }
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setApplying(false)
    }
  }, [session, structure, data, ends, taken, into])

  const ddl = useDeclarations(
    leftSession,
    comparison.left,
    rightSession,
    comparison.right,
    comparison.connected,
    comparison.token,
  )

  return { statements, applying, error, apply, ddl }
}

/**
 * Both tables' own declarations. They are read once per pair rather than with the comparison,
 * because they answer a different question: the comparison says what *this app* can tell apart, and
 * a declaration says what the table **is** — a storage engine, a character set, an auto-increment,
 * none of which a catalogue comparison can see.
 */
function useDeclarations(
  leftSession: string,
  left: TableTarget,
  rightSession: string,
  right: TableTarget,
  connected: boolean,
  token: number,
): Migration['ddl'] {
  const [ours, setOurs] = useState<string[] | null>(null)
  const [theirs, setTheirs] = useState<string[] | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!connected) return

    let live = true
    setLoading(true)
    setOurs(null)
    setTheirs(null)

    void Promise.all([
      compare.createStatements(leftSession, left.namespace, left.table),
      compare.createStatements(rightSession, right.namespace, right.table),
    ])
      .then(([one, other]) => {
        if (!live) return
        setOurs(one)
        setTheirs(other)
      })
      // A declaration that could not be read leaves the pane saying so rather than the tab
      // failing: the comparison above it is what the reader opened this for.
      .catch(() => {})
      .finally(() => {
        if (live) setLoading(false)
      })

    return () => {
      live = false
    }
  }, [connected, token, leftSession, rightSession, left.namespace, left.table, right.namespace, right.table])

  return { left: ours, right: theirs, loading }
}
