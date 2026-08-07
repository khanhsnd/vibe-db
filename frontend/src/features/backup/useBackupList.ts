import { useEffect, useMemo } from 'react'

import { fileSizeText, type Backup } from '@/bridge/backup'
import type { Namespace } from '@/bridge/connections'
import type { GridData, ResultColumn } from '@/features/results/resultSet'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'

/**
 * The columns the list draws, which are the app's own rather than an engine's: a backup is a file
 * on this machine and no catalogue has ever heard of one. They are the four the reader looks for,
 * in the order they look — which one, when, how big, and what they wrote about it.
 */
const COLUMNS: ResultColumn[] = [
  { name: 'Name', type: 'text', fullType: 'text', width: 190 },
  { name: 'Modified Date', type: 'text', fullType: 'text', width: 170 },
  { name: 'File Size', type: 'text', fullType: 'text', width: 110 },
  { name: 'Comment', type: 'text', fullType: 'text', width: 320 },
]

/**
 * Every backup kept for one namespace, as the grid draws it.
 *
 * The files come from `stores/connections`, which is **the one place they live** — the tree draws
 * the same folder under its Backups node, and a second copy read separately here would be a second
 * list to keep in step. Anything that changes the folder goes through `changeBackups`, so the tab
 * and the branch move together.
 */
export function useBackupList(profileId: string, namespace: Namespace) {
  const key = namespaceKey(profileId, namespace)

  const files = useConnectionsStore((s) => s.backups[key] ?? null)
  const refreshBackups = useConnectionsStore((s) => s.refreshBackups)

  // Re-read rather than cached-if-present, which is what the *tree* does with the same folder:
  // only the tab is worth a read on every visit, because what is in the folder can change without
  // this app doing it — another window of it, a shared drive, a file dropped in by hand — and a
  // directory listing on this machine is cheap enough to pay for on a tab switch.
  useEffect(() => {
    void refreshBackups(profileId, namespace)
  }, [refreshBackups, profileId, namespace])

  // The times and sizes are turned into words here rather than on the Go side, because how either
  // reads is the reader's own locale and only this side knows it.
  const data = useMemo<GridData | null>(() => rowsOf(files), [files])

  return {
    files,
    data,
    refresh: () => void refreshBackups(profileId, namespace),
  }
}

function rowsOf(files: Backup[] | null): GridData | null {
  if (files === null) return null

  return {
    columns: COLUMNS,
    rows: files.map((file) => [
      file.name,
      dateText(file.modifiedAt),
      fileSizeText(file.size),
      file.comment,
    ]),
  }
}

/** `2026-04-29 11:03:21` — sortable, unambiguous, and the same width on every row. */
function dateText(iso: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return iso

  const pad = (value: number) => String(value).padStart(2, '0')
  return (
    `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ` +
    `${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}`
  )
}
