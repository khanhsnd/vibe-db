import { useCallback } from 'react'

import {
  CONNECTION_COLOURS,
  engineInfo,
  engineWords,
  type EngineWords,
  type Namespace,
  type Profile,
} from '@/bridge/connections'
import { databases } from '@/bridge/databases'
import { ICON } from '@/shared/ui/menuIcons'
import { schema } from '@/bridge/schema'
import type { TableColumn } from '@/bridge/tables'
import { fieldActions, type FieldActions } from '@/features/schema/fieldActions'
import { tableActions, type TableActions } from '@/features/schema/tableActions'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { tableKey, useConnectionsStore, type ChangeSchema } from '@/stores/connections'
import { useSchemaStore, type SchemaRequest } from '@/stores/schema'
import type { TableTarget } from '@/stores/tabs'

import { colourSwatch } from './ConnectionColour'
import { ownsObjects, type TreeNode } from './tree'

interface TreeMenuActions {
  onConnect: (profile: Profile) => void
  onEditConnection: (profile: Profile) => void
  /** Opens a console on this connection, at whichever namespace it is pointed at. */
  onConsole: (profile: Profile) => void
  /** Raises the dialog that runs a file of statements against this connection. */
  onExecuteFile: (profile: Profile) => void
  /** Opens a data tab on whatever has rows: a table, or a view, which reads the same way. */
  onOpenRows: (target: TableTarget) => void
  /** Opens the designer on a table, or on a new one when the table is empty. */
  onDesign: (target: TableTarget) => void
}

/**
 * Every right-click in the object tree, in one place. A menu item either reads — Refresh — opens
 * the designer tab, where a shape is edited with room to work, or raises the one dialog that owns
 * the statement. Nothing here changes a schema without asking first, and nothing changes one
 * silently, because whatever asked is also where a rejection is shown.
 */
export function useTreeMenu(actions: TreeMenuActions): (node: TreeNode) => MenuItem[] {
  const profiles = useConnectionsStore((s) => s.profiles)
  const sessions = useConnectionsStore((s) => s.sessions)
  const structures = useConnectionsStore((s) => s.structures)
  const disconnect = useConnectionsStore((s) => s.disconnect)
  const deleteProfile = useConnectionsStore((s) => s.deleteProfile)
  const refreshObjects = useConnectionsStore((s) => s.refreshObjects)
  const refreshSchemas = useConnectionsStore((s) => s.refreshSchemas)
  const refreshStructure = useConnectionsStore((s) => s.refreshStructure)
  const refreshDatabases = useConnectionsStore((s) => s.refreshDatabases)
  const setColour = useConnectionsStore((s) => s.setColour)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const changeDatabases = useConnectionsStore((s) => s.changeDatabases)
  const dump = useConnectionsStore((s) => s.dump)
  const ask = useSchemaStore((s) => s.ask)
  const askAddField = useSchemaStore((s) => s.askAddField)

  const { onConnect, onEditConnection, onConsole, onExecuteFile, onOpenRows, onDesign } = actions

  return useCallback(
    (node: TreeNode): MenuItem[] => {
      const profile = profiles.find((p) => p.id === node.profileId)
      if (!profile) return []

      const info = engineInfo(profile.engine)
      const words = engineWords(profile.engine)

      const newDatabase: MenuItem = {
        label: 'New Database…',
        icon: ICON.newDatabase,
        onSelect: () => ask({ kind: 'database', profileId: node.profileId }),
      }

      if (node.kind === 'connection') {
        return connectionItems(profile, words, Boolean(sessions[node.profileId]), {
          newDatabase,
          onConnect,
          onEditConnection,
          onConsole,
          onExecuteFile,
          disconnect,
          deleteProfile,
          setColour,
          refreshDatabases,
        })
      }

      const ns = node.namespace
      if (!ns || !sessions[node.profileId]) return []

      // Every schema change is the same shape: run one statement, then re-read what it made
      // stale. Only the statement and what survives it differ.
      const change = (run: ChangeSchema, table?: string) => () =>
        changeSchema(node.profileId, { namespace: ns, ...(table ? { table } : {}) }, run)

      // Every table action is the same object wherever it is offered, the toolbar over the table
      // list included; only which of them a node is worth offering differs.
      const on = tableActions(
        { profileId: node.profileId, namespace: ns, table: node.label },
        { ask, changeSchema, openRows: onOpenRows, design: onDesign },
        words,
      )

      // A new table is designed in a tab like any other, with an empty name: the designer's Save
      // is what creates it, so declaring one and altering one are the same surface.
      const newTable: MenuItem = {
        label: `New ${words.table}`,
        icon: ICON.add,
        onSelect: on.create,
      }

      // Dumping is offered wherever a dump has a subject: one table, or every table a namespace
      // holds — `''`, which is what the Go side reads as all of them. It is the same two-way choice
      // Duplicate is, so it reads as the same submenu, and what the file is *called* is the
      // engine's own word for a file of its statements: a document store's are not SQL.
      //
      // Nothing is confirmed first, because nothing is changed: the save dialog is the question.
      const dumpItems = (table: string): MenuItem => ({
        label: `Dump ${words.scriptFile}`,
        icon: ICON.dump,
        items: [
          {
            label: 'Structure Only…',
            onSelect: () => void dump(node.profileId, { namespace: ns, table, withData: false }),
          },
          {
            label: 'Structure and Data…',
            onSelect: () => void dump(node.profileId, { namespace: ns, table, withData: true }),
          },
        ],
      })

      // One read fills all three folders, so Refresh means the same thing wherever it is asked
      // for under a namespace.
      const refreshObjectList: MenuItem = {
        label: 'Refresh',
        icon: ICON.refresh,
        separatorBefore: true,
        onSelect: () => void refreshObjects(node.profileId, ns),
      }

      switch (node.kind) {
        case 'database':
        case 'schema': {
          const own = ownsObjects(node)
            ? [newTable, dumpItems(''), refreshObjectList]
            : [
                {
                  label: 'Refresh',
                  icon: ICON.refresh,
                  separatorBefore: true,
                  onSelect: () => void refreshSchemas(node.profileId, ns.database),
                },
              ]

          // Only the database level can make or drop a database; a schema sits inside one.
          if (node.kind !== 'database') return own
          return [
            newDatabase,
            {
              label: 'Delete Database',
              icon: ICON.delete,
              danger: true,
              onSelect: () =>
                ask({
                  kind: 'confirm',
                  title: 'Delete Database',
                  message:
                    `Drop ${ns.database}? Every ${words.table.toLowerCase()} in it, and every ` +
                    `${words.row} of every one of them, goes with it.`,
                  submitLabel: 'Delete',
                  danger: true,
                  onSubmit: () =>
                    changeDatabases(node.profileId, (sessionId) =>
                      databases.drop(sessionId, ns.database),
                    ),
                }),
            },
            ...own.map((item, at) => (at === 0 ? { ...item, separatorBefore: true } : item)),
          ]
        }

        case 'tables':
          return [newTable, dumpItems(''), refreshObjectList]

        case 'views':
        case 'routines':
          return [{ ...refreshObjectList, separatorBefore: false }]

        case 'table':
          return tableItems(
            on,
            () => void refreshStructure(node.profileId, ns, node.label),
            dumpItems(node.label),
            words,
            info.hasTruncate,
          )

        case 'view':
          return [
            {
              label: 'Open View',
              icon: ICON.open,
              onSelect: () =>
                onOpenRows({ profileId: node.profileId, namespace: ns, table: node.label }),
            },
            {
              label: 'Delete View',
              icon: ICON.delete,
              danger: true,
              separatorBefore: true,
              onSelect: () =>
                ask({
                  kind: 'confirm',
                  title: 'Delete View',
                  message:
                    `Drop the view ${node.label}? The rows it read stay where they are; ` +
                    `anything selecting from it stops working.`,
                  submitLabel: 'Delete',
                  danger: true,
                  onSubmit: change((sessionId) => schema.dropView(sessionId, ns, node.label)),
                }),
            },
            refreshObjectList,
          ]

        case 'procedure':
        case 'function':
          return routineItems(node, ns, { change, ask, refresh: refreshObjectList })

        case 'fields':
        case 'field': {
          const table = node.table ?? ''
          const fields = structures[tableKey(node.profileId, ns, table)]?.columns ?? []
          const on = fieldActions(
            { profileId: node.profileId, namespace: ns, table },
            { ask, changeSchema, refreshStructure, design: onDesign, askAddField },
          )
          return fieldItems(
            node,
            fields.find((column) => column.name === node.label),
            on,
          )
        }

        case 'indexes':
        case 'index':
        case 'foreignKeys':
        case 'foreignKey':
          return constraintItems(node, ns, { change, ask, refreshStructure })

        default:
          return []
      }
    },
    [
      profiles,
      sessions,
      structures,
      disconnect,
      deleteProfile,
      refreshObjects,
      refreshSchemas,
      refreshStructure,
      refreshDatabases,
      setColour,
      changeSchema,
      changeDatabases,
      dump,
      ask,
      askAddField,
      onConnect,
      onEditConnection,
      onConsole,
      onExecuteFile,
      onOpenRows,
      onDesign,
    ],
  )
}

/**
 * A routine is dropped by the whole of what the tree holds for it, not by its name: on
 * PostgreSQL two functions may share a name and be told apart only by what they take.
 */
function routineItems(
  node: TreeNode,
  ns: Namespace,
  deps: {
    change: (run: ChangeSchema, table?: string) => () => Promise<void>
    ask: (request: SchemaRequest) => void
    refresh: MenuItem
  },
): MenuItem[] {
  const routine = node.routine
  if (!routine) return [{ ...deps.refresh, separatorBefore: false }]

  const kind = routine.procedure ? 'Procedure' : 'Function'

  return [
    {
      label: `Delete ${kind}`,
      icon: ICON.delete,
      danger: true,
      onSelect: () =>
        deps.ask({
          kind: 'confirm',
          title: `Delete ${kind}`,
          message: `Drop ${node.label}? Anything that calls it stops working.`,
          submitLabel: 'Delete',
          danger: true,
          onSubmit: deps.change((sessionId) => schema.dropRoutine(sessionId, ns, routine)),
        }),
    },
    deps.refresh,
  ]
}

/**
 * Everything a connection is offered, in three groups: what it *is* — open or shut, and the
 * database list it holds — then what can be *run* on it, then what the connection itself is.
 * Anything needing a live server is left out rather than greyed out while it is shut, because a
 * closed connection cannot be asked anything at all and a column of dead items says only that.
 */
function connectionItems(
  profile: Profile,
  words: EngineWords,
  connected: boolean,
  on: {
    newDatabase: MenuItem
    onConnect: (profile: Profile) => void
    onEditConnection: (profile: Profile) => void
    onConsole: (profile: Profile) => void
    onExecuteFile: (profile: Profile) => void
    disconnect: (profileId: string) => Promise<void>
    deleteProfile: (profileId: string) => Promise<void>
    setColour: (profileId: string, colour: string) => Promise<void>
    refreshDatabases: (profileId: string) => Promise<void>
  },
): MenuItem[] {
  const live: MenuItem[] = connected
    ? [
        {
          label: 'Refresh',
          icon: ICON.refresh,
          onSelect: () => void on.refreshDatabases(profile.id),
        },
        {
          label: 'Console',
          icon: ICON.console,
          separatorBefore: true,
          onSelect: () => on.onConsole(profile),
        },
        {
          label: `Execute ${words.scriptFile}…`,
          icon: ICON.executeFile,
          onSelect: () => on.onExecuteFile(profile),
        },
        on.newDatabase,
      ]
    : []

  return [
    connected
      ? {
          label: 'Disconnect',
          icon: ICON.disconnect,
          onSelect: () => void on.disconnect(profile.id),
        }
      : { label: 'Connect', icon: ICON.connect, onSelect: () => on.onConnect(profile) },
    ...live,
    {
      label: 'Colour',
      icon: ICON.colour,
      separatorBefore: true,
      // The palette is the submenu rather than nine items on the menu itself: a colour is picked
      // rarely and would otherwise be most of what a right-click shows.
      items: CONNECTION_COLOURS.map((info) => ({
        label: info.label,
        icon: colourSwatch(info.value, info.value === profile.colour),
        onSelect: () => void on.setColour(profile.id, info.value),
      })),
    },
    {
      label: 'Edit Connection…',
      icon: ICON.editConnection,
      onSelect: () => on.onEditConnection(profile),
    },
    {
      label: 'Delete Connection',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: () => void on.deleteProfile(profile.id),
    },
  ]
}

/**
 * The whole of what a table is offered, in the order the tree lists it.
 *
 * `dump` arrives built rather than being made here, because the same item is offered on a namespace
 * as well and differs between the two only in what it names — one table, or all of them.
 *
 * Truncate is left out where it is not a second thing: on a document store every document has to be
 * deleted to empty a collection, so Empty and Truncate would be one command under two names — see
 * `hasTruncate`.
 */
export function tableItems(
  on: TableActions,
  refresh: () => void,
  dump: MenuItem,
  words: EngineWords = engineWords(''),
  truncates = true,
): MenuItem[] {
  return [
    { label: `Open ${words.table}`, icon: ICON.open, onSelect: on.open },
    { label: `Design ${words.table}`, icon: ICON.design, onSelect: on.design },
    { label: `New ${words.table}`, icon: ICON.add, separatorBefore: true, onSelect: on.create },
    { label: `Rename ${words.table}…`, icon: ICON.rename, onSelect: on.rename },
    {
      label: `Duplicate ${words.table}`,
      icon: ICON.duplicate,
      // The copy is one action asked in two ways, so the submenu is where the reader says which
      // — and both still ask for the name, as everything that makes an object here does.
      items: [
        { label: 'Structure Only…', onSelect: () => on.duplicate(false) },
        { label: 'Structure and Data…', onSelect: () => on.duplicate(true) },
      ],
    },
    // Beside Duplicate rather than beside Delete: both copy the table, and the only difference is
    // whether the copy lands on this server or in a file.
    dump,
    { label: `Empty ${words.table}`, icon: ICON.empty, separatorBefore: true, onSelect: on.empty },
    ...(truncates
      ? [{ label: `Truncate ${words.table}`, icon: ICON.truncate, onSelect: on.truncate }]
      : []),
    { label: `Delete ${words.table}`, icon: ICON.delete, danger: true, onSelect: on.remove },
    { label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: refresh },
  ]
}

function fieldItems(node: TreeNode, stored: TableColumn | undefined, on: FieldActions): MenuItem[] {
  const addField: MenuItem = { label: 'Add Field', icon: ICON.add, onSelect: on.add }
  const refresh: MenuItem = {
    label: 'Refresh',
    icon: ICON.refresh,
    separatorBefore: true,
    onSelect: on.refresh,
  }

  if (node.kind === 'fields' || !stored) return [addField, refresh]

  return [
    { label: 'Edit Field', icon: ICON.design, onSelect: on.edit },
    { label: 'Rename Field…', icon: ICON.rename, onSelect: () => on.rename(stored) },
    addField,
    {
      label: 'Delete Field',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: () => on.remove(stored),
    },
    refresh,
  ]
}

interface ConstraintMenuDeps {
  change: (run: ChangeSchema, table?: string) => () => Promise<void>
  ask: (request: SchemaRequest) => void
  refreshStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
}

function constraintItems(node: TreeNode, ns: Namespace, deps: ConstraintMenuDeps): MenuItem[] {
  const table = node.table ?? ''
  const refresh: MenuItem = {
    label: 'Refresh',
    icon: ICON.refresh,
    onSelect: () => void deps.refreshStructure(node.profileId, ns, table),
  }

  if (node.kind === 'index') {
    return [
      {
        label: 'Delete Index',
        icon: ICON.delete,
        danger: true,
        // The index behind a primary key is the key, and neither engine will drop it as an
        // index — so the item says why rather than failing at the server.
        disabled: node.primaryKey === true,
        onSelect: () =>
          deps.ask({
            kind: 'confirm',
            title: 'Delete Index',
            message: `Drop the index ${node.label}? Reads that leaned on it will scan instead.`,
            submitLabel: 'Delete',
            danger: true,
            onSubmit: deps.change(
              (sessionId) => schema.dropIndex(sessionId, ns, table, node.label),
              table,
            ),
          }),
      },
      { ...refresh, separatorBefore: true },
    ]
  }

  if (node.kind === 'foreignKey') {
    return [
      {
        label: 'Delete Foreign Key',
        icon: ICON.delete,
        danger: true,
        onSelect: () =>
          deps.ask({
            kind: 'confirm',
            title: 'Delete Foreign Key',
            message: `Drop the constraint ${node.label}? The rows stay; nothing checks them again.`,
            submitLabel: 'Delete',
            danger: true,
            onSubmit: deps.change(
              (sessionId) => schema.dropForeignKey(sessionId, ns, table, node.label),
              table,
            ),
          }),
      },
      { ...refresh, separatorBefore: true },
    ]
  }

  return [refresh]
}
