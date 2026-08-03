import { useCallback } from 'react'

import type { Namespace, Profile } from '@/bridge/connections'
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

import { ownsObjects, type TreeNode } from './tree'

interface TreeMenuActions {
  onConnect: (profile: Profile) => void
  onEditConnection: (profile: Profile) => void
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
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const changeDatabases = useConnectionsStore((s) => s.changeDatabases)
  const ask = useSchemaStore((s) => s.ask)
  const askAddField = useSchemaStore((s) => s.askAddField)

  const { onConnect, onEditConnection, onOpenRows, onDesign } = actions

  return useCallback(
    (node: TreeNode): MenuItem[] => {
      const profile = profiles.find((p) => p.id === node.profileId)
      if (!profile) return []

      const newDatabase: MenuItem = {
        label: 'New Database…',
        icon: ICON.newDatabase,
        onSelect: () => ask({ kind: 'database', profileId: node.profileId }),
      }

      if (node.kind === 'connection') {
        return connectionItems(node, profile, Boolean(sessions[node.profileId]), {
          newDatabase,
          onConnect,
          onEditConnection,
          disconnect,
          deleteProfile,
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
      )

      // A new table is designed in a tab like any other, with an empty name: the designer's Save
      // is what creates it, so declaring one and altering one are the same surface.
      const newTable: MenuItem = { label: 'New Table', icon: ICON.add, onSelect: on.create }

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
            ? [newTable, refreshObjectList]
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
                    `Drop ${ns.database}? Every table in it, and every row of every table, ` +
                    `goes with it.`,
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
          return [newTable, refreshObjectList]

        case 'views':
        case 'routines':
          return [{ ...refreshObjectList, separatorBefore: false }]

        case 'table':
          return tableItems(on, () => void refreshStructure(node.profileId, ns, node.label))

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
      changeSchema,
      changeDatabases,
      ask,
      askAddField,
      onConnect,
      onEditConnection,
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

function connectionItems(
  node: TreeNode,
  profile: Profile,
  connected: boolean,
  on: {
    newDatabase: MenuItem
    onConnect: (profile: Profile) => void
    onEditConnection: (profile: Profile) => void
    disconnect: (profileId: string) => Promise<void>
    deleteProfile: (profileId: string) => Promise<void>
  },
): MenuItem[] {
  return [
    connected
      ? {
          label: 'Disconnect',
          icon: ICON.disconnect,
          onSelect: () => void on.disconnect(node.profileId),
        }
      : { label: 'Connect', icon: ICON.connect, onSelect: () => on.onConnect(profile) },
    // A database is made on a server, so the item is only there once there is a session to make
    // it on: offering it on a closed connection would be an error waiting to be raised.
    ...(connected ? [{ ...on.newDatabase, separatorBefore: true }] : []),
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
      onSelect: () => void on.deleteProfile(node.profileId),
    },
  ]
}

/** The whole of what a table is offered, in the order the tree lists it. */
export function tableItems(on: TableActions, refresh: () => void): MenuItem[] {
  return [
    { label: 'Open Table', icon: ICON.open, onSelect: on.open },
    { label: 'Design Table', icon: ICON.design, onSelect: on.design },
    { label: 'New Table', icon: ICON.add, separatorBefore: true, onSelect: on.create },
    { label: 'Rename Table…', icon: ICON.rename, onSelect: on.rename },
    { label: 'Empty Table', icon: ICON.empty, separatorBefore: true, onSelect: on.empty },
    { label: 'Truncate Table', icon: ICON.truncate, onSelect: on.truncate },
    { label: 'Delete Table', icon: ICON.delete, danger: true, onSelect: on.remove },
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
