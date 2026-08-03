import { useMemo } from 'react'

import type { StatementOutcome } from '@/bridge/query'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { Cell, GridData } from '@/features/results/resultSet'

/**
 * What each statement of a script did, in the order they ran. It is a `ResultGrid` because it is
 * rows and columns, and that is the one component for those.
 *
 * A rejection reaches this grid rather than the error panel: the statements above it ran, and a
 * panel that said only "failed" would hide how far the script got. One statement on its own still
 * fails the way it always has — see `QueryAPI.Run`.
 */
export function ScriptMessages({ statements }: { statements: StatementOutcome[] }) {
  const result = useMemo(() => messageRows(statements), [statements])

  return <ResultGrid result={result} />
}

function messageRows(statements: StatementOutcome[]): GridData {
  return {
    columns: [
      column('#', 'int'),
      column('Statement'),
      column('Rows', 'int'),
      column('Affected', 'int'),
      column('Time (ms)', 'int'),
      column('Message'),
    ],
    rows: statements.map((statement, at): Cell[] => [
      String(at + 1),
      oneLine(statement.sql),
      String(statement.rows),
      String(statement.rowsAffected),
      String(statement.elapsedMs),
      statement.error === '' ? 'OK' : `${statement.errorCode}: ${statement.error}`,
    ]),
  }
}

/** A statement is a cell here, and a cell is one line: the editor above still holds the whole. */
function oneLine(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim()
}

const column = (name: string, type = 'text') => ({ name, type, fullType: type })
