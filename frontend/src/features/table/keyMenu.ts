import { copyText } from '@/bridge/clipboard'
import { EXPORT_FORMATS } from '@/bridge/export'
import type { CellMenuContext } from '@/features/results/cellMenu'
import { COPY_FORMATS, copyAsText } from '@/features/results/copyAs'
import { selectedRows, toTSV } from '@/features/results/resultSet'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

/** What only the surface can do, and what no other grid's menu has to offer. */
interface KeyActions {
  /** Appends a draft key, which is the footer's own Add reached without travelling to the bar. */
  onAddKey: () => void
}

/**
 * The menu a right-click raises over a key space, which is a **shorter** menu than every other grid's
 * and deliberately so. The long one is built for a table store and nearly none of it means anything
 * here: there is no NULL to set on a key that would then not exist, no UUID worth minting into one,
 * no condition to add — the bar above the grid is where a key space is narrowed — and no sort worth
 * offering over a page cut from a sorted scan. What is left is what a reader actually does to a key,
 * which is edit it, add one, delete it, read it, copy it and read the page again.
 *
 * It acts through the same `CellMenuContext` the built-in menu is handed, so an override here is a
 * shorter menu and never a second set of handlers.
 */
export function keyMenuItems(context: CellMenuContext, on: KeyActions): MenuItem[] {
  const { at, data, selection, onEditInPanel, onDeleteRows, onExport, onRefresh } = context

  // A key space with no keys has no key to act on, so the one thing left that means anything is
  // reading it again — the rule the built-in menu already follows for an empty grid.
  if (data.rows.length === 0) {
    return [
      { label: 'Add Key', icon: ICON.add, onSelect: on.onAddKey },
      ...(onRefresh
        ? [{ label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: onRefresh }]
        : []),
    ]
  }

  const rows = selectedRows(selection)
  const column = data.columns[at.column]

  return [
    // First, because the panel below is always open on a key space: what Edit Key does is put this
    // key in it, which is the same call the SQL surfaces' **Edit in Cell Editor** makes.
    ...(onEditInPanel
      ? [{ label: 'Edit Key', icon: ICON.cellEditor, onSelect: () => onEditInPanel(at) }]
      : []),
    { label: 'Add Key', icon: ICON.add, onSelect: on.onAddKey },
    ...(onDeleteRows
      ? [
          {
            label: rows.length > 1 ? `Delete ${rows.length} Keys` : 'Delete Key',
            icon: ICON.delete,
            danger: true,
            onSelect: () => onDeleteRows(rows),
          },
        ]
      : []),

    {
      label: 'View Value',
      icon: ICON.viewValue,
      separatorBefore: true,
      onSelect: () => context.onViewValue(at),
    },
    {
      label: 'Copy',
      icon: ICON.copy,
      onSelect: () => void copyText(toTSV(selection, data)),
    },
    ...(column
      ? [
          {
            label: 'Copy Field Name',
            icon: ICON.copy,
            onSelect: () => void copyText(column.name),
          },
        ]
      : []),
    {
      label: 'Copy As',
      icon: ICON.copy,
      items: COPY_FORMATS.map((format) => ({
        label: format.label,
        onSelect: () => void copyText(copyAsText(format.value, selection, data)),
      })),
    },
    ...(onExport
      ? [
          {
            label: 'Save Data As',
            icon: ICON.saveAs,
            items: EXPORT_FORMATS.map((format) => ({
              label: format.label,
              onSelect: () => onExport(format.value),
            })),
          },
        ]
      : []),

    ...(onRefresh
      ? [{ label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: onRefresh }]
      : []),
  ]
}
