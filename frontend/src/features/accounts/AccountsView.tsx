import { useEffect, useState } from 'react'

import { accountLabel, applyAccountChanges, type AccountRef } from '@/bridge/accounts'
import { engineInfo } from '@/bridge/connections'
import { ResultGrid } from '@/features/results/ResultGrid'
import { focusOf, type GridData, type Selection } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore, type AccountsTab } from '@/stores/tabs'

import { dropChanges } from './accountDraft'
import { AccountEditor } from './AccountEditor'

/**
 * Who a connection knows, and the editor for whichever of them is picked — what the `Users` folder
 * opens. The list above is a `ResultGrid` in the columns the engine's own catalogue keeps, like
 * every other list in this app; the editor below is where an account is changed.
 *
 * There is **one of these per connection**, and picking a second account moves the editor rather
 * than opening a second tab: an account belongs to the server, and the server is what the tab is.
 */
export function AccountsView({ tab, sessionId }: { tab: AccountsTab; sessionId: string }) {
  const [selection, setSelection] = useState<Selection | null>(null)

  const list = useConnectionsStore((s) => s.accounts[tab.profileId])
  const loadAccounts = useConnectionsStore((s) => s.loadAccounts)
  const refreshAccounts = useConnectionsStore((s) => s.refreshAccounts)
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === tab.profileId)?.engine ?? '',
  )
  const info = engineInfo(engine)

  const ask = useSchemaStore((s) => s.ask)
  const setAccount = useTabsStore((s) => s.setAccount)

  useEffect(() => {
    void loadAccounts(tab.profileId)
  }, [loadAccounts, tab.profileId])

  // `creating` is a *new* account rather than one of the rows, and it is the tab's own state
  // because there is nothing on the server for the tab to be pointed at yet.
  const [creating, setCreating] = useState(false)
  const open = creating ? null : tab.account

  const refAt = (row: number): AccountRef | undefined => list?.refs[row]
  const focus = focusOf(selection)
  const picked = focus ? refAt(focus.row) : undefined

  const pick = (row: number) => {
    const ref = refAt(row)
    if (!ref) return
    setCreating(false)
    setAccount(tab.id, ref)
  }

  const remove = () => {
    if (!picked) return
    ask({
      kind: 'confirm',
      title: `Delete ${info.accountLabel}`,
      message:
        `Drop ${accountLabel(picked)}? Everything granted to it goes with it, and anything ` +
        `connecting as it stops being able to.`,
      submitLabel: 'Delete',
      danger: true,
      onSubmit: async () => {
        await applyAccountChanges(sessionId, dropChanges(picked))
        await refreshAccounts(tab.profileId)
        setAccount(tab.id, null)
        setSelection(null)
      },
    })
  }

  const data: GridData | null = list
    ? { columns: list.columns.map((column) => ({ ...column, readOnly: true })), rows: list.rows }
    : null

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button
          variant="subtle"
          icon={ICON.add}
          onClick={() => {
            setCreating(true)
            setAccount(tab.id, null)
          }}
        >
          New {info.accountLabel}
        </Button>
        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={!picked}
          onClick={remove}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete {info.accountLabel}
        </Button>
        <Button
          variant="subtle"
          icon={ICON.refresh}
          onClick={() => void refreshAccounts(tab.profileId)}
        >
          Refresh
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        <Split id="accounts" orientation="vertical" panelIds={['list', 'editor']} className="h-full">
          <SplitPanel id="list" defaultSize={40} minSize={15}>
            {data === null ? (
              <Loading label="Reading the account list…" />
            ) : data.rows.length === 0 ? (
              <Empty
                title={`No ${info.accountLabel.toLowerCase()}s to show`}
                hint={`Reading who a server knows needs a privilege of its own. New ${info.accountLabel} makes one.`}
              />
            ) : (
              <ResultGrid
                result={data}
                selection={selection}
                onSelectionChange={(next) => {
                  setSelection(next)
                  const at = focusOf(next)
                  if (at) pick(at.row)
                }}
                onActivateRow={pick}
              />
            )}
          </SplitPanel>

          <SplitHandle orientation="vertical" />

          <SplitPanel id="editor" defaultSize={60} minSize={20}>
            {open === null && !creating ? (
              <Empty
                title={`No ${info.accountLabel.toLowerCase()} picked`}
                hint="Pick one above to read what it may do, or make a new one."
              />
            ) : (
              <AccountEditor
                // A new account and an existing one are two editors, not one with its fields
                // cleared: the key is what makes a Save mean create rather than alter.
                key={open ? `${open.name} ${open.host} ${open.database}` : 'new'}
                sessionId={sessionId}
                profileId={tab.profileId}
                account={open}
                onSaved={(ref) => {
                  setCreating(false)
                  setAccount(tab.id, ref)
                }}
              />
            )}
          </SplitPanel>
        </Split>
      </div>
    </div>
  )
}
