import { useEffect, useRef } from 'react'
import { ChevronRight, Loader2 } from 'lucide-react'

import type { StatementOutcome } from '@/bridge/query'
import { cn } from '@/lib/cn'
import type { ConsoleEntry } from '@/stores/console'

/**
 * Everything sent at this console and what came back, oldest first. It is a transcript and not a
 * grid: what it holds is one line per statement — how many rows, how long, or the server's own
 * refusal — and the *rows* those statements returned are in the grid below it, which is the one
 * thing in this app that draws rows and columns.
 */
export function ConsoleLog({ entries, running }: { entries: ConsoleEntry[]; running: boolean }) {
  const end = useRef<HTMLDivElement>(null)

  // A console scrolls itself: what was just sent is what the reader is waiting for, and a
  // transcript that had to be scrolled to be read would be a log file.
  useEffect(() => {
    end.current?.scrollIntoView({ block: 'end' })
  }, [entries.length, running])

  return (
    // The whole transcript is selectable: what is in it is statements the reader wrote and answers
    // the server gave, and both are things they will want out of here.
    <div className="scroll-thin selectable min-h-0 flex-1 overflow-auto px-2 py-1.5 font-mono text-xs">
      {entries.length === 0 ? (
        <p className="px-1 py-0.5 text-faint">
          Type a statement below and press Enter. Shift+Enter starts a new line.
        </p>
      ) : null}

      {entries.map((entry, at) => (
        <div key={at} className="py-0.5">
          <div className="flex gap-1.5">
            <ChevronRight size={12} className="mt-0.5 shrink-0 text-accent" />
            <span className="min-w-0 whitespace-pre-wrap text-fg">{entry.sql}</span>
          </div>

          {entry.error ? (
            <Line tone="danger">{entry.error.message}</Line>
          ) : (
            entry.statements.map((statement, index) => (
              <Line key={index} tone={statement.error === '' ? 'muted' : 'danger'}>
                {outcomeLine(statement)}
              </Line>
            ))
          )}
        </div>
      ))}

      {running ? (
        <div className="flex items-center gap-1.5 py-0.5 pl-[18px] text-muted">
          <Loader2 size={12} className="animate-spin text-accent" />
          Running…
        </div>
      ) : null}

      <div ref={end} />
    </div>
  )
}

function Line({ tone, children }: { tone: 'muted' | 'danger'; children: string }) {
  return (
    <div className={cn('pl-[18px] whitespace-pre-wrap', tone === 'danger' ? 'text-danger' : 'text-muted')}>
      {children}
    </div>
  )
}

/**
 * What one statement did, in the words a console uses: what it *returned* where it returned rows,
 * what it *changed* where it changed any, and how long either took.
 */
function outcomeLine(statement: StatementOutcome): string {
  if (statement.error !== '') return `${statement.errorCode}: ${statement.error}`

  const what =
    statement.rows > 0
      ? `${statement.rows} ${statement.rows === 1 ? 'row' : 'rows'}`
      : statement.rowsAffected > 0
        ? `${statement.rowsAffected} affected`
        : 'OK'

  return `${what} · ${statement.elapsedMs} ms`
}
