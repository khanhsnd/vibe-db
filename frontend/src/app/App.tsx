import { useCallback, useEffect, useMemo, useState } from 'react'

import { toProfileInput, type Profile, type ProfileInput } from '@/bridge/connections'
import { ConnectionDialog } from '@/features/connections/ConnectionDialog'
import { PasswordPrompt } from '@/features/connections/PasswordPrompt'
import type { TreeNode } from '@/features/connections/tree'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { Split, SplitHandle, SplitPanel, usePanelRef } from '@/shared/ui/Split'
import { buildTree, profileIdFromNode, useConnectionsStore } from '@/stores/connections'
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
  const expanded = useConnectionsStore((s) => s.expanded)
  const error = useConnectionsStore((s) => s.error)
  const loadProfiles = useConnectionsStore((s) => s.loadProfiles)
  const connect = useConnectionsStore((s) => s.connect)
  const disconnect = useConnectionsStore((s) => s.disconnect)
  const deleteProfile = useConnectionsStore((s) => s.deleteProfile)
  const toggleNode = useConnectionsStore((s) => s.toggleNode)

  const roots = useMemo(
    () => buildTree(profiles, statuses, databases),
    [profiles, statuses, databases],
  )

  const [selectedId, setSelectedId] = useState<string | null>(null)
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
      const profileId = profileIdFromNode(node.id)
      if (!profileId) return []

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

  const activateNode = useCallback(
    (node: TreeNode) => {
      const profileId = profileIdFromNode(node.id)
      if (profileId) {
        const profile = profiles.find((p) => p.id === profileId)
        if (profile && !sessions[profileId]) startConnect(profile)
        else toggleNode(node.id)
        return
      }
      if (node.children === undefined) {
        openTab({ kind: 'table', title: node.label, sessionId: null })
      } else {
        toggleNode(node.id)
      }
    },
    [profiles, sessions, startConnect, toggleNode, openTab],
  )

  const newQuery = useCallback(() => {
    openTab({ kind: 'query', title: 'Query', sessionId: null })
  }, [openTab])

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
            selectedId={selectedId}
            filter={filter}
            onFilterChange={setFilter}
            onToggle={toggleNode}
            onSelect={(node) => setSelectedId(node.id)}
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
