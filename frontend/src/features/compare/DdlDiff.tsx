import { useMemo } from 'react'

import { cn } from '@/lib/cn'
import { CopyButton } from '@/shared/ui/CopyButton'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'

/**
 * The two tables' own declarations, side by side, line for line.
 *
 * It is the **server's answer** rather than a rendering of the comparison above it: MySQL is asked
 * `SHOW CREATE TABLE` and PostgreSQL is described through the same `Dialect` its designer writes
 * with, so what is read here is as close to the truth as that server can be asked for — storage
 * engine, character set, auto-increment and all, none of which a catalogue comparison can see.
 *
 * That is why it is worth having beside a diff that already names every field: the comparison says
 * what *this app* can tell apart, and this says what the two tables *are*.
 */
export function DdlDiff({
  left,
  right,
  leftLabel,
  rightLabel,
  loading,
}: {
  left: readonly string[] | null
  right: readonly string[] | null
  leftLabel: string
  rightLabel: string
  loading: boolean
}) {
  const lines = useMemo(() => alignLines(textOf(left), textOf(right)), [left, right])

  if (loading) return <Loading label="Reading both declarations…" />
  if (left === null || right === null) {
    return (
      <Empty
        title="No declaration read yet"
        hint="Both sides have to be open for a server to be asked what it would write."
      />
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-7 shrink-0 items-stretch border-b border-line bg-bar text-2xs">
        <Caption label={leftLabel} text={textOf(left)} />
        <Caption label={rightLabel} text={textOf(right)} />
      </div>

      <div className="scroll-thin min-h-0 flex-1 overflow-auto">
        {lines.map((line, at) => (
          <div key={at} className="flex items-stretch">
            <Half text={line.left} differs={line.differs} side="left" />
            <Half text={line.right} differs={line.differs} side="right" />
          </div>
        ))}
      </div>
    </div>
  )
}

function Caption({ label, text }: { label: string; text: string }) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-2 border-r border-line px-2">
      <span className="min-w-0 truncate font-mono text-fg">{label}</span>
      <div className="flex-1" />
      <CopyButton text={text} label={`Copy the declaration of ${label}`} />
    </div>
  )
}

/**
 * One side of one line. A line only one side has is drawn on the `absent` wash rather than left
 * blank, which is the same fourth colour the grids use and means the same thing: nothing is missing
 * from that line, the line is missing.
 */
function Half({
  text,
  differs,
  side,
}: {
  text: string | null
  differs: boolean
  side: 'left' | 'right'
}) {
  return (
    <pre
      className={cn(
        'min-w-0 flex-1 overflow-hidden border-r border-line px-2 font-mono text-2xs whitespace-pre',
        'selectable text-fg',
        text === null
          ? 'bg-grid-diff-absent'
          : differs && (side === 'left' ? 'bg-grid-diff-removed' : 'bg-grid-diff-added'),
      )}
    >
      {text ?? ' '}
    </pre>
  )
}

/** A declaration as one block of text, which is how a reader reads it and how a copy writes it. */
function textOf(statements: readonly string[] | null): string {
  return (statements ?? []).map((statement) => `${statement};`).join('\n\n')
}

interface DiffLine {
  left: string | null
  right: string | null
  differs: boolean
}

/**
 * The two declarations lined up so a line both sides hold sits on one row.
 *
 * It is the **same rule the field comparison follows** — match by what a thing *is* and never by
 * where it sits — applied to text: a line is matched by its own content, so a clause added in the
 * middle is drawn in the middle rather than pushing every line under it out of step. The
 * implementation is the shortest one that does that: walk both, emit what only one side holds, and
 * step together where they agree.
 */
function alignLines(left: string, right: string): DiffLine[] {
  const ours = left === '' ? [] : left.split('\n')
  const theirs = right === '' ? [] : right.split('\n')

  const held = new Map<string, number[]>()
  theirs.forEach((line, at) => {
    const trimmed = line.trim()
    held.set(trimmed, [...(held.get(trimmed) ?? []), at])
  })

  const lines: DiffLine[] = []
  let next = 0

  for (const line of ours) {
    const queue = held.get(line.trim()) ?? []
    const at = queue.find((one) => one >= next)

    if (at === undefined) {
      lines.push({ left: line, right: null, differs: true })
      continue
    }

    // Everything the other side holds before the line they agree on is new, and belongs here.
    for (; next < at; next++) {
      lines.push({ left: null, right: theirs[next] ?? '', differs: true })
    }
    lines.push({ left: line, right: theirs[at] ?? '', differs: false })
    next = at + 1
  }

  for (; next < theirs.length; next++) {
    lines.push({ left: null, right: theirs[next] ?? '', differs: true })
  }

  return lines
}
