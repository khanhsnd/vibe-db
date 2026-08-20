import { copyText } from '@/bridge/clipboard'
import type { ObjectRow, ObjectRules } from '@/bridge/objects'
import type { CellMenuContext } from '@/features/results/cellMenu'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

/** What only this surface can do, and what no other grid's menu has to offer. */
export interface ObjectActions {
  onOpen: (row: ObjectRow) => void
  onDownload: () => void
  onUpload: () => void
  onNewFolder: () => void
  onDelete: () => void
  onRename: (row: ObjectRow) => void
  onCopyLink: (row: ObjectRow) => void
  onDetails: (row: ObjectRow) => void
}

/**
 * The menu a right-click raises over a bucket. It is an **override**, like [a key
 * space's](../table/keyMenu.ts), and for a stronger reason: the built-in menu is written for rows
 * that are a table's, and nearly none of it means anything here.
 *
 * There is no NULL to set on an object, no UUID worth minting into one, no condition to add — the
 * bar above the grid is where a bucket is searched — and no sort worth offering over a page cut from
 * a service's own order. What is left is what a reader actually does to an object: bring it down,
 * put one up, look at it, copy a link to it, move it, delete it.
 *
 * It acts through the same `CellMenuContext` the built-in menu is handed, so an override here is a
 * different menu and never a second set of handlers. What it adds beyond that context is
 * `ObjectActions`, and every one of those is the **same call the toolbar makes** — a right-click and
 * a button cannot come to mean different things.
 */
export function objectMenuItems(
  context: CellMenuContext,
  rows: readonly ObjectRow[],
  picked: readonly ObjectRow[],
  rules: ObjectRules,
  on: ObjectActions,
): MenuItem[] {
  const { at, data, onRefresh } = context
  const refresh: MenuItem[] = onRefresh
    ? [{ label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: onRefresh }]
    : []

  // An empty folder has no row to act on, so what is left is what can still be done *to the folder*:
  // put something in it, make one inside it, and read it again.
  if (data.rows.length === 0) {
    return [
      ...(rules.upload.refused
        ? []
        : [
            { label: 'Upload Files…', icon: ICON.upload, writes: true, onSelect: on.onUpload },
            { label: 'New Folder…', icon: ICON.newFolder, writes: true, onSelect: on.onNewFolder },
          ]),
      ...refresh,
    ]
  }

  const row = rows[at.row]
  if (!row) return refresh

  const many = picked.length > 1
  const folders = picked.some((one) => one.folder)

  return [
    // First, because it is what a double-click does: a folder is walked into and an object is opened
    // in the panel below. The menu says which, so the two never read as one thing.
    row.folder
      ? { label: 'Open Folder', icon: ICON.openFolder, onSelect: () => on.onOpen(row) }
      : { label: 'Show Details', icon: ICON.cellEditor, onSelect: () => on.onDetails(row) },

    {
      label: many ? `Download ${picked.length} Items` : row.folder ? 'Download Folder…' : 'Download…',
      icon: ICON.download,
      separatorBefore: true,
      onSelect: on.onDownload,
    },
    ...(rules.upload.refused
      ? []
      : [
          { label: 'Upload Files…', icon: ICON.upload, writes: true, onSelect: on.onUpload },
          { label: 'New Folder…', icon: ICON.newFolder, writes: true, onSelect: on.onNewFolder },
        ]),

    // A link is offered for one object alone: fifteen links on a clipboard is not a thing anybody
    // pastes, and a folder has nothing to sign.
    ...(!row.folder && !many
      ? [
          {
            label: 'Copy Link',
            icon: ICON.link,
            separatorBefore: true,
            onSelect: () => on.onCopyLink(row),
          },
        ]
      : []),

    {
      label: 'Copy Key',
      icon: ICON.copy,
      ...(row.folder || many ? {} : {}),
      separatorBefore: row.folder || many,
      onSelect: () => void copyText(picked.map((one) => one.key).join('\n')),
    },
    { label: 'Copy Name', icon: ICON.copy, onSelect: () => void copyText(row.name) },

    // A rename is a copy and then a delete, so it is offered where both are allowed and on one
    // object at a time: renaming a folder is copying everything under it, which is a job and not a
    // menu item.
    ...(!row.folder && !many && !rules.move.refused
      ? [
          {
            label: 'Rename…',
            icon: ICON.rename,
            separatorBefore: true,
            writes: true,
            onSelect: () => on.onRename(row),
          },
        ]
      : []),

    ...(rules.delete.refused
      ? []
      : [
          {
            label: deleteLabel(picked.length, folders),
            icon: ICON.delete,
            danger: true,
            separatorBefore: true,
            writes: true,
            onSelect: on.onDelete,
          },
        ]),

    ...refresh,
  ]
}

/**
 * What a delete is called, and it names the **folder** wherever one is in the pick: a folder stands
 * for everything under it, so a menu item that said only `Delete 2 Items` would be understating what
 * the next press does.
 */
function deleteLabel(count: number, folders: boolean): string {
  if (count > 1) return folders ? `Delete ${count} Items and Their Contents` : `Delete ${count} Objects`
  return folders ? 'Delete Folder and Everything In It…' : 'Delete Object'
}
