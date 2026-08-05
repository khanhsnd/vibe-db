import { engineWords, type EngineWords } from '@/bridge/connections'
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
  /** A copy under a name the reader gives, with the rows or with the shape alone. */
  duplicate: (withRows: boolean) => void
  empty: () => void
  truncate: () => void
  remove: () => void
}

/**
 * What can be done to a table, wherever a table is offered — the tree's own menu and the toolbar
 * over the table list both build from here, so the wording of a confirmation and the order of a
 * change cannot differ between the two. Nothing runs without being confirmed or filled in first,
 * and whatever asked is where a rejection is shown.
 *
 * `words` is what this engine calls a table and its rows. Only the *wording* changes with it — every
 * one of these calls the same bound method whatever the server is — but the wording is worth getting
 * right: a warning about a foreign key is a warning about something a document store cannot have.
 */
export function tableActions(
  target: TableTarget,
  deps: TableDeps,
  words: EngineWords = engineWords(''),
): TableActions {
  const { profileId, namespace, table } = target
  const lower = words.table.toLowerCase()

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
        title: `Rename ${words.table}`,
        label: 'New name',
        value: table,
        submitLabel: 'Rename',
        onSubmit: (name) =>
          run((sessionId) => schema.renameTable(sessionId, namespace, table, name), name)(),
      }),

    duplicate: (withRows) =>
      deps.ask({
        kind: 'prompt',
        title: `Duplicate ${words.table}`,
        label: 'New name',
        value: `${table}_copy`,
        submitLabel: 'Duplicate',
        onSubmit: (name) =>
          run(
            (sessionId) => schema.duplicateTable(sessionId, namespace, table, name, withRows),
            name,
          )(),
      }),

    empty: () =>
      deps.ask({
        kind: 'confirm',
        title: `Empty ${words.table}`,
        message:
          `Delete every ${words.row} of ${table}? The ${lower} and its indexes stay; ` +
          `the ${words.rows} do not.`,
        submitLabel: 'Empty',
        danger: true,
        onSubmit: run((sessionId) => schema.emptyTable(sessionId, namespace, table)),
      }),

    truncate: () =>
      deps.ask({
        kind: 'confirm',
        title: `Truncate ${words.table}`,
        message:
          `Truncate ${table}? Every ${words.row} goes at once, which is faster than emptying it ` +
          `and cannot be rolled back per ${words.row} — and a table another table points at ` +
          `will refuse.`,
        submitLabel: 'Truncate',
        danger: true,
        onSubmit: run((sessionId) => schema.truncateTable(sessionId, namespace, table)),
      }),

    remove: () =>
      deps.ask({
        kind: 'confirm',
        title: `Delete ${words.table}`,
        message:
          `Drop ${table}? The ${lower}, its ${words.rows} and everything indexing them ` +
          `go with it.`,
        submitLabel: 'Delete',
        danger: true,
        onSubmit: run((sessionId) => schema.dropTable(sessionId, namespace, table), ''),
      }),
  }
}
