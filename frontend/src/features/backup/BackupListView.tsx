import { useState } from 'react'
import { Archive } from 'lucide-react'

import { backups as api } from '@/bridge/backup'
import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import { focusOf, type Selection } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { useBackupsStore } from '@/stores/backups'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'

import { backupActions } from './backupActions'
import { useBackupList } from './useBackupList'

interface BackupListViewProps {
  sessionId: string
  profileId: string
  namespace: Namespace
}

/**
 * Every backup kept for a namespace — the pane the tree's Backups node opens beside the branch it
 * expands. It is a `ResultGrid`, read-only, and the toolbar over it acts on the backup the
 * selection names.
 *
 * Every button here is the same action the tree's own right-click offers, built from the same
 * `backupActions`: what a confirmation says and what a change re-reads cannot differ between the
 * two surfaces, because there is only one of each.
 */
export function BackupListView({ sessionId, profileId, namespace }: BackupListViewProps) {
  const { files, data, refresh } = useBackupList(profileId, namespace)
  const [selection, setSelection] = useState<Selection | null>(null)
  const [failure, setFailure] = useState<AppError | null>(null)

  const ask = useSchemaStore((s) => s.ask)
  const askBackup = useBackupsStore((s) => s.ask)
  const askRestore = useBackupsStore((s) => s.askRestore)
  const changeBackups = useConnectionsStore((s) => s.changeBackups)

  const focus = focusOf(selection)
  const picked = focus ? (files?.[focus.row] ?? null) : null

  // A failure with no dialog behind it lands in the strip under the grid rather than in the status
  // bar, which is where the reader is looking: the list itself stays drawn, which is what they need
  // to try the next one.
  const on = backupActions(profileId, namespace, picked?.name ?? '', {
    ask,
    askRestore,
    changeBackups,
    report: (thrown) => setFailure(toAppError(thrown)),
  })

  const act = (run: () => void) => () => {
    setFailure(null)
    run()
  }

  const openFolder = act(() => {
    void api.openFolder(sessionId, namespace).catch((thrown: unknown) => {
      setFailure(toAppError(thrown))
    })
  })

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button
          variant="subtle"
          icon={ICON.restoreBackup}
          disabled={picked === null}
          onClick={act(on.restore)}
        >
          Restore Backup
        </Button>
        <Button
          variant="subtle"
          icon={ICON.add}
          onClick={() => askBackup({ profileId, namespace })}
        >
          New Backup
        </Button>
        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={picked === null}
          onClick={act(on.remove)}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete Backup
        </Button>
        <Button
          variant="subtle"
          icon={ICON.dump}
          disabled={picked === null}
          onClick={act(on.extractSQL)}
        >
          Extract SQL
        </Button>

        <span className="mx-1 h-4 w-px bg-line" />

        <Button
          variant="subtle"
          icon={ICON.duplicate}
          disabled={picked === null}
          onClick={act(on.copy)}
        >
          Copy
        </Button>
        <Button
          variant="subtle"
          icon={ICON.rename}
          disabled={picked === null}
          title="Rename Backup (F2)"
          onClick={act(on.rename)}
        >
          Rename
        </Button>
        <Button variant="subtle" icon={ICON.openFolder} onClick={openFolder}>
          Open Containing Folder
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        {data === null ? (
          <Loading label="Reading the backup folder…" />
        ) : data.rows.length === 0 ? (
          <Empty
            icon={<Archive size={26} strokeWidth={1.5} />}
            title="No backups yet"
            hint="New Backup writes one into this namespace's own folder."
          />
        ) : (
          <ResultGrid
            result={data}
            selection={selection}
            onSelectionChange={setSelection}
            onRenameRow={() => {
              if (picked) act(on.rename)()
            }}
            onRefresh={refresh}
          />
        )}
      </div>

      {failure !== null ? <ErrorStrip error={failure} /> : null}

      <GridFooter editable={false} busy={false} onRefresh={refresh}>
        <span>{files === null ? '' : `${files.length} backups`}</span>
      </GridFooter>
    </div>
  )
}
