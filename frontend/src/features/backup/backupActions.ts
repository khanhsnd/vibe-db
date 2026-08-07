import { backups } from '@/bridge/backup'
import type { Namespace } from '@/bridge/connections'
import type { RestoreTarget } from '@/stores/backups'
import type { ChangeSchema } from '@/stores/connections'
import type { SchemaRequest } from '@/stores/schema'

export interface BackupDeps {
  ask: (request: SchemaRequest) => void
  /**
   * Raises the restore dialog. A restore is the one action here with a *form* behind it — which of
   * the backup's tables to put back — so it is not a `SchemaRequest`, which holds none.
   */
  askRestore: (target: RestoreTarget) => void
  /** Runs something against the backup folder and re-reads it. Throws, for a dialog to catch. */
  changeBackups: (profileId: string, ns: Namespace, change: ChangeSchema) => Promise<void>
  /** Where a failure goes when nothing is waiting on it — the two items that raise no dialog. */
  report: (thrown: unknown) => void
}

export interface BackupActions {
  restore: () => void
  extractSQL: () => void
  copy: () => void
  rename: () => void
  remove: () => void
}

/**
 * What can be done to a backup that already exists, wherever one is offered — the tree's own menu
 * and the toolbar over the list both build from here, so the wording of a confirmation and the
 * order of a change cannot differ between the two.
 *
 * Everything that cannot be undone is confirmed or filled in first, and whatever asked is where the
 * rejection is shown. Only **Restore** touches a server; the other four are a file on this machine
 * being copied, moved, written out or deleted.
 */
export function backupActions(
  profileId: string,
  namespace: Namespace,
  name: string,
  deps: BackupDeps,
): BackupActions {
  const change = (run: ChangeSchema) => () => deps.changeBackups(profileId, namespace, run)

  // The two items with no dialog behind them. They ask nothing first because nothing is lost
  // either way — one writes a file the reader named, the other makes a second file beside the
  // first — and so there is nowhere for a failure to land but wherever the call site puts it.
  const quietly = (run: ChangeSchema) => () => {
    void deps.changeBackups(profileId, namespace, run).catch(deps.report)
  }

  return {
    // The only one that is a dialog of its own rather than a confirmation: what it asks — which of
    // the file's tables to put back — is a list, and the warning is on the same screen as the list.
    restore: () => deps.askRestore({ profileId, namespace, name }),

    extractSQL: quietly(async (sessionId) => {
      await backups.extractSQL(sessionId, namespace, name)
    }),

    copy: quietly(async (sessionId) => {
      await backups.copy(sessionId, namespace, name)
    }),

    rename: () =>
      deps.ask({
        kind: 'prompt',
        title: 'Rename Backup',
        label: 'New name',
        value: name,
        submitLabel: 'Rename',
        onSubmit: (to) => change((sessionId) => backups.rename(sessionId, namespace, name, to))(),
      }),

    remove: () =>
      deps.ask({
        kind: 'confirm',
        title: 'Delete Backup',
        message: `Delete ${name}? The file goes, and nothing brings it back.`,
        submitLabel: 'Delete',
        danger: true,
        onSubmit: change((sessionId) => backups.remove(sessionId, namespace, name)),
      }),
  }
}
