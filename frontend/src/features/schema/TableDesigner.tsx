import { useCallback, useEffect, useMemo, useState } from 'react'
import { KeyRound, Plus, RefreshCw, Save, Table2, Trash2 } from 'lucide-react'

import { engineInfo, type Namespace } from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { schema } from '@/bridge/schema'
import type { Cell } from '@/bridge/tables'
import { ResultGrid } from '@/features/results/ResultGrid'
import { ResultTabs } from '@/features/results/ResultTabs'
import { selectedRows, type CellRef, type GridData, type Selection } from '@/features/results/resultSet'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { cn } from '@/lib/cn'
import { tableKey, useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore } from '@/stores/tabs'

import { dataTypes, indexMethods } from './dataTypes'
import {
  draftOf,
  emptyDraft,
  newField,
  newIndex,
  newKey,
  newTableConstraints,
  newTableDraft,
  pendingChanges,
  sameIndex,
  sameKey,
  type DraftField,
  type TableDraft,
} from './draft'
import {
  changeRows,
  fieldEdited,
  fieldRows,
  indexRows,
  keyRows,
  writeField,
  writeIndex,
  writeKey,
} from './designerRows'

interface TableDesignerProps {
  /** The tab this designer fills, which its own Save may re-point at a table it created. */
  tabId: string
  profileId: string
  namespace: Namespace
  /** The table being designed, or empty for one that does not exist yet. */
  table: string
}

/** Which list the toolbar's Add and Delete act on, named the way its button reads. */
type PaneId = 'fields' | 'indexes' | 'keys' | 'changes'

const NOUN: Record<PaneId, string> = {
  fields: 'Field',
  indexes: 'Index',
  keys: 'Foreign Key',
  changes: '',
}

/**
 * A table's shape, staged. Every edit is local until **Save**, which works out what has to happen
 * for the server's table to become the one on screen and runs those in order — so a rename and a
 * retype in one sitting are one trip, and nothing is half-applied because a later box was wrong.
 *
 * The three lists are `ResultGrid`s: they are rows and columns, and that is the one component for
 * those. **The toolbar acts on whichever is in front**, so `Add` means a field, an index or a
 * foreign key depending on the tab — which is why the tab strip's selection is held here rather
 * than inside it.
 */
export function TableDesigner({ tabId, profileId, namespace, table }: TableDesignerProps) {
  const creating = table === ''
  const structure = useConnectionsStore((s) =>
    creating ? undefined : s.structures[tableKey(profileId, namespace, table)],
  )
  // The engine decides which types and index methods the cells offer, and is the profile's own.
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === profileId)?.engine ?? '',
  )
  const loadStructure = useConnectionsStore((s) => s.loadStructure)
  const refreshStructure = useConnectionsStore((s) => s.refreshStructure)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const designed = useTabsStore((s) => s.designed)
  const addFieldTo = useSchemaStore((s) => s.addFieldTo)
  const addedField = useSchemaStore((s) => s.addedField)

  const [name, setName] = useState(table)
  const [draft, setDraft] = useState<TableDraft>(creating ? newTableDraft : emptyDraft)
  const [pane, setPane] = useState<PaneId>('fields')
  const [selection, setSelection] = useState<Selection | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    if (!creating) void loadStructure(profileId, namespace, table)
  }, [creating, loadStructure, profileId, namespace, table])

  // The draft follows a fresh read — the one after a Save, or after Refresh — and is otherwise the
  // reader's alone. `stored` is what makes that safe: anything the server has not seen survives it.
  const stored = useMemo(() => draftOf(structure), [structure])
  useEffect(() => {
    if (!creating) setDraft(stored)
  }, [creating, stored])

  // The tree's Add Field lands here. This effect is declared *after* the one above so that a row
  // appended on the way in survives the first read arriving in the same commit.
  const key = tableKey(profileId, namespace, table)
  useEffect(() => {
    if (addFieldTo !== key || (!creating && !structure)) return
    setDraft((prev) => ({ ...prev, fields: [...prev.fields, newField()] }))
    setPane('fields')
    addedField()
  }, [addFieldTo, key, creating, structure, addedField])

  const changes = useMemo(
    () => (creating ? newTableConstraints(draft) : pendingChanges(draft, stored)),
    [creating, draft, stored],
  )
  const dirty = creating || changes.length > 0
  const rows = useMemo(() => selectedRows(selection), [selection])

  // A cell is written back onto the list it came from, which is the one the pane in front draws.
  const editCell = useCallback(
    (ref: CellRef, value: Cell) => {
      const text = (value ?? '').trim()
      setDraft((prev) => {
        switch (pane) {
          case 'fields':
            return { ...prev, fields: mapAt(prev.fields, ref.row, (f) => writeField(f, ref.column, text)) }
          case 'indexes':
            return { ...prev, indexes: mapAt(prev.indexes, ref.row, (i) => writeIndex(i, ref.column, text)) }
          case 'keys':
            return { ...prev, keys: mapAt(prev.keys, ref.row, (k) => writeKey(k, ref.column, text)) }
          default:
            return prev
        }
      })
    },
    [pane],
  )

  const save = async () => {
    setSaving(true)
    setError(null)
    try {
      if (creating) {
        await changeSchema(profileId, { namespace, table: name }, async (sessionId) => {
          await schema.createTable(sessionId, namespace, name, draft.fields)
          // CREATE TABLE carries the fields and the key; anything else staged is made after.
          for (const change of changes) await change.run(sessionId, namespace, name)
        })
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

  const editable = pane !== 'changes'

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2">
        {/* Save is the weight of the toolbar the moment there is anything to save, and the one
            button that destroys carries the danger tone. */}
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
          disabled={!editable}
          onClick={() => setDraft((prev) => appendTo(prev, pane))}
        >
          Add {NOUN[pane] || 'Field'}
        </Button>
        <Button
          variant="danger"
          icon={<Trash2 size={13} />}
          disabled={!editable || rows.length === 0}
          onClick={() => {
            setDraft((prev) => removeFrom(prev, pane, rows))
            setSelection(null)
          }}
        >
          Delete {NOUN[pane] || 'Field'}
        </Button>
        <Button
          variant="subtle"
          icon={<KeyRound size={13} />}
          disabled={pane !== 'fields' || rows.length === 0}
          onClick={() =>
            setDraft((prev) => {
              const adding = prev.fields.some((field, at) => rows.includes(at) && !field.primaryKey)
              return {
                ...prev,
                fields: prev.fields.map((field, at) =>
                  rows.includes(at) ? { ...field, primaryKey: adding } : field,
                ),
              }
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
          selectedId={pane}
          onSelect={(id) => {
            setPane(id as PaneId)
            // A cell reference is a position in the pane that reported it, so a selection can
            // never survive the pane changing under it.
            setSelection(null)
          }}
          panes={[
            {
              id: 'fields',
              label: `Fields (${draft.fields.length})`,
              content: (
                <ResultGrid
                  result={fieldRows(draft.fields, dataTypes(engine))}
                  selection={selection}
                  onSelectionChange={setSelection}
                  onEditCell={editCell}
                  isDirty={(row, at) => isFieldEdited(draft.fields[row], stored.fields, at)}
                />
              ),
            },
            {
              id: 'indexes',
              label: `Indexes (${draft.indexes.length})`,
              content: (
                <ResultGrid
                  result={indexRows(draft.indexes, indexMethods(engine))}
                  selection={selection}
                  onSelectionChange={setSelection}
                  onEditCell={editCell}
                  isDirty={(row) => isStagedEdited(draft.indexes[row], stored.indexes, sameIndex)}
                />
              ),
            },
            // A pane for a constraint this engine has no concept of would be a pane whose Save
            // could only fail, so an engine without referential constraints does not get one.
            ...(engineInfo(engine).hasForeignKeys
              ? [
                  {
                    id: 'keys',
                    label: `Foreign Keys (${draft.keys.length})`,
                    content: (
                      <ResultGrid
                        result={keyRows(draft.keys)}
                        selection={selection}
                        onSelectionChange={setSelection}
                        onEditCell={editCell}
                        isDirty={(row) => isStagedEdited(draft.keys[row], stored.keys, sameKey)}
                      />
                    ),
                  },
                ]
              : []),
            {
              id: 'changes',
              label: `Changes (${creating ? changes.length + 1 : changes.length})`,
              content: <ResultGrid result={plannedRows(creating, name, changes)} />,
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
}

// A whole row is marked pending for a field the server has not seen; an existing one is marked
// only where it differs, so what Save will touch is readable from the grid.
function isFieldEdited(field: DraftField | undefined, held: DraftField[], at: number): boolean {
  if (!field) return false
  if (field.stored === null) return true

  const source = held.find((one) => one.stored === field.stored)
  return source ? fieldEdited(field, source, at) : true
}

// An index and a key are dropped and made again rather than altered, so there is no such thing
// as one changed cell of them: the row is pending or it is not.
function isStagedEdited<T extends { stored: string | null }>(
  staged: T | undefined,
  held: T[],
  same: (staged: T, held: T) => boolean,
): boolean {
  if (!staged) return false
  if (staged.stored === null) return true

  const source = held.find((one) => one.stored === staged.stored)
  return !source || !same(staged, source)
}

function appendTo(draft: TableDraft, pane: PaneId): TableDraft {
  switch (pane) {
    case 'indexes':
      return { ...draft, indexes: [...draft.indexes, newIndex()] }
    case 'keys':
      return { ...draft, keys: [...draft.keys, newKey()] }
    default:
      return { ...draft, fields: [...draft.fields, newField()] }
  }
}

function removeFrom(draft: TableDraft, pane: PaneId, rows: number[]): TableDraft {
  const without = <T,>(list: T[]) => list.filter((_, at) => !rows.includes(at))

  switch (pane) {
    case 'indexes':
      return { ...draft, indexes: without(draft.indexes) }
    case 'keys':
      return { ...draft, keys: without(draft.keys) }
    default:
      return { ...draft, fields: without(draft.fields) }
  }
}

function mapAt<T>(list: T[], at: number, change: (one: T) => T): T[] {
  return list.map((one, index) => (index === at ? change(one) : one))
}

function plannedRows(creating: boolean, name: string, changes: { describe: string }[]): GridData {
  const described = changes.map((change) => change.describe)
  return changeRows(creating ? [`Create table ${name || '…'}`, ...described] : described)
}

const qualified = (ns: Namespace, table: string) =>
  `${ns.schema === '' ? ns.database : `${ns.database}.${ns.schema}`}.${table}`
