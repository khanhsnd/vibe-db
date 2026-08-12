import { useCallback } from 'react'

import {
  CONNECTION_COLOURS,
  engineInfo,
  engineWords,
  hasToolkit,
  type EngineInfo,
  type EngineWords,
  type Namespace,
  type Profile,
} from '@/bridge/connections'
import { databases } from '@/bridge/databases'
import { ICON } from '@/shared/ui/menuIcons'
import { NEW_SOURCE, schema, type SourceKind } from '@/bridge/schema'
import type { TableColumn } from '@/bridge/tables'
import { backupActions, type BackupActions } from '@/features/backup/backupActions'
import type { DumpTarget } from '@/features/backup/DumpDialog'
import { fieldActions, type FieldActions } from '@/features/schema/fieldActions'
import { newSourceItems, sourceActions } from '@/features/schema/sourceActions'
import { tableActions, type TableActions } from '@/features/schema/tableActions'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { useBackupsStore } from '@/stores/backups'
import { tableKey, useConnectionsStore, type ChangeSchema } from '@/stores/connections'
import { useToolsStore } from '@/stores/dbtool'
import { useSchemaStore, type SchemaRequest } from '@/stores/schema'
import {
  COMPARE_ARROW,
  SOURCE_NOUN,
  type CompareMode,
  type SourceTarget,
  type TableTarget,
} from '@/stores/tabs'

import { colourSwatch } from './ConnectionColour'
import { ownsObjects, sourceRefOf, type TreeNode } from './tree'

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
  /** Opens a definition, read from the server or — with `creating` — declared from a skeleton. */
  onOpenSource: (target: SourceTarget) => void
  /** Raises the wizard that reads a file into a table. An empty table is one not chosen yet. */
  onImport: (target: TableTarget) => void
  /** Raises the wizard that writes tables out as files. An empty table means all of them. */
  onExport: (target: TableTarget) => void
  /** Opens the pair the reader picked side by side, in whichever half they asked for. */
  onCompare: (left: TableTarget, right: TableTarget, mode: CompareMode) => void
  /** Searches every table of a namespace for one piece of text. */
  onFind: (profileId: string, namespace: Namespace) => void
  /** Opens the namespace's tables as boxes with the keys between them drawn as lines. */
  onDiagram: (profileId: string, namespace: Namespace) => void
  /** Opens the list of backups kept for a namespace. */
  onBackups: (profileId: string, namespace: Namespace) => void
  onDump: (targets: DumpTarget[], withData: boolean) => void
}

/**
 * The tables the reader has picked, in the order they picked them — what `Ctrl` in the tree
 * builds. Compare needs exactly two, and it is the *tree's* pick rather than the menu's because
 * the second table is nearly always somewhere the first one's right-click cannot reach: another
 * database, another server.
 */
export type PickedTables = readonly TableTarget[]

/**
 * The two halves of a comparison as the menu offers them, which is the same two-way choice Dump
 * and Duplicate already read as. Structure alone is a read of two catalogues and is instant; data
 * is a bounded read of two tables, which is why it is the second of the two and never the default.
 */
const COMPARE_MODES: ReadonlyArray<{ label: string; mode: CompareMode }> = [
  { label: 'Structure Only', mode: 'structure' },
  { label: 'Data Only', mode: 'data' },
]

/**
 * What a table's right-click offers about comparing it. Two tables picked is the whole
 * precondition, so with fewer the item stays and **says what is missing** rather than vanishing:
 * a gesture nobody can see is a gesture nobody finds. It is the same reason the tree's own Delete
 * Index is drawn dead with a reason rather than left out.
 */
function compareItem(picked: PickedTables, on: TreeMenuActions['onCompare']): MenuItem {
  const [left, right] = picked

  if (!left || !right) {
    return {
      label: 'Compare — Ctrl-click a second table',
      icon: ICON.compare,
      disabled: true,
      separatorBefore: true,
    }
  }

  return {
    label: `Compare ${left.table} ${COMPARE_ARROW} ${right.table}`,
    icon: ICON.compare,
    separatorBefore: true,
    items: COMPARE_MODES.map(({ label, mode }) => ({
      label: `${label}…`,
      onSelect: () => on(left, right, mode),
    })),
  }
}

/**
 * Every right-click in the object tree, in one place. A menu item either reads — Refresh — opens
 * the designer tab, where a shape is edited with room to work, or raises the one dialog that owns
 * the statement. Nothing here changes a schema without asking first, and nothing changes one
 * silently, because whatever asked is also where a rejection is shown.
 */
export function useTreeMenu(
  actions: TreeMenuActions,
  picked: PickedTables,
  pickedNamespaces: readonly DumpTarget[] = [],
): (node: TreeNode) => MenuItem[] {
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
  const renameProfile = useConnectionsStore((s) => s.renameProfile)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const changeDatabases = useConnectionsStore((s) => s.changeDatabases)
  const dump = useConnectionsStore((s) => s.dump)
  const openBackupFolder = useConnectionsStore((s) => s.openBackupFolder)
  const refreshBackups = useConnectionsStore((s) => s.refreshBackups)
  const changeBackups = useConnectionsStore((s) => s.changeBackups)
  const reportError = useConnectionsStore((s) => s.reportError)
  const askBackup = useBackupsStore((s) => s.ask)
  const askRestore = useBackupsStore((s) => s.askRestore)
  const askTool = useToolsStore((s) => s.ask)
  const ask = useSchemaStore((s) => s.ask)
  const askAddField = useSchemaStore((s) => s.askAddField)

  const {
    onConnect,
    onEditConnection,
    onConsole,
    onExecuteFile,
    onOpenRows,
    onDesign,
    onOpenSource,
    onImport,
    onExport,
    onCompare,
    onFind,
    onDiagram,
    onBackups,
    onDump,
  } = actions

  return useCallback(
    (node: TreeNode): MenuItem[] => {
      const profile = profiles.find((p) => p.id === node.profileId)
      if (!profile) return []

      const info = engineInfo(profile.engine)
      const words = engineWords(profile.engine)

      // A Redis server's databases are fixed at start-up, so there is nothing for this to make and
      // it is not offered — see `createsDatabases`.
      const newDatabase: MenuItem | null = info.createsDatabases
        ? {
            label: 'New Database…',
            icon: ICON.newDatabase,
            onSelect: () => ask({ kind: 'database', profileId: node.profileId }),
          }
        : null

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
          onRename: (profile) =>
            ask({
              kind: 'prompt',
              title: 'Rename Connection',
              label: 'Connection name',
              value: profile.name,
              submitLabel: 'Rename',
              onSubmit: (name) => renameProfile(profile.id, name),
            }),
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
      // is what creates it, so declaring one and altering one are the same surface. A key space is
      // not declared at all, so this is not offered there — see `createsTables`.
      const newTable: MenuItem[] = info.createsTables
        ? [{ label: `New ${words.table}`, icon: ICON.add, onSelect: on.create }]
        : []

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
            onSelect: () => table === ''
              ? onDump(namespacesFor(node, pickedNamespaces), false)
              : void dump(node.profileId, { namespace: ns, table, withData: false }),
          },
          {
            label: 'Structure and Data…',
            onSelect: () => table === ''
              ? onDump(namespacesFor(node, pickedNamespaces), true)
              : void dump(node.profileId, { namespace: ns, table, withData: true }),
          },
        ],
      })

      // The two wizards are offered wherever a dump is, and cover the same subject: one table, or
      // `''` for every table the namespace holds — which the import reads as "not chosen yet", a
      // file having to land in exactly one table, and the export reads as all of them.
      //
      // They sit beside `Dump` because all three carry a table out of this server, and apart from it
      // because a dump carries *statements* — it is for putting the table back on a server, and
      // these are for everything that is not one.
      const wizards = (table: string): MenuItem[] => [
        {
          label: 'Import Wizard…',
          icon: ICON.importWizard,
          separatorBefore: true,
          onSelect: () => onImport({ profileId: node.profileId, namespace: ns, table }),
        },
        {
          label: 'Export Wizard…',
          icon: ICON.exportWizard,
          onSelect: () => onExport({ profileId: node.profileId, namespace: ns, table }),
        },
      ]

      // The engine's **own** pair of programs, named after themselves: a reader who has installed
      // MongoDB Database Tools is looking for the word `mongodump`, not for "Dump".
      //
      // They sit below the wizards because all four carry this namespace somewhere, and apart from
      // them because what these write is not this app's — it is the vendor's own format, and it
      // goes back with that vendor's own program and with nothing else. Both are drawn whether or
      // not the program is on this machine: an item nobody can see is a feature nobody finds, and
      // the dialog is where a missing program is said and answered.
      const toolItems: MenuItem[] = hasToolkit(info)
        ? (['dump', 'restore'] as const).map((mode, at) => ({
            label: `${mode === 'dump' ? info.dumpTool : info.restoreTool}…`,
            icon: mode === 'dump' ? ICON.toolDump : ICON.toolRestore,
            separatorBefore: at === 0,
            onSelect: () => askTool({ profileId: node.profileId, namespace: ns, mode }),
          }))
        : []

      // Offered wherever a namespace's tables are — the level that owns them, and the folder that
      // lists them — because that is the whole subject of a search: every table of one namespace.
      // It only ever reads, so nothing is confirmed and nothing is asked twice.
      const findInSchema: MenuItem = {
        label: 'Find in Schema…',
        icon: ICON.find,
        onSelect: () => onFind(node.profileId, ns),
      }

      // Offered beside it and for the same reason: the subject of a diagram is every table of one
      // namespace. It is left out where a namespace has no tables to draw at all — a key space is
      // one flat list of keys, and a box round it would say nothing (see `createsTables`).
      const diagram: MenuItem[] = info.createsTables
        ? [{ label: 'Diagram', icon: ICON.diagram, onSelect: () => onDiagram(node.profileId, ns) }]
        : []

      // Everything a backup is offered, wherever it is offered — the toolbar over the list builds
      // from the same call. A failure with no dialog behind it goes where a dump's does: the bar.
      const backup = backupActions(node.profileId, ns, node.label, {
        ask,
        askRestore,
        changeBackups,
        report: reportError,
      })

      // The backup folder has a read of its own, so it has a Refresh of its own: the catalogue
      // read below would not notice a file appearing on disk, and this one asks no server anything.
      const refreshBackupList: MenuItem = {
        label: 'Refresh',
        icon: ICON.refresh,
        onSelect: () => void refreshBackups(node.profileId, ns),
      }

      // One read fills every object folder, so Refresh means the same thing wherever it is asked
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
            ? [...newTable, findInSchema, ...diagram, dumpItems(''), ...wizards(''), ...toolItems, refreshObjectList]
            : [
                {
                  label: 'Refresh',
                  icon: ICON.refresh,
                  separatorBefore: true,
                  onSelect: () => void refreshSchemas(node.profileId, ns.database),
                },
              ]

          // Only the database level can make or drop a database; a schema sits inside one. An
          // engine whose databases are fixed has neither, and its database node is its objects.
          if (node.kind !== 'database' || !newDatabase) return own
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
          return [...newTable, findInSchema, ...diagram, dumpItems(''), ...wizards(''), ...toolItems, refreshObjectList]

        // A folder offers what goes in it, which on `Routines` is both kinds — no engine has a
        // folder for a function alone. Neither is offered where this engine's objects are not
        // declared by a statement at all; see `hasSource`.
        case 'views':
        case 'routines': {
          if (!info.hasSource) return [{ ...refreshObjectList, separatorBefore: false }]

          const kinds = node.kind === 'views' ? VIEW_KIND : ROUTINE_KINDS
          const on = sourceActions(
            node.profileId,
            ns,
            { ...NEW_SOURCE, kind: kinds[0] },
            { ask, changeSchema, openSource: onOpenSource },
          )
          return [...newSourceItems(kinds, on.create), refreshObjectList]
        }

        // The one object folder with no New of its own: a trigger is declared *about* a table, and
        // this folder names none. So the item stays and **says where it is** rather than vanishing —
        // the same rule Compare follows with its second table, and for the same reason.
        case 'triggers':
          return [
            {
              label: 'New Trigger — right-click a table',
              icon: ICON.add,
              disabled: true,
            },
            refreshObjectList,
          ]

        // The one folder under a namespace whose menu asks no server anything. Restore is not here:
        // a restore is addressed to *one* file, and the file is what carries it — the folder offers
        // the list instead, which is where the reader sees which files there are.
        case 'backups':
          return [
            {
              label: 'New Backup…',
              icon: ICON.add,
              onSelect: () => askBackup({ profileId: node.profileId, namespace: ns }),
            },
            {
              label: 'Backup List',
              icon: ICON.open,
              onSelect: () => onBackups(node.profileId, ns),
            },
            {
              label: 'Open Containing Folder',
              icon: ICON.openFolder,
              separatorBefore: true,
              onSelect: () => void openBackupFolder(node.profileId, ns),
            },
            { ...refreshBackupList, separatorBefore: true },
          ]

        case 'backup':
          return backupItems(node.label, {
            ...backup,
            refresh: refreshBackupList,
            openFolder: () => void openBackupFolder(node.profileId, ns),
          })

        // A trigger is declared *about* a table, and this is the only node that names one — which is
        // why New Trigger is offered here and nowhere else, the `Triggers` folder saying so instead.
        case 'table': {
          const trigger = sourceActions(
            node.profileId,
            ns,
            { ...NEW_SOURCE, kind: 'trigger', table: node.label },
            { ask, changeSchema, openSource: onOpenSource },
          )

          return tableItems(
            on,
            () => void refreshStructure(node.profileId, ns, node.label),
            dumpItems(node.label),
            wizards(node.label),
            compareItem(picked, onCompare),
            info,
            info.hasTriggers && info.hasSource
              ? newSourceItems(TRIGGER_KIND, trigger.create)
              : [],
          )
        }

        // A view has rows and a shape like a table, so it compares like one — and it has a
        // *definition*, which is the one thing about it no grid could ever draw.
        case 'view': {
          const on = sourceActions(
            node.profileId,
            ns,
            { ...NEW_SOURCE, kind: 'view', name: node.label },
            { ask, changeSchema, openSource: onOpenSource },
          )

          return [
            {
              label: 'Open View',
              icon: ICON.open,
              onSelect: () =>
                onOpenRows({ profileId: node.profileId, namespace: ns, table: node.label }),
            },
            ...sourceItems(info, 'view', on),
            compareItem(picked, onCompare),
            {
              label: 'Delete View',
              icon: ICON.delete,
              danger: true,
              separatorBefore: true,
              onSelect: on.remove,
            },
            refreshObjectList,
          ]
        }

        // A routine is addressed by the whole of what the tree holds for it and never by its name:
        // on PostgreSQL two functions may share one and be told apart only by what they take. A
        // trigger is the same arrangement under a different second fact — the table it fires on.
        case 'procedure':
        case 'function':
        case 'trigger': {
          const ref = sourceRefOf(node)
          if (!ref) return [{ ...refreshObjectList, separatorBefore: false }]

          const on = sourceActions(node.profileId, ns, ref, {
            ask,
            changeSchema,
            openSource: onOpenSource,
          })

          return [
            ...sourceItems(info, ref.kind, on),
            {
              label: `Delete ${SOURCE_NOUN[ref.kind]}`,
              icon: ICON.delete,
              danger: true,
              separatorBefore: true,
              onSelect: on.remove,
            },
            refreshObjectList,
          ]
        }

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
      openBackupFolder,
      refreshBackups,
      changeBackups,
      reportError,
      askBackup,
      askRestore,
      askTool,
      ask,
      askAddField,
      onConnect,
      onEditConnection,
      onConsole,
      onExecuteFile,
      onOpenRows,
      onDesign,
      onOpenSource,
      onImport,
      onExport,
      onCompare,
      onFind,
      onDiagram,
      onBackups,
      onDump,
      picked,
      pickedNamespaces,
    ],
  )
}

function namespacesFor(node: TreeNode, picked: readonly DumpTarget[]): DumpTarget[] {
  const same = picked.filter((one) => one.profileId === node.profileId)
  return same.length > 0 ? same : [{ profileId: node.profileId, namespace: node.namespace! }]
}

/**
 * Everything one backup is offered, in the order a reader reaches for it: put it back, take it
 * somewhere, keep a second copy, retitle it, throw it away — then the folder it is in.
 *
 * Restore is the only one that touches a server, so it is alone above the first divider and is the
 * only one drawn as a change. Delete is the only one that loses anything, so it is alone above the
 * last one and is drawn in the danger colour, exactly as every other Delete in the tree is.
 */
export function backupItems(
  name: string,
  on: BackupActions & { refresh: MenuItem; openFolder: () => void },
): MenuItem[] {
  return [
    { label: 'Restore Backup…', icon: ICON.restoreBackup, onSelect: on.restore },
    { label: 'Extract SQL…', icon: ICON.dump, separatorBefore: true, onSelect: on.extractSQL },
    // A copy is named after the original rather than asked for, the original's name being a
    // moment and a moment being the one thing a copy of it does not have a new one of.
    { label: `Copy ${name}`, icon: ICON.duplicate, onSelect: on.copy },
    { label: 'Rename…', icon: ICON.rename, shortcut: 'F2', onSelect: on.rename },
    {
      label: 'Delete Backup',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: on.remove,
    },
    {
      label: 'Open Containing Folder',
      icon: ICON.openFolder,
      separatorBefore: true,
      onSelect: on.openFolder,
    },
    on.refresh,
  ]
}

/** The kinds each object folder holds: no engine has a folder for a function alone. */
const VIEW_KIND = ['view'] as const satisfies readonly SourceKind[]
const ROUTINE_KINDS = ['function', 'procedure'] as const satisfies readonly SourceKind[]
const TRIGGER_KIND = ['trigger'] as const satisfies readonly SourceKind[]

/** The kinds a `New …` beside this one offers, which is whatever its own folder holds. */
function siblingKinds(kind: SourceKind): readonly SourceKind[] {
  if (kind === 'view') return VIEW_KIND
  if (kind === 'trigger') return TRIGGER_KIND
  return ROUTINE_KINDS
}

/**
 * Opening one of these definitions, and declaring another beside it — the pair every object with a
 * definition is offered, and neither where this engine's objects are not declared by a statement.
 * It is one function because the two always travel together: a reader who can open a definition
 * can write one.
 */
function sourceItems(
  info: EngineInfo,
  kind: SourceKind,
  on: ReturnType<typeof sourceActions>,
): MenuItem[] {
  if (!info.hasSource) return []

  return [
    { label: `Design ${SOURCE_NOUN[kind]}`, icon: ICON.design, onSelect: on.design },
    ...newSourceItems(siblingKinds(kind), on.create),
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
    newDatabase: MenuItem | null
    onConnect: (profile: Profile) => void
    onEditConnection: (profile: Profile) => void
    onConsole: (profile: Profile) => void
    onExecuteFile: (profile: Profile) => void
    disconnect: (profileId: string) => Promise<void>
    deleteProfile: (profileId: string) => Promise<void>
    setColour: (profileId: string, colour: string) => Promise<void>
    onRename: (profile: Profile) => void
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
        ...(on.newDatabase ? [on.newDatabase] : []),
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
      label: 'Rename…',
      icon: ICON.rename,
      shortcut: 'F2',
      onSelect: () => on.onRename(profile),
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
 * `dump`, `wizards` and `compare` arrive built rather than being made here, because the same items
 * are offered elsewhere and differ only in what they name — one table or all of them for the first
 * two, and for the third the *pair* the tree has picked, which no one table's menu could know.
 *
 * Two groups of items are the engine's rather than the table's. **Declaring one** — designing,
 * making, renaming, duplicating and dropping — is left out where a table is not declared at all: a
 * Redis database has one key space, always has had, and there is nothing there to make or remove
 * (`createsTables`). And **Truncate** is left out where it is not a second thing: on a document
 * store every document has to be deleted to empty a collection, and on a key space `FLUSHDB` is
 * both, so the two would be one command under two names (`hasTruncate`).
 *
 * `triggers` is the New a trigger has to be offered from, this being the only node that names the
 * table one would fire on. It arrives built rather than being made here, for the reason `compare`
 * does: what it opens is a definition tab, which no table action knows about.
 */
export function tableItems(
  on: TableActions,
  refresh: () => void,
  dump: MenuItem,
  wizards: MenuItem[],
  compare: MenuItem,
  info: EngineInfo = engineInfo(''),
  triggers: MenuItem[] = [],
): MenuItem[] {
  const words = engineWords(info.value)

  const declaring: MenuItem[] = info.createsTables
    ? [
        { label: `New ${words.table}`, icon: ICON.add, separatorBefore: true, onSelect: on.create },
        { label: `Rename ${words.table}…`, icon: ICON.rename, shortcut: 'F2', onSelect: on.rename },
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
      ]
    : []

  return [
    { label: `Open ${words.table}`, icon: ICON.open, onSelect: on.open },
    ...(info.createsTables
      ? [{ label: `Design ${words.table}`, icon: ICON.design, onSelect: on.design }]
      : []),
    // Beside Open and Design rather than beside Dump: comparing only ever *reads*, and the three
    // items under it all carry the table somewhere else.
    compare,
    ...declaring,
    // With the rest of what is *declared* here, since that is what it is — and after them, a
    // trigger being about the table rather than of it.
    ...triggers,
    // Beside Duplicate rather than beside Delete: both copy the table, and the only difference is
    // whether the copy lands on this server or in a file.
    dump,
    ...wizards,
    { label: `Empty ${words.table}`, icon: ICON.empty, separatorBefore: true, onSelect: on.empty },
    ...(info.hasTruncate
      ? [{ label: `Truncate ${words.table}`, icon: ICON.truncate, onSelect: on.truncate }]
      : []),
    ...(info.createsTables
      ? [{ label: `Delete ${words.table}`, icon: ICON.delete, danger: true, onSelect: on.remove }]
      : []),
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
    {
      label: 'Rename Field…',
      icon: ICON.rename,
      shortcut: 'F2',
      onSelect: () => on.rename(stored),
    },
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
