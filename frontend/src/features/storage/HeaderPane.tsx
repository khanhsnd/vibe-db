import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Plus, Undo2 } from 'lucide-react'

import { objects, type ObjectFacts, type ObjectRule } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import { GridBlock } from '@/features/results/GridBlock'
import type { Cell, ResultColumn } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { IconButton } from '@/shared/ui/IconButton'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { useSchemaStore } from '@/stores/schema'

/**
 * An object's headers, and the six of them a service will let a reader **change**.
 *
 * The whole shape of this pane comes from one fact about the protocol: **there is no request that
 * sets a header.** What there is, is a copy of the object onto itself with the new set — so
 *
 *  - only the six the service takes are editable, and everything else is under a second heading and
 *    read-only: the tag, the length, the last-modified, the request id are the *service's* and there
 *    is no request that writes them. Offering to edit those would be offering something that does
 *    not exist;
 *  - a change is **staged and applied in one press**, because the request replaces the set entire. A
 *    row deleted here is a header the object stops having, and a per-row write would mean one rewrite
 *    of the object per keystroke;
 *  - Apply **rewrites the object** — a new tag, a new modified time, a new version on a bucket that
 *    keeps them — which is why it is a `write` on `domain.ObjectHazard` and why the confirmation says
 *    so out loud.
 */
export function HeaderPane({
  sessionId,
  bucket,
  objectKey,
  rule,
}: {
  sessionId: string
  bucket: string
  objectKey: string
  rule: ObjectRule
}) {
  const [facts, setFacts] = useState<ObjectFacts | null>(null)
  const [failure, setFailure] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)
  /** What the reader has typed, staged. `null` is a row they dropped. */
  const [draft, setDraft] = useState<Record<string, string | null>>({})
  const ask = useSchemaStore((s) => s.ask)

  const load = useCallback(() => objects.head(sessionId, bucket, objectKey), [sessionId, bucket, objectKey])

  // The object is the identity of this pane — `ObjectPanel` gives it a `key` that changes with it —
  // so this runs once for each rather than on every redraw.
  useEffect(() => {
    let live = true
    void load()
      .then((answer) => {
        if (live) setFacts(answer)
      })
      .catch((thrown: unknown) => {
        if (live) setFailure(toAppError(thrown))
      })
    return () => {
      live = false
    }
  }, [load])

  const rows = useMemo(() => (facts ? headerRows(facts, draft) : []), [facts, draft])
  const staged = Object.keys(draft).length > 0

  const stage = (name: string, value: string | null) =>
    setDraft((held) => ({ ...held, [name]: value }))

  /** Takes one row's change back off the pile, which is how a row dropped by mistake is kept. */
  const unstage = (name: string) =>
    setDraft((held) => Object.fromEntries(Object.entries(held).filter(([key]) => key !== name)))

  const discard = () => {
    setDraft({})
    setFailure(null)
  }

  const apply = () => {
    if (!facts) return

    const input = { system: {} as Record<string, string>, meta: {} as Record<string, string> }
    for (const row of rows) {
      if (!row.editable || row.dropped) continue
      if (row.meta) input.meta[row.name] = row.value
      else input.system[row.name] = row.value
    }

    const run = async () => {
      setBusy(true)
      setFailure(null)
      try {
        // What comes back is the object **read again**, because the write rewrote it: the tag and
        // the modified time have both changed, and showing what was typed would hide that.
        setFacts(await objects.setHeaders(sessionId, bucket, objectKey, input))
        setDraft({})
      } catch (thrown) {
        setFailure(toAppError(thrown))
      } finally {
        setBusy(false)
      }
    }

    if (!rule.confirm) {
      void run()
      return
    }

    ask({
      kind: 'confirm',
      title: rule.verb,
      message:
        'There is no request that sets a header, so this copies the object onto itself with the new ' +
        'set. It gets a new ETag and a new modified time, a versioned bucket keeps a new version, and ' +
        'any header not listed here is dropped.',
      submitLabel: rule.verb,
      danger: false,
      onSubmit: run,
    })
  }

  const add = () => {
    ask({
      kind: 'prompt',
      title: 'Add Header',
      label: 'Name — one of the six, or anything for an x-amz-meta- one',
      value: '',
      submitLabel: 'Add',
      onSubmit: async (name) => {
        const clean = name.trim()
        if (clean === '') return
        // The row appears empty at the end of the changeable table; a click on its value opens the
        // box, which is what a click on any other cell of any other grid does.
        stage(clean, '')
      },
    })
  }

  if (facts === null) {
    return failure !== null ? <ErrorStrip error={failure} /> : <Loading label="Reading the object…" />
  }

  const editable = rows.filter((row) => row.editable)
  const owned = rows.filter((row) => !row.editable)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="scroll-thin flex min-h-0 flex-1 flex-col overflow-auto">
        <Heading>
          Changeable
          <span className="pl-1.5 font-normal text-faint">
            the six the protocol takes, and any x-amz-meta- pair
          </span>
        </Heading>
        {editable.length === 0 ? (
          <p className="px-2 py-2 text-xs text-muted">
            None set. Add one — the service takes {facts.settable.join(', ')}, and anything else
            becomes an <code>x-amz-meta-</code> pair.
          </p>
        ) : (
          <GridBlock
            result={{ columns: HEADER_COLUMNS, rows: gridRows(editable) }}
            // A cell of the Value column, and nothing else: the name is the catalogue's here, a row
            // being named when it is added rather than typed over afterwards.
            onEditCell={
              rule.refused
                ? undefined
                : ({ row, column }, value) => {
                    const one = editable[row]
                    if (one && column === VALUE) stage(one.name, value ?? '')
                  }
            }
            isDirty={(row) => editable[row]?.dirty ?? false}
            // A dropped row is struck through and banded by the grid itself, which is the same thing
            // it draws for a row of a table waiting to be deleted — and it is waiting for the same
            // reason: nothing has been sent yet.
            isDeleted={(row) => editable[row]?.dropped ?? false}
            rowActions={
              rule.refused
                ? undefined
                : {
                    label: '',
                    width: 34,
                    render: (row) => {
                      const one = editable[row]
                      if (!one) return null
                      return one.dropped ? (
                        <IconButton
                          label={`Keep ${one.name}`}
                          icon={<Undo2 size={12} />}
                          onClick={() => unstage(one.name)}
                        />
                      ) : (
                        <IconButton
                          label={`Drop ${one.name}`}
                          icon={ICON.delete}
                          onClick={() => stage(one.name, null)}
                        />
                      )
                    },
                  }
            }
          />
        )}

        <Heading>
          The service&rsquo;s own
          <span className="pl-1.5 font-normal text-faint">
            no request writes these
          </span>
        </Heading>
        {/* No `onEditCell`, which is the whole of what makes a grid read-only: there is no request
            that writes any of these, so there is nothing to open a box over. */}
        <GridBlock result={{ columns: HEADER_COLUMNS, rows: gridRows(owned) }} />
      </div>

      {failure !== null ? <ErrorStrip error={failure} /> : null}

      {/* The bar of actions, and it is under the rows rather than over them for the reason the grid's
          own is: what a row can be done to belongs beside the rows, and what is *counted* belongs on
          the right. It is left out entirely on a read-only connection — a row of dead buttons says
          only that they are dead. */}
      {!rule.refused ? (
        <div
          role="toolbar"
          className="flex h-8 shrink-0 items-center gap-1 border-t border-line bg-bar px-1.5"
        >
          <Button variant="subtle" icon={<Plus size={13} />} onClick={add}>
            Add
          </Button>

          <span className="flex-1" />

          {staged ? (
            <span className="shrink-0 font-mono text-2xs text-warn">
              {Object.keys(draft).length} staged
            </span>
          ) : null}
          <Button variant="subtle" disabled={!staged || busy} onClick={discard}>
            Discard
          </Button>
          <Button variant="primary" disabled={!staged || busy} onClick={apply}>
            Apply
          </Button>
        </div>
      ) : null}
    </div>
  )
}

/** One row of the pane: what it is called, what it says, and whether anything can write it. */
interface HeaderRow {
  name: string
  /**
   * What the row says, and it is the value the object **has** even on a row the reader has dropped:
   * a row on its way out is drawn struck through with what it is about to lose, which is the one
   * thing a reader has to see before pressing Apply.
   */
  value: string
  /** Whether the reader has dropped it — a header the object stops having when Apply is pressed. */
  dropped: boolean
  editable: boolean
  /** Whether it is an `x-amz-meta-` pair rather than one of the six. */
  meta: boolean
  /** Whether the reader has changed it. */
  dirty: boolean
}

/** The one editable column. The name is what the row *is*; a value is what it says. */
const VALUE = 1

const HEADER_COLUMNS: ResultColumn[] = [
  { name: 'Name', type: 'text', fullType: 'text', width: 190, readOnly: true },
  { name: 'Value', type: 'text', fullType: 'text', width: 420 },
]

/** An `x-amz-meta-` pair is named as one, so two rows cannot read as the same header. */
const gridRows = (rows: readonly HeaderRow[]): Cell[][] =>
  rows.map((row) => [row.meta ? `meta: ${row.name}` : row.name, row.value])

/**
 * The rows the pane draws, which is the service's answer with what the reader typed over the top.
 *
 * The **settable six are always drawn**, even when the object has none of them: a reader looking for
 * `Cache-Control` should find an empty row to type in rather than have to know it can be added.
 */
function headerRows(facts: ObjectFacts, draft: Record<string, string | null>): HeaderRow[] {
  const settable = new Set(facts.settable.map((one) => one.toLowerCase()))
  const rows: HeaderRow[] = []
  const seen = new Set<string>()

  const push = (name: string, value: string, editable: boolean, meta: boolean) => {
    if (seen.has(name.toLowerCase())) return
    seen.add(name.toLowerCase())

    const typed = Object.hasOwn(draft, name) ? draft[name]! : undefined
    rows.push({
      name,
      // A dropped row keeps the value it had — `dropped` is what says it is going, and the grid
      // strikes it through.
      value: typed ?? value,
      dropped: Object.hasOwn(draft, name) && draft[name] === null,
      editable,
      meta,
      dirty: Object.hasOwn(draft, name),
    })
  }

  for (const name of facts.settable) {
    const held = facts.headers.find((one) => one.name.toLowerCase() === name.toLowerCase())
    push(name, held?.value ?? '', true, false)
  }

  for (const pair of facts.metadata) {
    push(pair.name, pair.value, true, true)
  }
  // Anything the reader added that is not one of the six is an `x-amz-meta-` pair.
  for (const name of Object.keys(draft)) {
    push(name, draft[name] ?? '', true, !settable.has(name.toLowerCase()))
  }
  for (const header of facts.headers) {
    // The uploader's own reach the section above with their prefix off, so the raw form is left out
    // of this one rather than drawn twice under two names.
    if (header.name.toLowerCase().startsWith('x-amz-meta-')) continue
    push(header.name, header.value, false, false)
  }

  return rows
}

function Heading({ children }: { children: ReactNode }) {
  return (
    <h3 className="sticky top-0 border-b border-line bg-bar px-2 py-1 text-2xs font-medium text-muted">
      {children}
    </h3>
  )
}
