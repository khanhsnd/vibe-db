import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'

/**
 * Backups: writing one, listing them, running one back, and the two things a reader does to the
 * file itself.
 *
 * It is not `transfer.ts` and it is not the grid's `export.ts`. Those carry a table *out* — to a
 * file the reader names, in a format something else reads. A backup goes to a folder this app owns
 * and is found again by being there, and what is in it is the same statements a dump writes, so
 * restoring one is running a script.
 */

export type Backup = app.Backup
export type BackupResult = app.BackupResult

/** What a new backup covers and what it holds. An empty `tables` is every table the namespace has. */
export interface BackupRequest {
  namespace: Namespace
  tables: string[]
  withStructure: boolean
  withData: boolean
  comment: string
}

export interface RestoreRequest {
  namespace: Namespace
  name: string
  runId: string
}

/** What the New Backup dialog starts on: everything, structure and records both. */
export function defaultBackupRequest(namespace: Namespace): BackupRequest {
  return { namespace, tables: [], withStructure: true, withData: true, comment: '' }
}

const wire = <T>(value: unknown) => value as T

export const backups = {
  /** Every backup kept for this namespace, newest first. */
  list: (sessionId: string, namespace: Namespace): Promise<Backup[]> =>
    call(() => api.ListBackups(sessionId, wire<app.Namespace>(namespace))),

  run: (sessionId: string, request: BackupRequest): Promise<BackupResult> =>
    call(() =>
      api.RunBackup(
        sessionId,
        wire<app.BackupInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),

  restore: (sessionId: string, request: RestoreRequest): Promise<BackupResult> =>
    call(() =>
      api.RestoreBackup(
        sessionId,
        wire<app.RestoreInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),

  remove: (sessionId: string, namespace: Namespace, name: string): Promise<void> =>
    call(() => api.DeleteBackup(sessionId, wire<app.Namespace>(namespace), name)),

  /** Duplicates one beside itself, answering with what the copy is called. */
  copy: (sessionId: string, namespace: Namespace, name: string): Promise<string> =>
    call(() => api.CopyBackup(sessionId, wire<app.Namespace>(namespace), name)),

  rename: (sessionId: string, namespace: Namespace, name: string, to: string): Promise<void> =>
    call(() => api.RenameBackup(sessionId, wire<app.Namespace>(namespace), name, to)),

  /** Answers with the path written, or `''` for a dialog the reader dismissed. */
  extractSQL: (sessionId: string, namespace: Namespace, name: string): Promise<string> =>
    call(() => api.ExtractBackupSQL(sessionId, wire<app.Namespace>(namespace), name)),

  openFolder: (sessionId: string, namespace: Namespace): Promise<void> =>
    call(() => api.OpenBackupFolder(sessionId, wire<app.Namespace>(namespace))),
}

/**
 * How big a backup reads. Binary units, because that is what a file manager on any of the three
 * desktops this runs on shows for the same file — a size that disagreed with the one beside it in
 * Explorer would be the reader's problem rather than a rounding choice.
 */
export function fileSizeText(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`

  const units = ['KB', 'MB', 'GB', 'TB']
  let size = bytes / 1024
  let at = 0
  while (size >= 1024 && at < units.length - 1) {
    size /= 1024
    at += 1
  }
  return `${size < 10 ? size.toFixed(1) : Math.round(size)} ${units[at]}`
}
