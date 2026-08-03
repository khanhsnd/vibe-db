import { useCallback, useEffect, useMemo, useState } from 'react'
import { KeyRound, Plus, RefreshCw, Save, Table2, Trash2 } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { schema, type TableStructure } from '@/bridge/schema'
import type { Cell } from '@/bridge/tables'
import { ResultGrid } from '@/features/results/ResultGrid'
import { ResultTabs } from '@/features/results/ResultTabs'
import {
  selectedRows,
  type CellRef,
  type GridData,
  type Selection,
} from '@/features/results/resultSet'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { cn } from '@/lib/cn'
import { tableKey, useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore } from '@/stores/tabs'

import { draftOf, newField, newTableDraft, pendingChanges, type DraftField } from './draft'
import { formatFieldType, parseFieldType } from './fieldType'

interface TableDesignerProps {
  /** The tab this designer fills, which its own Save may re-point at a table it created. */
  tabId: string
  profileId: string
  namespace: Namespace
  /** The table being designed, or empty for one that does not exist yet. */
  table: string
}

/** The editable columns of the Fields grid, in the order they are drawn. */
const NAME = 0
const TYPE = 1
const LENGTH = 2
const DECIMAL = 3
const NOT_NULL = 4
const KEY = 5

/**
 * A table's shape, staged. Every edit is local until **Save**, which works out what has to happen
 * for the server's fields to become the ones on screen and runs those in order — so a rename and a
 * retype in one sitting are one trip, and nothing is half-applied because a later box was wrong.
 *
 * The three lists are `ResultGrid`s: they are rows and columns, and that is the one component for
 * those. Editing happens in the grid's own cell editor, which is why the columns are the parts a
 * field is declared from rather than the string the server reports.
 */
export function TableDesigner({ tabId, profileId, namespace, table }: TableDesignerProps) {
  const creating = table === ''
  const structure = useConnectionsStore((s) =>
    creating ? undefined : s.structures[tableKey(profileId, namespace, table)],
  )
  const loadStructure = useConnectionsStore((s) => s.loadStructure)
  const refreshStructure = useConnectionsStore((s) => s.refreshStructure)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const designed = useTabsStore((s) => s.designed)
  const addFieldTo = useSchemaStore((s) => s.addFieldTo)
  const addedField = useSchemaStore((s) => s.addedField)

  const [name, setName] = useState(table)
  const [draft, setDraft] = useState<DraftField[]>(creating ? newTableDraft : [])
  const [selection, setSelection] = useState<Selection | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    if (!creating) void loadStructure(profileId, namespace, table)
  }, [creating, loadStructure, profileId, namespace, table])

  // The draft follows a fresh read — the one after a Save, or after Refresh — and is otherwise the
  // reader's alone. `stored` is what makes that safe: a field the server has not seen survives it.
  const stored = useMemo(() => draftOf(structure), [structure])
  useEffect(() => {
    if (!creating) setDraft(stored)
  }, [creating, stored])

  // The tree's Add Field lands here. This effect is declared *after* the one above so that a row
  // appended on the way in survives the first read arriving in the same commit.
  const key = tableKey(profileId, namespace, table)
  useEffect(() => {
    if (addFieldTo !== key || (!creating && !structure)) return
    setDraft((prev) => [...prev, newField()])
    addedField()
  }, [addFieldTo, key, creating, structure, addedField])

  const changes = useMemo(
    () => (creating ? [] : pendingChanges(draft, stored)),
    [creating, draft, stored],
  )
  const dirty = creating || changes.length > 0

  const rows = useMemo(() => selectedRows(selection), [selection])

  const editCell = useCallback((ref: CellRef, value: Cell) => {
    const text = (value ?? '').trim()
    setDraft((prev) =>
      prev.map((field, at) => (at === ref.row ? withCell(field, ref.column, text) : field)),
    )
  }, [])

  const save = async () => {
    setSaving(true)
    setError(null)
    try {
      if (creating) {
        await changeSchema(profileId, { namespace, table: name }, (sessionId) =>
          schema.createTable(sessionId, namespace, name, draft),
        )
        designed(tabId, name)
      } else {
        await changeSchema(profileId, { namespace, table }, async (sessionId) => {
          for (const change of changes) await change.run(sessionId, namespace, table)
        })
      }
      setSelection(null)
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  if (!creating && !structure) {
    return <Empty title="Reading the table's shape…" hint={qualified(namespace, table)} />
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2">
        {/* Save is the weight of the toolbar the moment there is anything to save, and the one
            button that destroys a field carries the danger tone. */}
        <Button
          variant={dirty ? 'primary' : 'subtle'}
          icon={<Save size={13} />}
          disabled={saving || !dirty}
          onClick={() => void save()}
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button
          variant="subtle"
          icon={<Plus size={13} />}
          onClick={() => setDraft((prev) => [...prev, newField()])}
        >
          Add Field
        </Button>
        <Button
          variant="danger"
          icon={<Trash2 size={13} />}
          disabled={rows.length === 0}
          onClick={() => setDraft((prev) => prev.filter((_, at) => !rows.includes(at)))}
        >
          Delete Field
        </Button>
        <Button
          variant="subtle"
          icon={<KeyRound size={13} />}
          disabled={rows.length === 0}
          onClick={() =>
            setDraft((prev) => {
              const adding = prev.some((field, at) => rows.includes(at) && !field.primaryKey)
              return prev.map((field, at) =>
                rows.includes(at) ? { ...field, primaryKey: adding } : field,
              )
            })
          }
        >
          Primary Key
        </Button>

        <div className="h-4 w-px bg-line" />

        {creating ? (
          <div className="flex items-center gap-1.5">
            <Table2 size={13} className="shrink-0 text-faint" />
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Table name"
              spellCheck={false}
              className={cn(TOOLBAR_CONTROL, 'w-48')}
            />
          </div>
        ) : (
          <Button variant="subtle" icon={<RefreshCw size={13} />} onClick={() => void reload()}>
            Refresh
          </Button>
        )}

        <div className="flex-1" />
        <span className="truncate rounded-md border border-line bg-canvas px-1.5 py-0.5 font-mono text-2xs text-faint">
          {creating ? qualified(namespace, name || 'new table') : qualified(namespace, table)}
        </span>
      </div>

      {error ? (
        <div className="shrink-0 px-2 pt-2">
          <Banner tone="danger" text={error.message} />
        </div>
      ) : null}

      <div className="min-h-0 flex-1">
        <ResultTabs
          panes={[
            {
              id: 'fields',
              label: `Fields (${draft.length})`,
              content: (
                <ResultGrid
                  result={fieldRows(draft)}
                  selection={selection}
                  onSelectionChange={setSelection}
                  onEditCell={editCell}
                  isDirty={(row, column) => isEdited(draft[row], stored, column)}
                />
              ),
            },
            {
              id: 'indexes',
              label: `Indexes (${structure?.indexes.length ?? 0})`,
              content: <ResultGrid result={indexRows(structure)} />,
            },
            {
              id: 'keys',
              label: `Foreign Keys (${structure?.foreignKeys.length ?? 0})`,
              content: <ResultGrid result={keyRows(structure)} />,
            },
            {
              id: 'changes',
              label: `Changes (${creating ? 1 : changes.length})`,
              content: <ResultGrid result={changeRows(creating, name, changes)} />,
            },
          ]}
        />
      </div>
    </div>
  )

  function reload() {
    setError(null)
    setSelection(null)
    return refreshStructure(profileId, namespace, table)
  }

  // A whole row is marked pending for a field the server has not seen; an existing one is marked
  // only where it differs, so what Save will touch is readable from the grid.
  function isEdited(field: DraftField | undefined, held: DraftField[], column: number): boolean {
    if (!field) return false
    if (field.stored === null) return true

    const source = held.find((f) => f.stored === field.stored)
    if (!source) return true

    const type = parseFieldType(field.type)
    const was = parseFieldType(source.type)

    switch (column) {
      case NAME:
        return field.name !== source.name
      case TYPE:
        return type.base !== was.base
      case LENGTH:
        return type.length !== was.length
      case DECIMAL:
        return type.scale !== was.scale
      case NOT_NULL:
        return field.nullable !== source.nullable
      case KEY:
        return field.primaryKey !== source.primaryKey
      default:
        return false
    }
  }
}

// withCell writes one edited cell back onto the field it came from. Length and decimals are parts
// of the type rather than facts of their own, which is what they are on the wire.
function withCell(field: DraftField, column: number, text: string): DraftField {
  const type = parseFieldType(field.type)
  const digits = text.replace(/\D/g, '')
  const count = digits === '' ? null : Number(digits)

  switch (column) {
    case NAME:
      return { ...field, name: text }
    case TYPE:
      return { ...field, type: formatFieldType({ ...type, base: text }) }
    case LENGTH:
      return { ...field, type: formatFieldType({ ...type, length: count }) }
    case DECIMAL:
      return { ...field, type: formatFieldType({ ...type, scale: count }) }
    case NOT_NULL:
      return { ...field, nullable: !truthy(text) }
    case KEY:
      return { ...field, primaryKey: truthy(text) }
    default:
      return { ...field, default: text === '' ? null : text }
  }
}

// What a person types into a yes/no cell. Anything else — including a cleared cell — is no.
function truthy(text: string): boolean {
  const said = text.trim().toLowerCase()
  return said === 'yes' || said === 'y' || said === 'true' || said === '1'
}

// Length and decimals read as 0 rather than blank when the type declares none, so a column of
// numbers stays a column of numbers.
function fieldRows(draft: DraftField[]): GridData {
  return {
    columns: [
      column('Name'),
      column('Type'),
      column('Length', 'int'),
      column('Decimal', 'int'),
      column('Not null'),
      column('Key'),
      column('Default'),
    ],
    rows: draft.map((field): Cell[] => {
      const type = parseFieldType(field.type)
      return [
        field.name,
        type.base,
        String(type.length ?? 0),
        String(type.scale ?? 0),
        field.nullable && !field.primaryKey ? '' : 'YES',
        field.primaryKey ? 'YES' : '',
        field.default ?? '',
      ]
    }),
  }
}

function indexRows(structure: TableStructure | undefined): GridData {
  return {
    columns: [column('Name'), column('Fields'), column('Unique'), column('Method')],
    rows: (structure?.indexes ?? []).map((index): Cell[] => [
      index.name,
      index.columns.join(', '),
      index.unique ? 'YES' : '',
      index.method,
    ]),
  }
}

function keyRows(structure: TableStructure | undefined): GridData {
  return {
    columns: [
      column('Name'),
      column('Fields'),
      column('References'),
      column('On Update'),
      column('On Delete'),
    ],
    rows: (structure?.foreignKeys ?? []).map((key): Cell[] => [
      key.name,
      key.columns.join(', '),
      `${key.referencedSchema}.${key.referencedTable} (${key.referencedColumns.join(', ')})`,
      key.onUpdate,
      key.onDelete,
    ]),
  }
}

// What Save will do, in the order it will do it. Not the SQL: how a statement is spelled is the
// driver's business, and this side has never seen one.
function changeRows(
  creating: boolean,
  name: string,
  changes: { describe: string }[],
): GridData {
  const described = creating ? [`Create table ${name || '…'}`] : changes.map((c) => c.describe)

  return {
    columns: [column('#', 'int'), column('Change')],
    rows: described.map((describe, index): Cell[] => [String(index + 1), describe]),
  }
}

const column = (name: string, type = 'text') => ({ name, type, fullType: type })

const qualified = (ns: Namespace, table: string) =>
  `${ns.schema === '' ? ns.database : `${ns.database}.${ns.schema}`}.${table}`
