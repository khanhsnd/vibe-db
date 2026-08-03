import { useCallback } from 'react'

import type { Namespace, Profile } from '@/bridge/connections'
import { ICON } from '@/shared/ui/menuIcons'
import { schema } from '@/bridge/schema'
import type { TableColumn } from '@/bridge/tables'
import { fieldActions, type FieldActions } from '@/features/schema/fieldActions'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { tableKey, useConnectionsStore, type ChangeSchema } from '@/stores/connections'
import { useSchemaStore, type SchemaRequest } from '@/stores/schema'

import { ownsObjects, type TreeNode } from './tree'

interface TreeMenuActions {
  onConnect: (profile: Profile) => void
  onEditConnection: (profile: Profile) => void
  onOpenTable: (node: TreeNode) => void
  /** Opens the designer on a table, or on a new one when the table is empty. */
  onDesign: (target: DesignTarget) => void
}

export interface DesignTarget {
  profileId: string
  namespace: Namespace
  table: string
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
  const refreshTables = useConnectionsStore((s) => s.refreshTables)
  const refreshSchemas = useConnectionsStore((s) => s.refreshSchemas)
  const refreshStructure = useConnectionsStore((s) => s.refreshStructure)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const ask = useSchemaStore((s) => s.ask)
  const askAddField = useSchemaStore((s) => s.askAddField)

  const { onConnect, onEditConnection, onOpenTable, onDesign } = actions

  return useCallback(
    (node: TreeNode): MenuItem[] => {
      const profile = profiles.find((p) => p.id === node.profileId)
      if (!profile) return []

      if (node.kind === 'connection') {
        return connectionItems(node, profile, Boolean(sessions[node.profileId]), {
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

      // A new table is designed in a tab like any other, with an empty name: the designer's Save
      // is what creates it, so declaring one and altering one are the same surface.
      const newTable: MenuItem = {
        label: 'New Table',
        icon: ICON.add,
        onSelect: () => onDesign({ profileId: node.profileId, namespace: ns, table: '' }),
      }

      const refreshTableList: MenuItem = {
        label: 'Refresh',
        icon: ICON.refresh,
        separatorBefore: true,
        onSelect: () => void refreshTables(node.profileId, ns),
      }

      switch (node.kind) {
        case 'database':
        case 'schema':
          return ownsObjects(node)
            ? [newTable, refreshTableList]
            : [
                {
                  label: 'Refresh',
                  icon: ICON.refresh,
                  onSelect: () => void refreshSchemas(node.profileId, ns.database),
                },
              ]

        case 'folder':
          return [newTable, refreshTableList]

        case 'table':
          return tableItems(node, ns, {
            newTable,
            change,
            ask,
            refreshStructure,
            onOpenTable,
            onDesign,
          })

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
      refreshTables,
      refreshSchemas,
      refreshStructure,
      changeSchema,
      ask,
      askAddField,
      onConnect,
      onEditConnection,
      onOpenTable,
      onDesign,
    ],
  )
}

function connectionItems(
  node: TreeNode,
  profile: Profile,
  connected: boolean,
  on: {
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

interface TableMenuDeps {
  newTable: MenuItem
  change: (run: ChangeSchema, table?: string) => () => Promise<void>
  ask: (request: SchemaRequest) => void
  refreshStructure: (profileId: string, ns: Namespace, table: string) => Promise<void>
  onOpenTable: (node: TreeNode) => void
  onDesign: (target: DesignTarget) => void
}

function tableItems(node: TreeNode, ns: Namespace, deps: TableMenuDeps): MenuItem[] {
  const table = node.label

  return [
    { label: 'Open Table', icon: ICON.open, onSelect: () => deps.onOpenTable(node) },
    {
      label: 'Design Table',
      icon: ICON.design,
      onSelect: () => deps.onDesign({ profileId: node.profileId, namespace: ns, table }),
    },
    { ...deps.newTable, separatorBefore: true },
    {
      label: 'Rename Table…',
      icon: ICON.rename,
      onSelect: () =>
        deps.ask({
          kind: 'prompt',
          title: 'Rename Table',
          label: 'New name',
          value: table,
          submitLabel: 'Rename',
          onSubmit: (name) =>
            deps.change((sessionId) => schema.renameTable(sessionId, ns, table, name), name)(),
        }),
    },
    {
      label: 'Empty Table',
      icon: ICON.empty,
      separatorBefore: true,
      onSelect: () =>
        deps.ask({
          kind: 'confirm',
          title: 'Empty Table',
          message: `Delete every row of ${table}? The table and its shape stay; the rows do not.`,
          submitLabel: 'Empty',
          danger: true,
          onSubmit: deps.change((sessionId) => schema.emptyTable(sessionId, ns, table), table),
        }),
    },
    {
      label: 'Truncate Table',
      icon: ICON.truncate,
      onSelect: () =>
        deps.ask({
          kind: 'confirm',
          title: 'Truncate Table',
          message:
            `Truncate ${table}? Every row goes at once, which is faster than emptying it and ` +
            `cannot be rolled back per row — and a table another table points at will refuse.`,
          submitLabel: 'Truncate',
          danger: true,
          onSubmit: deps.change((sessionId) => schema.truncateTable(sessionId, ns, table), table),
        }),
    },
    {
      label: 'Delete Table',
      icon: ICON.delete,
      danger: true,
      onSelect: () =>
        deps.ask({
          kind: 'confirm',
          title: 'Delete Table',
          message: `Drop ${table}? The table, its rows and everything indexing them go with it.`,
          submitLabel: 'Delete',
          danger: true,
          onSubmit: deps.change((sessionId) => schema.dropTable(sessionId, ns, table)),
        }),
    },
    {
      label: 'Refresh',
      icon: ICON.refresh,
      separatorBefore: true,
      onSelect: () => void deps.refreshStructure(node.profileId, ns, table),
    },
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
