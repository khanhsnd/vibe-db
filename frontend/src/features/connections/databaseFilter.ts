/**
 * Which of a connection's databases the tree draws.
 *
 * A server with three hundred of them is a server whose branch nobody can read, so a reader names
 * the part they work on once and the sidebar keeps that narrowing for good. It is a fact about how
 * they *read* rather than about the connection — nothing on the server changes and no other reader
 * of the list is narrowed by it — which is why it is stored beside the column orders and not in the
 * profile the server is dialled from.
 *
 * A pattern is a **substring**, matched case-insensitively, exactly as the sidebar's own filter box
 * matches a name: one notation for "holds this text" in both places, and there is no second one to
 * pick between. The two compose — this one decides what the connection has, that one searches what
 * is left. It matches **names this app has already listed**, in the browser and never on the server:
 * nothing typed here is spelled into a statement.
 */

const STORE_KEY = 'vibe-db.databaseFilters'

/** Every stored pattern in one entry, keyed by connection, as a reader has one per server. */
export function loadDatabaseFilters(): Record<string, string> {
  const raw = localStorage.getItem(STORE_KEY)
  if (raw === null) return {}

  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const filters: Record<string, string> = {}

    // Read a field at a time rather than trusted whole: what is in the entry was written by a
    // version of this app, and one of them wrote something else there.
    for (const [profileId, held] of Object.entries(parsed)) {
      if (typeof held === 'string' && held !== '') filters[profileId] = held
    }

    return filters
  } catch {
    return {}
  }
}

export function saveDatabaseFilters(filters: Record<string, string>): void {
  localStorage.setItem(STORE_KEY, JSON.stringify(filters))
}

/**
 * The names that hold `pattern` — and the very same list where nothing was typed, so a connection
 * nobody has narrowed costs the tree neither a copy nor a re-render.
 */
export function filterDatabases(names: string[], pattern: string): string[] {
  const needle = pattern.trim().toLowerCase()
  if (needle === '') return names

  return names.filter((name) => name.toLowerCase().includes(needle))
}
