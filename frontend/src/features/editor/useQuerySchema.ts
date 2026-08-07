import { useEffect, useMemo, useState } from 'react'

import { engineInfo, namespaceOf, type Namespace } from '@/bridge/connections'
import { databaseKey, namespaceKey, useConnectionsStore } from '@/stores/connections'

import type { QuerySchema } from './sqlLanguage'

const NO_NAMES: string[] = []
const NO_TABLES: never[] = []

/**
 * What one editor completes from: the namespace it is pointed at, and every **sibling** of that
 * namespace the script reaches into. A join across two databases is an ordinary statement — a
 * qualified name is all it takes — so completion that only knew one namespace went quiet at
 * exactly the point the reader needed it most.
 *
 * A sibling is whatever varies below a connection: a **database** where a table is qualified by
 * one (MySQL, Oracle, SQLite) and a **schema** where it is qualified by that instead (PostgreSQL,
 * SQL Server). A document store and a key space qualify nothing, so neither has siblings.
 *
 * Every sibling is offered by name, but a sibling is only **read** once the script names it: each
 * read is a catalogue scan and a server can hold hundreds of databases, so reading all of them to
 * open an empty tab is a cost nobody asked for.
 */
export function useQuerySchema(
  profileId: string,
  current: Namespace,
  engine: string,
  text: string,
): QuerySchema {
  const info = engineInfo(engine)
  const { hasSchemas } = info
  const qualifies = !info.documents && !info.keys

  const siblings = useConnectionsStore((s) => {
    if (!qualifies) return NO_NAMES
    return hasSchemas
      ? (s.schemas[databaseKey(profileId, current.database)] ?? NO_NAMES)
      : (s.databases[profileId] ?? NO_NAMES)
  })
  const tableSchemas = useConnectionsStore((s) => s.tableSchemas)
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)
  const loadTableSchemas = useConnectionsStore((s) => s.loadTableSchemas)

  // Held rather than derived so that a keystroke which changes nothing about which namespaces the
  // script names hands the editor the very same schema, and reconfigures nothing.
  const [named, setNamed] = useState<string[]>(NO_NAMES)
  useEffect(() => {
    const found = namesUsed(text, siblings)
    setNamed((was) => (same(was, found) ? was : found))
  }, [text, siblings])

  // The sibling list itself, for a database the tree may never have been expanded on. The toolbar's
  // schema picker reads the same list.
  useEffect(() => {
    if (hasSchemas && current.database !== '') void loadSchemas(profileId, current.database)
  }, [hasSchemas, loadSchemas, profileId, current.database])

  useEffect(() => {
    if (hasSchemas ? current.schema !== '' : current.database !== '') {
      void loadTableSchemas(profileId, current)
    }
    for (const name of named) {
      void loadTableSchemas(profileId, siblingOf(current, name, hasSchemas))
    }
  }, [hasSchemas, loadTableSchemas, profileId, current, named])

  return useMemo(
    () => ({
      tables: tableSchemas[namespaceKey(profileId, current)] ?? NO_TABLES,
      qualified: Object.fromEntries(
        siblings.map((name) => [
          name,
          tableSchemas[namespaceKey(profileId, siblingOf(current, name, hasSchemas))] ?? NO_TABLES,
        ]),
      ),
    }),
    [tableSchemas, profileId, current, siblings, hasSchemas],
  )
}

function siblingOf(current: Namespace, name: string, hasSchemas: boolean): Namespace {
  return hasSchemas ? namespaceOf(current.database, name) : namespaceOf(name)
}

/**
 * Which of the siblings the script mentions at all — not which of them it qualifies a table with,
 * because the point is to have the tables in hand by the time the dot is typed. A word that only
 * happens to be spelled like a database costs one catalogue read and nothing else.
 */
function namesUsed(text: string, siblings: string[]): string[] {
  if (siblings.length === 0) return NO_NAMES

  const words = new Set(Array.from(text.matchAll(/[\w$]+/g), ([word]) => word.toLowerCase()))
  return siblings.filter((name) => words.has(name.toLowerCase()))
}

const same = (a: string[], b: string[]) =>
  a.length === b.length && a.every((name, at) => name === b[at])
