import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Lock, Plus } from 'lucide-react'

import { objects, type ObjectFacts, type ObjectRule } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { TextInput } from '@/shared/ui/Field'
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
 *  - only the six the service takes are editable, and everything else is drawn with a lock: the tag,
 *    the length, the last-modified, the request id are the *service's* and there is no request that
 *    writes them. Offering to edit those would be offering something that does not exist;
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
  /** What the reader has typed, staged. `null` is a row they deleted. */
  const [draft, setDraft] = useState<Record<string, string | null>>({})
  /**
   * Two different things, and they are two pieces of state because they are two questions: which row
   * the reader has **picked** — what Edit and Delete act on — and which row's editor is **open**. A
   * click on a value opens its editor directly, because making a reader press twice to change a value
   * they are already pointing at buys nothing; a click on a name picks without opening, which is how
   * Delete is aimed.
   */
  const [picked, setPicked] = useState('')
  const [editing, setEditing] = useState('')
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

  const discard = () => {
    setDraft({})
    setEditing('')
    setFailure(null)
  }

  const apply = () => {
    if (!facts) return

    const input = { system: {} as Record<string, string>, meta: {} as Record<string, string> }
    for (const row of rows) {
      if (!row.editable || row.value === null) continue
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
        setEditing('')
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
        stage(clean, '')
        setEditing(clean)
      },
    })
  }

  if (facts === null) {
    return failure !== null ? <ErrorStrip error={failure} /> : <Loading label="Reading the object…" />
  }

  const editable = rows.filter((row) => row.editable)
  const owned = rows.filter((row) => !row.editable)
  const focused = rows.find((row) => row.name === picked && row.editable)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="min-h-0 flex-1 overflow-auto">
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
          <Table
            rows={editable}
            editing={editing}
            picked={picked}
            onPick={setPicked}
            onEdit={(name) => {
              setPicked(name)
              setEditing(name)
            }}
            onChange={stage}
            onDelete={(name) => stage(name, null)}
            readOnly={rule.refused}
          />
        )}

        <Heading>
          The service&rsquo;s own
          <span className="pl-1.5 font-normal text-faint">
            no request writes these
          </span>
        </Heading>
        <Table
          rows={owned}
          editing=""
          picked={picked}
          onPick={setPicked}
          onEdit={() => {}}
          onChange={() => {}}
          readOnly
        />
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
          <Button
            variant="subtle"
            icon={ICON.rename}
            disabled={focused === undefined}
            onClick={() => setEditing(picked)}
          >
            Edit
          </Button>
          <Button
            variant="subtle"
            icon={ICON.delete}
            disabled={focused === undefined}
            className="text-danger hover:bg-danger-subtle hover:text-danger"
            onClick={() => {
              if (focused) stage(focused.name, null)
            }}
          >
            Delete
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
  /** `null` is a row the reader has deleted, drawn struck through until Apply. */
  value: string | null
  editable: boolean
  /** Whether it is an `x-amz-meta-` pair rather than one of the six. */
  meta: boolean
  /** Whether the reader has changed it. */
  dirty: boolean
}

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

  const push = (name: string, value: string | null, editable: boolean, meta: boolean) => {
    if (seen.has(name.toLowerCase())) return
    seen.add(name.toLowerCase())

    const held = Object.hasOwn(draft, name) ? draft[name]! : value
    rows.push({ name, value: held, editable, meta, dirty: Object.hasOwn(draft, name) })
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

function Table({
  rows,
  editing,
  picked,
  onPick,
  onEdit,
  onChange,
  onDelete,
  readOnly,
}: {
  rows: readonly HeaderRow[]
  editing: string
  picked: string
  onPick: (name: string) => void
  onEdit: (name: string) => void
  onChange: (name: string, value: string | null) => void
  onDelete?: (name: string) => void
  readOnly: boolean
}) {
  if (rows.length === 0) return null

  return (
    <dl className="grid grid-cols-[minmax(9rem,auto)_1fr_auto] gap-x-2">
      {rows.map((row) => (
        <div
          key={row.name}
          onClick={() => onPick(row.name)}
          className={cn(
            'col-span-3 grid grid-cols-subgrid items-center border-b border-line/60',
            // A staged row wins over a picked one: what the reader has *changed* is the more
            // consequential fact, and the two shades are never both wanted at once.
            row.dirty ? 'bg-accent-subtle' : picked === row.name && 'bg-selected',
          )}
        >
          <dt className="flex items-center gap-1 px-2 py-1 text-xs text-muted">
            {!row.editable ? <Lock size={10} className="shrink-0 text-faint" /> : null}
            <span className="truncate" title={row.name}>
              {row.meta ? `meta: ${row.name}` : row.name}
            </span>
          </dt>

          <dd className="min-w-0 px-1 py-0.5">
            {row.editable && !readOnly && editing === row.name ? (
              <TextInput
                autoFocus
                value={row.value ?? ''}
                className="h-6 text-xs"
                onChange={(e) => onChange(row.name, e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === 'Escape') onEdit('')
                }}
                onBlur={() => onEdit('')}
              />
            ) : (
              <button
                type="button"
                disabled={!row.editable || readOnly}
                onClick={() => onEdit(row.name)}
                className={cn(
                  'selectable block w-full truncate px-1 py-0.5 text-left font-mono text-2xs',
                  row.value === null ? 'text-danger line-through' : 'text-fg',
                  row.editable && !readOnly ? 'rounded hover:bg-hover' : 'cursor-default',
                )}
                title={row.value ?? 'This row will be dropped when Apply is pressed'}
              >
                {row.value === null ? '(dropped)' : row.value || '—'}
              </button>
            )}
          </dd>

          <dd className="px-1">
            {row.editable && !readOnly && onDelete ? (
              <IconButton
                label={`Drop ${row.name}`}
                icon={ICON.delete}
                onClick={() => onDelete(row.name)}
              />
            ) : null}
          </dd>
        </div>
      ))}
    </dl>
  )
}

function Heading({ children }: { children: ReactNode }) {
  return (
    <h3 className="sticky top-0 border-b border-line bg-bar px-2 py-1 text-2xs font-medium text-muted">
      {children}
    </h3>
  )
}
