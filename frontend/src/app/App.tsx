import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  authInfo,
  engineInfo,
  namespaceLabel,
  namespaceOf,
  NO_NAMESPACE,
  toProfileInput,
  type Profile,
  type ProfileInput,
} from '@/bridge/connections'
import { NewBackupDialog } from '@/features/backup/NewBackupDialog'
import { RestoreBackupDialog } from '@/features/backup/RestoreBackupDialog'
import { ToolDialog } from '@/features/dbtool/ToolDialog'
import { ConnectionDialog } from '@/features/connections/ConnectionDialog'
import { ExecuteFileDialog } from '@/features/connections/ExecuteFileDialog'
import { PasswordPrompt } from '@/features/connections/PasswordPrompt'
import {
  hasRows,
  isExpandable,
  isObjectFolder,
  ownsObjects,
  searchTree,
  type PickHow,
  type TreeNode,
} from '@/features/connections/tree'
import { useTreeMenu } from '@/features/connections/useTreeMenu'
import { SchemaDialogs } from '@/features/schema/SchemaDialogs'
import { ExportWizard, type ExportTarget } from '@/features/transfer/ExportWizard'
import { ImportWizard, type ImportTarget } from '@/features/transfer/ImportWizard'
import { Split, SplitHandle, SplitPanel, usePanelRef } from '@/shared/ui/Split'
import { useBackupsStore } from '@/stores/backups'
import { buildTree, useConnectionsStore } from '@/stores/connections'
import { useSettingsStore } from '@/stores/settings'
import { useTabsStore } from '@/stores/tabs'
import { useUIStore } from '@/stores/ui'

import { CommandBar } from './layout/CommandBar'
import { MenuBar } from './layout/MenuBar'
import { Sidebar } from './layout/Sidebar'
import { StatusBar, type StatusInfo } from './layout/StatusBar'
import { WorkArea } from './layout/WorkArea'

const SIDEBAR_DEFAULT_WIDTH = 240
const SIDEBAR_MIN_WIDTH = 180
const SIDEBAR_MAX_WIDTH = 480

export function App() {
  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed)
  const sidebarResized = useUIStore((s) => s.sidebarResized)
  const sidebarRef = usePanelRef()
  const openTab = useTabsStore((s) => s.open)
  const openTable = useTabsStore((s) => s.openTable)
  const openDesign = useTabsStore((s) => s.openDesign)
  const openSource = useTabsStore((s) => s.openSource)
  const openTables = useTabsStore((s) => s.openTables)
  const openBackups = useTabsStore((s) => s.openBackups)
  const openConsole = useTabsStore((s) => s.openConsole)
  const openCompare = useTabsStore((s) => s.openCompare)
  const openFind = useTabsStore((s) => s.openFind)
  const activeTab = useTabsStore((s) => s.tabs.find((t) => t.id === s.activeId) ?? null)

  const profiles = useConnectionsStore((s) => s.profiles)
  const sessions = useConnectionsStore((s) => s.sessions)
  const statuses = useConnectionsStore((s) => s.statuses)
  const databases = useConnectionsStore((s) => s.databases)
  const schemas = useConnectionsStore((s) => s.schemas)
  const objects = useConnectionsStore((s) => s.objects)
  const backups = useConnectionsStore((s) => s.backups)
  const structures = useConnectionsStore((s) => s.structures)
  const activeNamespace = useConnectionsStore((s) => s.activeNamespace)
  const expanded = useConnectionsStore((s) => s.expanded)
  const error = useConnectionsStore((s) => s.error)
  const loadProfiles = useConnectionsStore((s) => s.loadProfiles)
  const connect = useConnectionsStore((s) => s.connect)
  const selectNamespace = useConnectionsStore((s) => s.selectNamespace)
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)
  const loadObjects = useConnectionsStore((s) => s.loadObjects)
  const loadBackups = useConnectionsStore((s) => s.loadBackups)
  const loadStructure = useConnectionsStore((s) => s.loadStructure)
  const toggleNode = useConnectionsStore((s) => s.toggleNode)
  const askBackup = useBackupsStore((s) => s.ask)

  const roots = useMemo(
    () =>
      buildTree({
        profiles,
        statuses,
        databases,
        schemas,
        objects,
        backups,
        structures,
        activeNamespace,
      }),
    [profiles, statuses, databases, schemas, objects, backups, structures, activeNamespace],
  )

  /**
   * Every row the reader has picked, in the order they picked them, and the **last is the focused
   * one** — the same arrangement the grid's own selection has, and for the same reason: everything
   * that acts on one row acts on the last press, and Compare acts on two.
   */
  const [picked, setPicked] = useState<TreeNode[]>([])
  const selected = picked.at(-1) ?? null
  const pickedIds = useMemo(() => new Set(picked.map((node) => node.id)), [picked])
  const [filter, setFilter] = useState('')

  // The tables among what is picked, in the order they were picked. Compare is offered on two of
  // them, and anything that is not a table — a folder, a database — is simply not one of the two.
  const pickedTables = useMemo(
    () =>
      picked.flatMap((node) =>
        hasRows(node) && node.namespace
          ? [{ profileId: node.profileId, namespace: node.namespace, table: node.label }]
          : [],
      ),
    [picked],
  )

  // Searching narrows the tree it is looking at and opens what it had to open to show a hit, so the
  // reader's own expansion survives the search and is what is left when the box is emptied.
  const search = useMemo(() => searchTree(roots, filter), [roots, filter])
  const shownRoots = search?.roots ?? roots
  const shownExpanded = useMemo(
    () => (search ? new Set([...expanded, ...search.expanded]) : expanded),
    [search, expanded],
  )

  const [dialogProfile, setDialogProfile] = useState<ProfileInput | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [promptFor, setPromptFor] = useState<Profile | null>(null)
  /** The connection a file is being run against, and non-null exactly while that dialog is up. */
  const [fileFor, setFileFor] = useState<Profile | null>(null)
  /** Where each transfer wizard is pointed, and non-null exactly while it is up. */
  const [importInto, setImportInto] = useState<ImportTarget | null>(null)
  const [exportFrom, setExportFrom] = useState<ExportTarget | null>(null)

  useEffect(() => {
    void loadProfiles()
  }, [loadProfiles])

  useEffect(() => {
    const panel = sidebarRef.current
    if (!panel) return
    if (sidebarCollapsed) panel.collapse()
    else panel.expand()
  }, [sidebarCollapsed, sidebarRef])

  const openNewConnection = useCallback(() => {
    setDialogProfile(null)
    setDialogOpen(true)
  }, [])

  const openEditConnection = useCallback((profile: Profile) => {
    setDialogProfile(toProfileInput(profile))
    setDialogOpen(true)
  }, [])

  const startConnect = useCallback(
    (profile: Profile) => {
      // A cloud identity has no password to ask for: one is signed for at the moment of
      // connecting, and a prompt would be a box the reader has nothing to type into.
      if (!profile.hasPassword && !authInfo(profile.auth).minted) {
        setPromptFor(profile)
        return
      }
      void connect(profile.id, '').catch((thrown) => {
        if (thrown instanceof Error && 'code' in thrown && thrown.code === 'BAD_CREDENTIALS') {
          setPromptFor(profile)
        }
      })
    },
    [connect],
  )

  // Each level loads what the level below it will need: a PostgreSQL database its schemas, any
  // of a namespace's three folders that namespace's objects, and any of a table's three folders
  // that table's shape. One read fills all three folders either way.
  const revealNode = useCallback(
    (node: TreeNode) => {
      const ns = node.namespace
      if (!ns) return

      if (node.kind === 'database' && !ownsObjects(node)) {
        void loadSchemas(node.profileId, ns.database)
      }
      if (ownsObjects(node) || isObjectFolder(node)) void loadObjects(node.profileId, ns)
      // Backups is not one of those three folders and is not filled by their read: it is a folder
      // on this machine, and nothing the catalogue answered with would ever mention it.
      if (node.kind === 'backups') void loadBackups(node.profileId, ns)
      if (node.table !== undefined) void loadStructure(node.profileId, ns, node.table)
    },
    [loadSchemas, loadObjects, loadBackups, loadStructure],
  )

  // Opening a closed connection means dialling it: a disconnected connection node still carries
  // an (empty) child list, so a plain toggle only spins the caret over nothing. The store expands
  // the node itself once the session lands.
  const toggleTreeNode = useCallback(
    (node: TreeNode) => {
      if (node.kind === 'connection' && !sessions[node.profileId]) {
        const profile = profiles.find((p) => p.id === node.profileId)
        if (profile && statuses[node.profileId] !== 'connecting') startConnect(profile)
        return
      }

      toggleNode(node.id)
      revealNode(node)
    },
    [sessions, profiles, statuses, startConnect, toggleNode, revealNode],
  )

  // A view's rows are read exactly as a table's are, so they open the same tab — and land in it
  // read-only, since no column of a view is a primary key and no single stored row is named.
  const openNodeRows = useCallback(
    (node: TreeNode) => {
      if (!hasRows(node) || !node.namespace) return
      openTable({ profileId: node.profileId, namespace: node.namespace, table: node.label })
    },
    [openTable],
  )

  // A console opens where the reader already is on that connection: the namespace the tree has
  // selected, or the one the profile connects to, so the first statement typed has somewhere to run.
  const openConnectionConsole = useCallback(
    (profile: Profile) => {
      openConsole(profile.id, activeNamespace[profile.id] ?? namespaceOf(profile.database))
    },
    [openConsole, activeNamespace],
  )

  const menuItemsFor = useTreeMenu(
    {
      onConnect: startConnect,
      onEditConnection: openEditConnection,
      onConsole: openConnectionConsole,
      onExecuteFile: setFileFor,
      onOpenRows: openTable,
      onDesign: openDesign,
      onOpenSource: openSource,
      onImport: setImportInto,
      onExport: setExportFrom,
      onCompare: openCompare,
      onFind: openFind,
      onBackups: openBackups,
    },
    pickedTables,
  )

  // Selecting reads nothing and dials nothing, and expanding stays the caret's job. The one
  // thing it opens is the Tables folder's own list, which is what that folder *is* to a reader:
  // the tree draws the names, and the list beside it draws what the catalogue keeps about them.
  //
  // What a press does to the pick is `how`'s: a plain one starts over, `Ctrl` toggles this row
  // beside whatever is there, and a right-click that landed *inside* the pick leaves it whole so
  // that the pair a menu is about survives being right-clicked. Only a press that names one row
  // reads a namespace or opens a list — picking a second table is not travelling to it.
  const selectNode = useCallback(
    (node: TreeNode, how: PickHow) => {
      if (how === 'add') {
        setPicked((current) =>
          current.some((one) => one.id === node.id)
            ? current.filter((one) => one.id !== node.id)
            : [...current, node],
        )
        return
      }

      if (how === 'menu' && picked.length > 1 && pickedIds.has(node.id)) return
      setPicked([node])

      // The node that owns tables is the one a query tab can run against. A PostgreSQL
      // database is not that node — its schemas are — so selecting one names nothing.
      if (node.namespace && ownsObjects(node)) selectNamespace(node.profileId, node.namespace)

      // Neither of the two folders that open a list is opened by picking a *second* row, and
      // neither is opened twice by the caret, which does its own expanding — see `PickHow`.
      const opens = (how === 'replace' || how === 'caret') && node.namespace !== undefined
      if (!opens || !node.namespace) return

      if (node.kind === 'tables') openTables(node.profileId, node.namespace)

      // Backups is the one folder a plain press also *expands*: the tree has room for the names
      // and the grid beside it has room for the date, the size and the comment, and a reader
      // reaching for a backup wants whichever of the two is nearer.
      if (node.kind === 'backups') {
        openBackups(node.profileId, node.namespace)
        if (how === 'replace') toggleTreeNode(node)
      }
    },
    [picked.length, pickedIds, selectNamespace, openTables, openBackups, toggleTreeNode],
  )

  // A table opens its data rather than its folders, even though it now has folders to open: the
  // caret is what expands, and a double-click on a table has always meant "show me the rows".
  const activateNode = useCallback(
    (node: TreeNode) => {
      if (hasRows(node)) openNodeRows(node)
      else if (isExpandable(node)) toggleTreeNode(node)
    },
    [toggleTreeNode, openNodeRows],
  )

  // A new editor inherits the selected connection and namespace, falling back to
  // whichever connection is open.
  const newQuery = useCallback(() => {
    const profileId = selected?.profileId ?? Object.keys(sessions)[0] ?? ''
    const namespace = selected?.namespace ?? activeNamespace[profileId] ?? NO_NAMESPACE
    const count = useTabsStore.getState().tabs.filter((tab) => tab.kind === 'query').length

    openTab({
      kind: 'query',
      title: `Query ${count + 1}`,
      profileId,
      namespace,
      sql: '',
      rowLimit: useSettingsStore.getState().settings.queryRows,
    })
  }, [openTab, selected, sessions, activeNamespace])

  /**
   * Taking a backup from the bar, of wherever the reader already is — the selected row's namespace,
   * falling back to whichever connection is open, exactly as New Query picks its target.
   *
   * It is `null` where there is nothing to back up, which is what greys the button: no live
   * connection, or one whose engine has no such thing — see `hasBackups`. A button that opened a
   * dialog only to have the Go side refuse it would be asking the reader to find that out twice.
   */
  const newBackup = useMemo(() => {
    const profileId = selected?.profileId ?? Object.keys(sessions)[0] ?? ''
    if (profileId === '' || !sessions[profileId]) return null

    const engine = profiles.find((profile) => profile.id === profileId)?.engine ?? ''
    if (!engineInfo(engine).hasBackups) return null

    const namespace = selected?.namespace ?? activeNamespace[profileId] ?? NO_NAMESPACE
    if (namespace.database === '') return null

    return () => askBackup({ profileId, namespace })
  }, [selected, sessions, profiles, activeNamespace, askBackup])

  // The bar reports the connection the reader is *looking at* — the tab in front, or the row they
  // have selected where no tab is open. Reporting whichever session happened to be first would
  // name one server while the grid under it drew another's rows.
  const status = useMemo<StatusInfo>(() => {
    const profileId = activeTab?.profileId ?? selected?.profileId ?? Object.keys(sessions)[0] ?? ''
    const session = sessions[profileId]
    const profile = profiles.find((p) => p.id === profileId)
    const open = Object.keys(sessions).length

    if (!session) {
      return {
        status: error ? 'error' : 'idle',
        name: profile?.name ?? null,
        target: error ? error.message : null,
        engine: profile?.engine ?? null,
        environment: profile?.environment ?? '',
        serverVersion: null,
        namespace: '',
        connections: open,
        rowCount: null,
        elapsedMs: null,
        // The failure is already standing where the server would be, so it is not said twice.
        error: null,
      }
    }

    const ns = activeTab?.namespace ?? activeNamespace[profileId] ?? NO_NAMESPACE

    return {
      status: 'connected',
      name: profile?.name ?? null,
      target: session.target,
      engine: profile?.engine ?? null,
      environment: profile?.environment ?? '',
      serverVersion: session.serverVersion,
      namespace: namespaceLabel(ns),
      connections: open,
      rowCount: null,
      elapsedMs: null,
      // A live connection has its host to report, so a failure nothing was waiting on gets a fact of
      // its own instead — otherwise the one action asked for from a menu rather than from a dialog,
      // a dump, could fail with nowhere to say so.
      error: error?.message ?? null,
    }
  }, [activeTab, selected, sessions, profiles, activeNamespace, error])

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <MenuBar />

      <CommandBar
        onNewConnection={openNewConnection}
        onNewQuery={newQuery}
        onBackup={newBackup}
      />

      <Split
        id="vibe-db.shell"
        orientation="horizontal"
        panelIds={['sidebar', 'work']}
        className="min-h-0 flex-1"
      >
        <SplitPanel
          id="sidebar"
          panelRef={sidebarRef}
          collapsible
          collapsedSize={0}
          defaultSize={SIDEBAR_DEFAULT_WIDTH}
          minSize={SIDEBAR_MIN_WIDTH}
          maxSize={SIDEBAR_MAX_WIDTH}
          // Dragging the divider past the minimum closes the sidebar as well, so the button that
          // opens it has to hear about it from here — see `sidebarResized`.
          onResize={(size) => sidebarResized(size.inPixels === 0)}
        >
          <Sidebar
            roots={shownRoots}
            expanded={shownExpanded}
            pickedIds={pickedIds}
            focusedId={selected?.id ?? null}
            filter={filter}
            onFilterChange={setFilter}
            onToggle={toggleTreeNode}
            onSelect={selectNode}
            onActivate={activateNode}
            onNewConnection={openNewConnection}
            menuItemsFor={menuItemsFor}
          />
        </SplitPanel>

        <SplitHandle orientation="horizontal" />

        <SplitPanel id="work" minSize="30%">
          <WorkArea onNewQuery={newQuery} />
        </SplitPanel>
      </Split>

      <StatusBar {...status} />

      <ConnectionDialog open={dialogOpen} initial={dialogProfile} onOpenChange={setDialogOpen} />

      <ExecuteFileDialog
        profile={fileFor}
        onOpenChange={(open) => {
          if (!open) setFileFor(null)
        }}
      />

      <ImportWizard
        target={importInto}
        onOpenChange={(open) => {
          if (!open) setImportInto(null)
        }}
      />

      <ExportWizard
        target={exportFrom}
        onOpenChange={(open) => {
          if (!open) setExportFrom(null)
        }}
      />

      <NewBackupDialog />

      <RestoreBackupDialog />

      <ToolDialog />

      <SchemaDialogs />

      <PasswordPrompt
        open={promptFor !== null}
        target={promptFor ? `${promptFor.user}@${promptFor.host}:${promptFor.port}` : ''}
        onOpenChange={(open) => {
          if (!open) setPromptFor(null)
        }}
        onSubmit={(password) => {
          if (promptFor) void connect(promptFor.id, password).catch(() => {})
          setPromptFor(null)
        }}
      />
    </div>
  )
}
