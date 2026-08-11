import { useEffect, useRef, useState } from 'react'
import { Plus, Spline, Trash2, TriangleAlert } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { schema, type TableStructure } from '@/bridge/schema'
import { REFERENTIAL_ACTIONS } from '@/features/schema/dataTypes'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'
import { useConnectionsStore } from '@/stores/connections'

import type { LinkDraft } from './model'

/**
 * The line the reader has just drawn, turned into a constraint the server will hold.
 *
 * **A drawn line is not a relation until this is answered.** Nothing appears on the canvas in the
 * meantime, because what the canvas draws is the catalogue — so a diagram never shows a key that is
 * not there. The statement is the driver's: this fills in a `ForeignKeySpec` and hands it to the
 * same bound method the tree's own Add Foreign Key uses, so what runs here and what runs there is
 * one thing.
 *
 * The columns arrive from the drag and are still editable, and more pairs can be added: a composite
 * key is two columns pointing at two, and a diagram is exactly where somebody notices they need one.
 */
interface RelationDialogProps {
  draft: LinkDraft | null
  profileId: string
  namespace: Namespace
  structureOf: (table: string) => TableStructure | undefined
  onClose: () => void
}

interface Pair {
  from: string
  to: string
}

export function RelationDialog({
  draft,
  profileId,
  namespace,
  structureOf,
  onClose,
}: RelationDialogProps) {
  const changeSchema = useConnectionsStore((s) => s.changeSchema)

  const [name, setName] = useState('')
  const [pairs, setPairs] = useState<Pair[]>([])
  const [onUpdate, setOnUpdate] = useState('NO ACTION')
  const [onDelete, setOnDelete] = useState('NO ACTION')
  const [error, setError] = useState<AppError | null>(null)
  const [saving, setSaving] = useState(false)

  const from = draft?.from ?? ''
  const to = draft?.to ?? ''
  const fromColumns = structureOf(from)?.columns ?? []
  const toColumns = structureOf(to)?.columns ?? []

  // Read through a ref, so the reset below turns on the **drag** alone. The catalogue is re-read
  // while the dialog is up — the diagram asks for it — and a form that started over every time a
  // structure landed would wipe what was being typed into it.
  const shapes = useRef(structureOf)
  shapes.current = structureOf

  // The dialog opens on the drag that raised it, so everything it holds is that drag's. A column
  // the drop did not name falls back to the other table's key, which is what a reader dropping on a
  // box rather than on a row meant.
  useEffect(() => {
    if (!draft) return

    const target =
      draft.toColumn !== ''
        ? draft.toColumn
        : (shapes.current(draft.to)?.columns.find((column) => column.primaryKey)?.name ?? '')

    setName(`fk_${draft.from}_${draft.fromColumn || 'ref'}`)
    setPairs([{ from: draft.fromColumn, to: target }])
    setOnUpdate('NO ACTION')
    setOnDelete('NO ACTION')
    setError(null)
    setSaving(false)
  }, [draft])

  const filled = pairs.filter((pair) => pair.from !== '' && pair.to !== '')
  const valid = name.trim() !== '' && filled.length === pairs.length && pairs.length > 0

  const submit = async () => {
    if (!draft || !valid) return

    setSaving(true)
    setError(null)
    try {
      await changeSchema(profileId, { namespace, table: draft.from }, (sessionId) =>
        schema.createForeignKey(sessionId, namespace, draft.from, {
          name: name.trim(),
          columns: filled.map((pair) => pair.from),
          // Empty is "the same namespace", which is the only place a diagram's boxes are.
          referencedSchema: '',
          referencedTable: draft.to,
          referencedColumns: filled.map((pair) => pair.to),
          onUpdate,
          onDelete,
        }),
      )
      onClose()
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={draft !== null}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title="New Foreign Key"
      description={from === '' ? '' : `${from} references ${to}`}
      icon={<Spline size={15} />}
      width={520}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={!valid || saving} onClick={() => void submit()}>
            {saving ? 'Creating…' : 'Create'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label="Constraint name:">
          {(id) => (
            <TextInput
              id={id}
              autoFocus
              value={name}
              className="font-mono"
              onChange={(event) => setName(event.target.value)}
            />
          )}
        </Field>

        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-muted">Columns:</span>

          {pairs.map((pair, at) => (
            <div key={at} className="flex items-center gap-1.5">
              <SelectInput
                aria-label={`Column of ${from}`}
                value={pair.from}
                className="font-mono"
                onChange={(event) =>
                  setPairs((held) =>
                    held.map((one, index) =>
                      index === at ? { ...one, from: event.target.value } : one,
                    ),
                  )
                }
              >
                <option value="">Pick a column</option>
                {fromColumns.map((column) => (
                  <option key={column.name} value={column.name}>
                    {column.name}
                  </option>
                ))}
              </SelectInput>

              <span className="shrink-0 text-xs text-faint">→</span>

              <SelectInput
                aria-label={`Column of ${to}`}
                value={pair.to}
                className="font-mono"
                onChange={(event) =>
                  setPairs((held) =>
                    held.map((one, index) =>
                      index === at ? { ...one, to: event.target.value } : one,
                    ),
                  )
                }
              >
                <option value="">Pick a column</option>
                {toColumns.map((column) => (
                  <option key={column.name} value={column.name}>
                    {column.name}
                  </option>
                ))}
              </SelectInput>

              <IconButton
                label="Drop this pair"
                icon={<Trash2 size={13} />}
                disabled={pairs.length === 1}
                onClick={() => setPairs((held) => held.filter((_, index) => index !== at))}
              />
            </div>
          ))}

          <div>
            <Button
              variant="subtle"
              icon={<Plus size={13} />}
              onClick={() => setPairs((held) => [...held, { from: '', to: '' }])}
            >
              Add a pair
            </Button>
          </div>
        </div>

        <div className="flex gap-3">
          <div className="flex-1">
            <Field label="On update:">
              {(id) => (
                <SelectInput
                  id={id}
                  value={onUpdate}
                  onChange={(event) => setOnUpdate(event.target.value)}
                >
                  {REFERENTIAL_ACTIONS.map((action) => (
                    <option key={action} value={action}>
                      {action}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>
          </div>
          <div className="flex-1">
            <Field label="On delete:">
              {(id) => (
                <SelectInput
                  id={id}
                  value={onDelete}
                  onChange={(event) => setOnDelete(event.target.value)}
                >
                  {REFERENTIAL_ACTIONS.map((action) => (
                    <option key={action} value={action}>
                      {action}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>
          </div>
        </div>

        <Banner
          tone="muted"
          text={`The columns of ${to} have to be a key it already has, and the types on both sides have to agree — the server is what checks that, and what it says lands here.`}
        />

        {error ? (
          <Banner
            tone="danger"
            icon={<TriangleAlert size={13} />}
            text={error.detail ? `${error.message} — ${error.detail}` : error.message}
          />
        ) : null}
      </div>
    </Dialog>
  )
}
