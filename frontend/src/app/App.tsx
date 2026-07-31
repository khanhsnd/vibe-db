import { useCallback, useEffect, useMemo, useState } from 'react'

import { NO_NAMESPACE, toProfileInput, type Profile, type ProfileInput } from '@/bridge/connections'
import { DEFAULT_QUERY_ROWS } from '@/bridge/rows'
import { ConnectionDialog } from '@/features/connections/ConnectionDialog'
import { PasswordPrompt } from '@/features/connections/PasswordPrompt'
import { isExpandable, ownsObjects, type TreeNode } from '@/features/connections/tree'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { Split, SplitHandle, SplitPanel, usePanelRef } from '@/shared/ui/Split'
import { buildTree, useConnectionsStore } from '@/stores/connections'
import { useTabsStore } from '@/stores/tabs'
import { useUIStore } from '@/stores/ui'

import { CommandBar } from './layout/CommandBar'
import { Sidebar } from './layout/Sidebar'
import { StatusBar, type StatusInfo } from './layout/StatusBar'
import { WorkArea } from './layout/WorkArea'

const SIDEBAR_DEFAULT_WIDTH = 240
const SIDEBAR_MIN_WIDTH = 180
const SIDEBAR_MAX_WIDTH = 480

export function App() {
  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed)
  const sidebarRef = usePanelRef()
  const openTab = useTabsStore((s) => s.open)

  const profiles = useConnectionsStore((s) => s.profiles)
  const sessions = useConnectionsStore((s) => s.sessions)
  const statuses = useConnectionsStore((s) => s.statuses)
  const databases = useConnectionsStore((s) => s.databases)
  const schemas = useConnectionsStore((s) => s.schemas)
  const tables = useConnectionsStore((s) => s.tables)
  const activeNamespace = useConnectionsStore((s) => s.activeNamespace)
  const expanded = useConnectionsStore((s) => s.expanded)
  const error = useConnectionsStore((s) => s.error)
  const loadProfiles = useConnectionsStore((s) => s.loadProfiles)
  const connect = useConnectionsStore((s) => s.connect)
  const disconnect = useConnectionsStore((s) => s.disconnect)
  const deleteProfile = useConnectionsStore((s) => s.deleteProfile)
  const selectNamespace = useConnectionsStore((s) => s.selectNamespace)
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)
  const loadTables = useConnectionsStore((s) => s.loadTables)
  const toggleNode = useConnectionsStore((s) => s.toggleNode)

  const roots = useMemo(
    () =>
      buildTree({
        profiles,
        statuses,
        databases,
        schemas,
        tables,
        activeNamespace,
      }),
    [profiles, statuses, databases, schemas, tables, activeNamespace],
  )

  const [selected, setSelected] = useState<TreeNode | null>(null)
  const [filter, setFilter] = useState('')
  const [dialogProfile, setDialogProfile] = useState<ProfileInput | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [promptFor, setPromptFor] = useState<Profile | null>(null)

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
      if (!profile.hasPassword) {
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

  const menuItemsFor = useCallback(
    (node: TreeNode): MenuItem[] => {
      if (node.kind !== 'connection') return []

      const profileId = node.profileId
      const profile = profiles.find((p) => p.id === profileId)
      if (!profile) return []

      const connected = Boolean(sessions[profileId])

      return [
        connected
          ? { label: 'Disconnect', onSelect: () => void disconnect(profileId) }
          : { label: 'Connect', onSelect: () => startConnect(profile) },
        {
          label: 'Edit Connection…',
          onSelect: () => openEditConnection(profile),
        },
        {
          label: 'Delete Connection',
          danger: true,
          separatorBefore: true,
          onSelect: () => void deleteProfile(profileId),
        },
      ]
    },
    [profiles, sessions, disconnect, startConnect, openEditConnection, deleteProfile],
  )

  // Each level loads what the level below it will need: a PostgreSQL database its schemas,
  // and the Tables folder its table list.
  const revealNode = useCallback(
    (node: TreeNode) => {
      const ns = node.namespace
      if (!ns) return

      if (node.kind === 'database' && !ownsObjects(node)) {
        void loadSchemas(node.profileId, ns.database)
      }
      if (node.kind === 'folder') void loadTables(node.profileId, ns)
    },
    [loadSchemas, loadTables],
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

  const openTable = useCallback(
    (node: TreeNode) => {
      if (node.kind !== 'table' || !node.namespace) return
      openTab({
        kind: 'table',
        title: node.label,
        profileId: node.profileId,
        namespace: node.namespace,
        table: node.label,
      })
    },
    [openTab],
  )

  // Selecting reads nothing and opens nothing: expanding is the caret's job, and loading is
  // `revealNode`'s, which only an expand calls.
  const selectNode = useCallback(
    (node: TreeNode) => {
      setSelected(node)

      // The node that owns tables is the one a query tab can run against. A PostgreSQL
      // database is not that node — its schemas are — so selecting one names nothing.
      if (node.namespace && ownsObjects(node)) selectNamespace(node.profileId, node.namespace)
    },
    [selectNamespace],
  )

  const activateNode = useCallback(
    (node: TreeNode) => {
      if (isExpandable(node)) toggleTreeNode(node)
      else openTable(node)
    },
    [toggleTreeNode, openTable],
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
      rowLimit: DEFAULT_QUERY_ROWS,
    })
  }, [openTab, selected, sessions, activeNamespace])

  const status = useMemo<StatusInfo>(() => {
    const activeProfileId = Object.keys(sessions)[0]
    const session = activeProfileId ? sessions[activeProfileId] : undefined

    if (!session) {
      return {
        status: error ? 'error' : 'idle',
        target: error ? error.message : null,
        serverVersion: null,
        rowCount: null,
        elapsedMs: null,
      }
    }

    return {
      status: 'connected',
      target: session.target,
      serverVersion: session.serverVersion,
      rowCount: null,
      elapsedMs: null,
    }
  }, [sessions, error])

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <CommandBar onNewConnection={openNewConnection} onNewQuery={newQuery} />

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
        >
          <Sidebar
            roots={roots}
            expanded={expanded}
            selectedId={selected?.id ?? null}
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
