import type { Namespace } from '@/bridge/connections'
import { schema } from '@/bridge/schema'
import type { TableColumn } from '@/bridge/tables'
import { tableKey, type ChangeSchema, type SchemaTarget } from '@/stores/connections'
import type { SchemaRequest } from '@/stores/schema'

/** Which table's fields these actions act on. */
export interface FieldTarget {
  profileId: string
  namespace: Namespace
  table: string
}

export interface FieldDeps {
  ask: (request: SchemaRequest) => void
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
  refreshStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  /** Opens the designer, where a field's shape is changed. */
  design: (target: FieldTarget) => void
  /** Asks that designer to open with a blank field ready. */
  askAddField: (tableKey: string) => void
}

export interface FieldActions {
  add: () => void
  edit: () => void
  rename: (column: TableColumn) => void
  remove: (column: TableColumn) => void
  refresh: () => void
}

/**
 * What can be done to a field from the tree. Anything that needs boxes filled in — adding a field,
 * restating one — opens the designer tab, which is the one surface a field's shape is edited on;
 * only the two that are a single answer, a rename and a drop, are asked for here and run at once.
 */
export function fieldActions(target: FieldTarget, deps: FieldDeps): FieldActions {
  const { profileId, namespace, table } = target

  const run = (change: ChangeSchema) => () =>
    deps.changeSchema(profileId, { namespace, table }, change)

  return {
    add: () => {
      deps.design(target)
      deps.askAddField(tableKey(profileId, namespace, table))
    },
    edit: () => deps.design(target),

    rename: (column) =>
      deps.ask({
        kind: 'prompt',
        title: 'Rename Field',
        label: 'New name',
        value: column.name,
        submitLabel: 'Rename',
        onSubmit: (name) =>
          run((sessionId) =>
            schema.renameField(sessionId, namespace, table, column.name, name),
          )(),
      }),

    remove: (column) =>
      deps.ask({
        kind: 'confirm',
        title: 'Delete Field',
        message: `Drop ${column.name} from ${table}? Everything stored in that column goes with it.`,
        submitLabel: 'Delete',
        danger: true,
        onSubmit: run((sessionId) => schema.dropField(sessionId, namespace, table, column.name)),
      }),

    refresh: () => void deps.refreshStructure(profileId, namespace, table),
  }
}
