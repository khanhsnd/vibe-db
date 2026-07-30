import { useCallback, useEffect, useMemo, useState } from 'react'

import { toProfileInput, type Profile, type ProfileInput } from '@/bridge/connections'
import { DEFAULT_QUERY_ROWS } from '@/bridge/query'
import { ConnectionDialog } from '@/features/connections/ConnectionDialog'
import { PasswordPrompt } from '@/features/connections/PasswordPrompt'
import type { TreeNode } from '@/features/connections/tree'
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
  const tables = useConnectionsStore((s) => s.tables)
  const activeDatabase = useConnectionsStore((s) => s.activeDatabase)
  const expanded = useConnectionsStore((s) => s.expanded)
  const error = useConnectionsStore((s) => s.error)
  const loadProfiles = useConnectionsStore((s) => s.loadProfiles)
  const connect = useConnectionsStore((s) => s.connect)
  const disconnect = useConnectionsStore((s) => s.disconnect)
  const deleteProfile = useConnectionsStore((s) => s.deleteProfile)
  const selectDatabase = useConnectionsStore((s) => s.selectDatabase)
  const loadTables = useConnectionsStore((s) => s.loadTables)
  const toggleNode = useConnectionsStore((s) => s.toggleNode)

  const roots = useMemo(
    () => buildTree({ profiles, statuses, databases, tables, activeDatabase }),
    [profiles, statuses, databases, tables, activeDatabase],
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
        { label: 'Edit Connection…', onSelect: () => openEditConnection(profile) },
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

  const toggleTreeNode = useCallback(
    (node: TreeNode) => {
      toggleNode(node.id)
      if (node.kind === 'folder' && node.database) void loadTables(node.profileId, node.database)
    },
    [toggleNode, loadTables],
  )

  const openTable = useCallback(
    (node: TreeNode) => {
      if (node.kind !== 'table' || !node.database) return
      openTab({
        kind: 'table',
        title: node.label,
        profileId: node.profileId,
        database: node.database,
        table: node.label,
      })
    },
    [openTab],
  )

  const selectNode = useCallback(
    (node: TreeNode) => {
      setSelected(node)
      if (!node.database) return

      if (node.kind === 'database') selectDatabase(node.profileId, node.database)
      if (node.kind === 'folder') {
        if (!expanded.has(node.id)) toggleNode(node.id)
        void loadTables(node.profileId, node.database)
      }
    },
    [expanded, selectDatabase, toggleNode, loadTables],
  )

  const activateNode = useCallback(
    (node: TreeNode) => {
      if (node.kind === 'connection') {
        const profile = profiles.find((p) => p.id === node.profileId)
        if (profile && !sessions[node.profileId]) startConnect(profile)
        else toggleNode(node.id)
        return
      }
      openTable(node)
    },
    [profiles, sessions, startConnect, toggleNode, openTable],
  )

  // A new editor inherits the selected connection and database, falling back to
  // whichever connection is open.
  const newQuery = useCallback(() => {
    const profileId = selected?.profileId ?? Object.keys(sessions)[0] ?? ''
    const database = selected?.database ?? activeDatabase[profileId] ?? ''
    const count = useTabsStore.getState().tabs.filter((tab) => tab.kind === 'query').length

    openTab({
      kind: 'query',
      title: `Query ${count + 1}`,
      profileId,
      database,
      sql: '',
      rowLimit: DEFAULT_QUERY_ROWS,
    })
  }, [openTab, selected, sessions, activeDatabase])

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
      <CommandBar onNewConnection={openNewConnection} />

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

      <ConnectionDialog
        open={dialogOpen}
        initial={dialogProfile}
        onOpenChange={setDialogOpen}
      />

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
