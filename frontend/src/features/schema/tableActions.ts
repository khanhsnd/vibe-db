import { schema } from '@/bridge/schema'
import type { ChangeSchema, SchemaTarget } from '@/stores/connections'
import type { SchemaRequest } from '@/stores/schema'
import type { TableTarget } from '@/stores/tabs'

export interface TableDeps {
  ask: (request: SchemaRequest) => void
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
  /** Opens the table's rows, and its shape in the designer — both are tabs. */
  openRows: (target: TableTarget) => void
  design: (target: TableTarget) => void
}

export interface TableActions {
  open: () => void
  design: () => void
  /** The designer with no table behind it yet, which is what New Table is. */
  create: () => void
  rename: () => void
  empty: () => void
  truncate: () => void
  remove: () => void
}

/**
 * What can be done to a table, wherever a table is offered — the tree's own menu and the toolbar
 * over the table list both build from here, so the wording of a confirmation and the order of a
 * change cannot differ between the two. Nothing runs without being confirmed or filled in first,
 * and whatever asked is where a rejection is shown.
 */
export function tableActions(target: TableTarget, deps: TableDeps): TableActions {
  const { profileId, namespace, table } = target

  // Every change is the same shape: run one statement, then re-read what it made stale. The
  // table named is the one whose shape is worth re-reading afterwards — a drop names none.
  const run = (change: ChangeSchema, named = table) => () =>
    deps.changeSchema(profileId, { namespace, ...(named ? { table: named } : {}) }, change)

  return {
    open: () => deps.openRows(target),
    design: () => deps.design(target),
    create: () => deps.design({ profileId, namespace, table: '' }),

    rename: () =>
      deps.ask({
        kind: 'prompt',
        title: 'Rename Table',
        label: 'New name',
        value: table,
        submitLabel: 'Rename',
        onSubmit: (name) =>
          run((sessionId) => schema.renameTable(sessionId, namespace, table, name), name)(),
      }),

    empty: () =>
      deps.ask({
        kind: 'confirm',
        title: 'Empty Table',
        message: `Delete every row of ${table}? The table and its shape stay; the rows do not.`,
        submitLabel: 'Empty',
        danger: true,
        onSubmit: run((sessionId) => schema.emptyTable(sessionId, namespace, table)),
      }),

    truncate: () =>
      deps.ask({
        kind: 'confirm',
        title: 'Truncate Table',
        message:
          `Truncate ${table}? Every row goes at once, which is faster than emptying it and ` +
          `cannot be rolled back per row — and a table another table points at will refuse.`,
        submitLabel: 'Truncate',
        danger: true,
        onSubmit: run((sessionId) => schema.truncateTable(sessionId, namespace, table)),
      }),

    remove: () =>
      deps.ask({
        kind: 'confirm',
        title: 'Delete Table',
        message: `Drop ${table}? The table, its rows and everything indexing them go with it.`,
        submitLabel: 'Delete',
        danger: true,
        onSubmit: run((sessionId) => schema.dropTable(sessionId, namespace, table), ''),
      }),
  }
}
