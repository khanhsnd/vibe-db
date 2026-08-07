import { useState } from 'react'
import { Loader2, Play } from 'lucide-react'

import { backups as api, fileSizeText, type BackupResult } from '@/bridge/backup'
import { namespaceLabel } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { TransferLog } from '@/features/transfer/TransferLog'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { ICON } from '@/shared/ui/menuIcons'
import { useBackupsStore, type RestoreTarget } from '@/stores/backups'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'

import { ObjectPicker, useObjectPicker } from './ObjectPicker'

const NO_TABLES: string[] = []

/**
 * Running one backup back into the namespace it came off.
 *
 * **One screen, and not a wizard.** A restore has exactly two answers — which file, which of its
 * tables — and the first was given by the row this was raised on, so there is nothing to page
 * through: the list is on the screen the confirmation is on, and Restore is under both.
 *
 * Which tables the file holds is **read off its own first line**, which the backup listing already
 * read: the choice costs no read of the file and no question of any server. A backup with nothing
 * on that line — one written by an older build, or dropped into the folder by hand — offers no
 * narrowing and restores whole, which is exactly what it did before there was a line to read.
 */
export function RestoreBackupDialog() {
  const target = useBackupsStore((s) => s.restoring)
  const close = useBackupsStore((s) => s.closeRestore)

  return (
    <Dialog
      open={target !== null}
      onOpenChange={(open) => {
        if (!open) close()
      }}
      title="Restore Backup"
      icon={ICON.restoreBackup}
      tone="danger"
      width={620}
      bodyClassName=""
      footer={null}
    >
      {/* Keyed on the file, so a dialog raised on a second backup starts on all of its tables
          rather than on the first one's selection and the first one's log. */}
      {target ? <RestoreForm key={keyOf(target)} target={target} onClose={close} /> : null}
    </Dialog>
  )
}

const keyOf = (target: RestoreTarget) =>
  `${namespaceKey(target.profileId, target.namespace)}/${target.name}`

function RestoreForm({ target, onClose }: { target: RestoreTarget; onClose: () => void }) {
  const { profileId, namespace, name } = target

  // The row is read out of the one list the tree and the tab both draw, rather than handed in when
  // the menu was clicked: a folder re-read while this is open is a folder this is looking at.
  const file = useConnectionsStore(
    (s) => s.backups[namespaceKey(profileId, namespace)]?.find((one) => one.name === name) ?? null,
  )
  const connected = useConnectionsStore((s) => s.sessions[profileId] !== undefined)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)

  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<BackupResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const tables = file?.tables ?? NO_TABLES
  const picked = useObjectPicker(tables)

  // What stops the button, said in one line rather than left to fail silently — `changeSchema` does
  // nothing at all on a connection that has been closed under it.
  const blocked = !connected
    ? 'This connection is closed. Reconnect it to restore.'
    : tables.length > 0 && picked.chosen.length === 0
      ? 'Nothing is selected. Pick at least one table to put back.'
      : ''

  const start = async () => {
    if (blocked !== '') return

    setRunning(true)
    setError(null)
    setResult(null)

    try {
      // A restore creates tables, so it is the *catalogue* that goes stale — the folder is
      // untouched. `changeSchema` is what re-reads it.
      await changeSchema(profileId, { namespace }, async (sessionId) => {
        const ran = await api.restore(sessionId, {
          namespace,
          name,
          tables: picked.wire,
          runId: `restore-${name}`,
        })
        // A restore that stopped part way is not an exception on the wire — the Go side answers
        // with the log and lets the caller read it — so what the server refused is *shown* rather
        // than raised. The reader sees every rejection, not the first one wrapped in a banner.
        setResult(ran)
      })
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setRunning(false)
    }
  }

  const finished = result !== null && !running
  const failed = finished && result.failed > 0

  return (
    <div className="flex flex-col">
      <div className="scroll-thin max-h-[60vh] overflow-y-auto px-4 py-3.5">
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <Fact label="Backup" value={name} />
            <Fact label="Into" value={namespaceLabel(namespace)} />
            {file ? <Fact label="Size" value={fileSizeText(file.size)} /> : null}
            {file?.comment ? <Fact label="Comment" value={file.comment} /> : null}
          </div>

          {/* Said before the list rather than after it: what the reader ticks changes how much of
              this happens, not whether it does. */}
          <p className="text-xs text-muted">
            Its statements are applied to the server as they are — tables it declares are created
            and its records are added to whatever is already there. It stops at the first statement
            the server refuses, and what has already run stays run.
          </p>

          {tables.length > 0 ? (
            <ObjectPicker
              tables={tables}
              excluded={picked.excluded}
              onToggle={picked.toggle}
              onAll={picked.all}
              onNone={picked.none}
            />
          ) : (
            <p className="text-xs text-faint">
              This file does not say which tables it holds, so all of it is restored. Backups taken
              by this app do say, and can be narrowed here.
            </p>
          )}

          {error ? <Banner tone="danger" text={error.message} /> : null}

          {running || result ? (
            <TransferLog
              messages={result?.messages ?? []}
              started={running ? 'Restoring…' : ''}
              finished={!finished ? '' : failed ? 'Failed' : `Finished in ${result.elapsedMs} ms`}
            />
          ) : null}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
        <span className="mr-auto text-xs text-muted">
          {blocked !== ''
            ? blocked
            : tables.length > 0
              ? `${picked.chosen.length} of ${tables.length} tables`
              : ''}
        </span>

        <Button onClick={onClose}>{finished && !failed ? 'Close' : 'Cancel'}</Button>
        <Button
          variant="danger"
          icon={
            running ? (
              <Loader2 size={12} className="animate-spin" />
            ) : (
              <Play size={12} fill="currentColor" />
            )
          }
          disabled={running || blocked !== ''}
          onClick={() => void start()}
        >
          Restore
        </Button>
      </div>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-2 text-sm">
      <span className="text-muted">{label}:</span>
      <span className="selectable truncate text-accent">{value}</span>
    </div>
  )
}
