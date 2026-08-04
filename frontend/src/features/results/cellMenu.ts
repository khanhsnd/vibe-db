import { copyText } from '@/bridge/clipboard'
import { EXPORT_FORMATS, type ExportFormat } from '@/bridge/export'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

import { COPY_FORMATS, copyAsText } from './copyAs'
import {
  arityOf,
  FILTER_OPS,
  NO_FILTER,
  operatorLabel,
  type Condition,
  type FilterOp,
  type GridFiltering,
} from './filter'
import {
  cellText,
  selectedCells,
  selectedRows,
  toTSV,
  type Cell,
  type CellRef,
  type GridData,
  type Selection,
  type SortKey,
} from './resultSet'

/** The UUIDs a cell can be filled with. One is minted per cell, so a range gets a fresh one each. */
const UUIDS: ReadonlyArray<{ label: string; make: () => string }> = [
  { label: 'Lowercase', make: () => crypto.randomUUID() },
  { label: 'Uppercase', make: () => crypto.randomUUID().toUpperCase() },
  { label: 'Without Hyphens', make: () => crypto.randomUUID().replaceAll('-', '') },
]

/**
 * Everything the cell menu acts through. What the grid was given decides what the menu offers:
 * a read-only grid has no `onEditCell` and therefore no writes, a surface without a cell editor
 * no `onEditInPanel`, and a surface that cannot narrow its rows no `filtering`. An absent handler
 * leaves its item off rather than showing it dead, because an item that can never run is noise.
 */
export interface CellMenuContext {
  /** The cell the menu was raised on, which is not always the selection's focus. */
  at: CellRef
  data: GridData
  selection: Selection
  sort: readonly SortKey[]
  onEditCell?: ((ref: CellRef, value: Cell) => void) | undefined
  onEditInPanel?: ((ref: CellRef) => void) | undefined
  onDeleteRows?: ((rows: number[]) => void) | undefined
  onSortChange?: ((sort: SortKey[]) => void) | undefined
  onExport?: ((format: ExportFormat) => void) | undefined
  onRefresh?: (() => void) | undefined
  filtering?: GridFiltering | undefined
  /** The grid's own paste, so the menu writes a block exactly as `Ctrl`+`V` does. */
  onPaste: (at: CellRef) => void
}

/** The cell the menu names: its column, and the value the filter items compare against. */
interface Target {
  column: string
  value: Cell
}

/**
 * The menu a right-click on a cell raises. It is built here rather than in `ResultGrid` because it
 * is the longest menu in the app and the grid is already the largest component; every item still
 * acts through a prop the grid was handed, so nothing here knows a call site.
 *
 * The writes act on the **selection**, not on the one cell: right-clicking outside a selection has
 * already replaced it with the cell under the pointer, so one cell is the ordinary case and a
 * range is the deliberate one.
 */
export function cellMenuItems(context: CellMenuContext): MenuItem[] {
  const column = context.data.columns[context.at.column]
  if (!column) return []

  // A grid with no rows has no cell to act on, so the one thing that still means something is
  // reading it again — and every other item would name row zero, which is not there.
  if (context.data.rows.length === 0) {
    return context.onRefresh
      ? [{ label: 'Refresh', icon: ICON.refresh, onSelect: context.onRefresh }]
      : []
  }

  const target: Target = {
    column: column.name,
    value: context.data.rows[context.at.row]?.[context.at.column] ?? null,
  }

  return [
    ...writeItems(context),
    ...deleteItems(context),
    ...clipboardItems(context, target),
    ...narrowItems(context, target),
    ...refreshItem(context),
  ]
}

function writeItems(context: CellMenuContext): MenuItem[] {
  const { at, data, selection, onEditCell, onEditInPanel } = context

  const cells = selectedCells(selection, data.rows.length, data.columns.length)
  const write = (make: () => Cell) => () => {
    for (const ref of cells) onEditCell?.(ref, make())
  }

  return [
    ...(onEditCell
      ? [
          { label: 'Set to Empty String', icon: ICON.emptyString, onSelect: write(() => '') },
          { label: 'Set to NULL', icon: ICON.nullValue, onSelect: write(() => null) },
          {
            label: 'Generate UUID',
            icon: ICON.uuid,
            items: UUIDS.map((uuid) => ({ label: uuid.label, onSelect: write(uuid.make) })),
          },
        ]
      : []),
    ...(onEditInPanel
      ? [{ label: 'Edit in Cell Editor', icon: ICON.cellEditor, onSelect: () => onEditInPanel(at) }]
      : []),
  ]
}

function deleteItems({ selection, onDeleteRows }: CellMenuContext): MenuItem[] {
  if (!onDeleteRows) return []

  const rows = selectedRows(selection)

  return [
    {
      label: rows.length > 1 ? `Delete ${rows.length} Records` : 'Delete Record',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: () => onDeleteRows(rows),
    },
  ]
}

function clipboardItems(context: CellMenuContext, target: Target): MenuItem[] {
  const { at, data, selection, onEditCell, onExport, onPaste } = context

  return [
    {
      label: 'Copy',
      icon: ICON.copy,
      separatorBefore: true,
      onSelect: () => void copyText(toTSV(selection, data)),
    },
    { label: 'Copy Field Name', icon: ICON.copy, onSelect: () => void copyText(target.column) },
    {
      label: 'Copy As',
      icon: ICON.copy,
      items: COPY_FORMATS.map((format) => ({
        label: format.label,
        onSelect: () => void copyText(copyAsText(format.value, selection, data)),
      })),
    },
    ...(onEditCell ? [{ label: 'Paste', icon: ICON.paste, onSelect: () => onPaste(at) }] : []),
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
  ]
}

/** Filter and sort, which narrow what the grid shows rather than change what it holds. */
function narrowItems(context: CellMenuContext, target: Target): MenuItem[] {
  const { sort, onSortChange, filtering } = context

  const sorting: MenuItem[] = onSortChange
    ? [
        {
          label: 'Sort',
          icon: ICON.sortAscending,
          items: [
            {
              label: 'Ascending',
              icon: ICON.sortAscending,
              onSelect: () => onSortChange([{ column: target.column, descending: false }]),
            },
            {
              label: 'Descending',
              icon: ICON.sortDescending,
              onSelect: () => onSortChange([{ column: target.column, descending: true }]),
            },
            {
              label: 'Remove Sort',
              icon: ICON.sortNone,
              disabled: sort.length === 0,
              onSelect: () => onSortChange([]),
            },
          ],
        },
      ]
    : []

  if (!filtering) return sorting.map(afterSeparator)

  const root = filtering.criteria

  return [
    afterSeparator({
      label: filtering.shown ? 'Hide Filter & Sort' : 'Show Filter & Sort',
      icon: ICON.filterPanel,
      onSelect: () => filtering.onShownChange(!filtering.shown),
    }),
    {
      label: 'Filter',
      icon: ICON.filter,
      // A condition picked here lands at the end of the root, joined by whatever the root joins
      // by, and applies at once — it was picked whole, so there is nothing left to fill in.
      items: FILTER_OPS.map((one) => ({
        label: operatorLabel(one.operator, target.value),
        onSelect: () =>
          filtering.onChange({
            ...root,
            children: [...root.children, conditionOn(one.operator, target)],
          }),
      })),
    },
    ...sorting,
    {
      label: 'Remove All Filters & Sorts',
      icon: ICON.filterNone,
      disabled: root.children.length === 0 && sort.length === 0,
      onSelect: () => {
        filtering.onChange(NO_FILTER)
        onSortChange?.([])
      },
    },
  ]
}

function refreshItem({ onRefresh }: CellMenuContext): MenuItem[] {
  if (!onRefresh) return []

  return [{ label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: onRefresh }]
}

/**
 * A condition read off one cell. Only an operator taking a single value can be: a NULL cell
 * compares against nothing — `(Null)` is the grid's word for it, not a value — and a range or a
 * list cannot be read off one cell at all, so both open in the builder with the value unset.
 */
function conditionOn(operator: FilterOp, target: Target): Condition {
  const one = arityOf(operator) === 1 && target.value !== null

  return {
    kind: 'condition',
    column: target.column,
    operator,
    values: one ? [cellText(target.value)] : [],
    enabled: true,
  }
}

const afterSeparator = (item: MenuItem): MenuItem => ({ ...item, separatorBefore: true })
