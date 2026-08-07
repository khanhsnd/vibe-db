import { engineInfo } from '@/bridge/connections'
import type { LogEntry } from '@/bridge/log'

/**
 * What a line of the log *is*, which is the only thing that decides its colour: the heading that
 * opens an entry, the statement itself, the note that closes it, or the server's refusal.
 */
export type LineTone = 'head' | 'sql' | 'note' | 'error'

export interface LogLine {
  text: string
  tone: LineTone
}

/**
 * The log as the lines it is read as. An entry is a heading, the statement as it was written —
 * newlines and all, which is why this is lines rather than rows — and what became of it, with a
 * blank between entries so the eye can find where one ends.
 *
 * It is deliberately not a `ResultGrid`: a statement is not a cell. What a reader does here is read
 * a hundred lines of SQL down the page, and a grid would show them one squashed line at a time.
 */
export function logLines(entries: LogEntry[]): LogLine[] {
  const lines: LogLine[] = []

  for (const entry of entries) {
    if (lines.length > 0) lines.push({ text: '', tone: 'note' })

    lines.push({ text: heading(entry), tone: 'head' })
    for (const line of entry.sql.split('\n')) lines.push({ text: line, tone: 'sql' })

    if (entry.error !== '') lines.push({ text: entry.error, tone: 'error' })
    lines.push({ text: `Time: ${(entry.elapsedMs / 1000).toFixed(3)}s`, tone: 'note' })
  }

  return lines
}

/**
 * When it ran, on what, where, and against which engine — the four facts that say which of two
 * identical statements this one was. A fact the app has not got is left out rather than drawn
 * empty: a session that has closed leaves no connection name, and an engine with no schemas has
 * no second half to its namespace.
 */
function heading(entry: LogEntry): string {
  const where = entry.schema === '' ? entry.database : `${entry.database}.${entry.schema}`
  const parts = [
    stamp(entry.ranAt),
    entry.connection,
    where,
    entry.engine === '' ? '' : engineInfo(entry.engine).label.toUpperCase(),
  ]

  return parts
    .filter((part) => part !== '')
    .map((part) => `[${part}]`)
    .join('')
}

/**
 * The clock the log is read by: the reader's own time zone, down to the millisecond. A log is read
 * to find out what ran between two things the reader watched happen, and two statements a tenth of
 * a second apart are exactly the pair that question is usually about.
 */
function stamp(ms: number): string {
  const at = new Date(ms)
  const pad = (value: number, width = 2) => String(value).padStart(width, '0')

  return (
    `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ` +
    `${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}.` +
    pad(at.getMilliseconds(), 3)
  )
}

/**
 * Whether an entry holds the reader's text, matched however either is cased and across everything
 * the line says: a reader looking for `db-gold` is looking for a connection, and one looking for
 * `users` is looking for a statement.
 */
export function matches(entry: LogEntry, find: string): boolean {
  const wanted = find.trim().toLowerCase()
  if (wanted === '') return true

  return [entry.sql, entry.connection, entry.database, entry.schema, entry.error].some((part) =>
    part.toLowerCase().includes(wanted),
  )
}
