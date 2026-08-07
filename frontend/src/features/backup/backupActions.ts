import { backups } from '@/bridge/backup'
import { namespaceLabel, type Namespace } from '@/bridge/connections'
import { AppError } from '@/bridge/errors'
import type { ChangeSchema, SchemaTarget } from '@/stores/connections'
import type { SchemaRequest } from '@/stores/schema'

export interface BackupDeps {
  ask: (request: SchemaRequest) => void
  /** Runs something against the backup folder and re-reads it. Throws, for a dialog to catch. */
  changeBackups: (profileId: string, ns: Namespace, change: ChangeSchema) => Promise<void>
  /**
   * Runs something that changed the *server* and re-reads the catalogue. A restore is one: it
   * creates tables, and the tree beside it would otherwise still be showing the ones from before.
   */
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
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
    restore: () =>
      deps.ask({
        kind: 'confirm',
        title: 'Restore Backup',
        message:
          `Run ${name} back into ${namespaceLabel(namespace)}? Its statements are applied to the ` +
          `server as they are — tables it declares are created and its records are added to ` +
          `whatever is already there. It stops at the first statement the server refuses, and ` +
          `what has already run stays run.`,
        submitLabel: 'Restore',
        danger: true,
        // The *catalogue* is what a restore made stale, not the folder — see `changeSchema`.
        onSubmit: () =>
          deps.changeSchema(profileId, { namespace }, async (sessionId) => {
            const ran = await backups.restore(sessionId, {
              namespace,
              name,
              runId: `restore-${name}`,
            })
            // A restore that stopped part way is not an exception on the wire — the Go side answers
            // with the log and lets the caller read it — so the first refusal is raised here, which
            // is what keeps the confirmation open with the server's own wording on it.
            if (ran.failed > 0) {
              const first = ran.messages.find((message) => message.error)
              throw new AppError('SQL_ERROR', first?.text ?? 'The restore did not finish.')
            }
          }),
      }),

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
