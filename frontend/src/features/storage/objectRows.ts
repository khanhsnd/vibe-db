import type { ObjectRow } from '@/bridge/objects'
import { keyRows, type KeyRow } from '@/features/table/keyTree'
import type { Cell, GridData, ResultColumn } from '@/features/results/resultSet'

/**
 * A bucket's page as the grid draws it. The columns are **not read from anywhere and not inferred
 * from anything**: they are the six facts an S3 listing carries for every key, on every service
 * there is, which is what a bucket has instead of a catalogue — the arrangement a
 * [key space](../../../../docs/features/key-space.md) has with its five.
 */
export const NAME_COLUMN = 'Name'
export const SIZE_COLUMN = 'Size'
export const KIND_COLUMN = 'Kind'
export const MODIFIED_COLUMN = 'Modified'
export const CLASS_COLUMN = 'Storage Class'
export const ETAG_COLUMN = 'ETag'

/** The one character a key is read as a path by — the service's own, and not a setting. */
export const OBJECT_SEPARATOR = '/'

/** How wide the name column opens at. It is the widest thing in the grid and the reason for a page. */
const NAME_WIDTH = 320

const COLUMNS: ResultColumn[] = [
  { name: NAME_COLUMN, type: 'varchar', fullType: 'key', width: NAME_WIDTH },
  { name: SIZE_COLUMN, type: 'bigint', fullType: 'bytes', width: 96 },
  { name: KIND_COLUMN, type: 'varchar', fullType: 'kind', width: 96 },
  { name: MODIFIED_COLUMN, type: 'timestamp', fullType: 'timestamp', width: 160 },
  { name: CLASS_COLUMN, type: 'varchar', fullType: 'storage class', width: 130 },
  { name: ETAG_COLUMN, type: 'varchar', fullType: 'etag', width: 220 },
]

/**
 * What a row *is*, in the words a reader already has for it. A folder says so; an object says its
 * extension, which is the only thing a listing knows about what is inside it — a content type is a
 * `Head` and therefore a request per row.
 */
export function kindOf(row: ObjectRow): string {
  if (row.folder) return 'Folder'

  const at = row.name.lastIndexOf('.')
  if (at <= 0 || at === row.name.length - 1) return 'File'
  return row.name.slice(at + 1).toLowerCase()
}

/**
 * The page as one `GridData`. A **folder is a row** like any other and carries nothing but its name:
 * a prefix has no size, no clock and no tag, and drawing a zero for any of them would be claiming
 * something the service never said.
 */
export function objectGrid(rows: readonly ObjectRow[]): GridData {
  return {
    columns: COLUMNS,
    rows: rows.map((row) => cellsOf(row)),
  }
}

function cellsOf(row: ObjectRow): Cell[] {
  if (row.folder) return [row.name, '', kindOf(row), '', '', '']
  return [
    row.name,
    String(row.size),
    kindOf(row),
    row.modifiedAt,
    row.storageClass,
    row.etag,
  ]
}

/**
 * The page grouped into the tree its keys spell, for **Flatten** — where the rows are whole keys
 * rather than the tails of one folder. It is `keyRows` with a slash instead of a colon, which is the
 * whole of the difference: a bucket and a key space are both one flat namespace whose names are
 * paths, so there is one flattener and not two.
 *
 * A folder row is left out of it: in Flatten there are none, the service having been asked for every
 * key under the prefix and no prefixes at all.
 */
export function objectTree(rows: readonly ObjectRow[], shut: ReadonlySet<string>): KeyRow[] {
  return keyRows(
    rows.map((row) => row.key as Cell),
    shut,
    OBJECT_SEPARATOR,
  )
}

/**
 * The tree as a `GridData`, with each branch drawn as the segment it is and everything a branch has
 * no answer for left **blank** rather than `(Null)`: there is no object there, so there is no value
 * that is absent — the distinction a key space's own tree already makes.
 */
export function treeGrid(rows: readonly ObjectRow[], tree: readonly KeyRow[]): GridData {
  return {
    columns: COLUMNS,
    rows: tree.map((entry) => {
      const held = entry.source < 0 ? undefined : rows[entry.source]
      if (held === undefined) return [entry.label, '', 'Folder', '', '', '']

      const cells = cellsOf(held)
      // The bare segment: the prefix is on the row above, and how many keys are under a branch is
      // the grid's own pill beside the name rather than part of this text.
      cells[0] = entry.label
      return cells
    }),
  }
}

/**
 * How a size reads. Binary units, because that is what a file manager on any of the three desktops
 * this runs on shows for the same file — the rule `fileSizeText` already follows for a backup, and
 * the same function would do if a folder row did not have to read as **nothing** rather than as
 * `0 B`.
 */
export function sizeText(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`

  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let size = bytes / 1024
  let at = 0
  while (size >= 1024 && at < units.length - 1) {
    size /= 1024
    at += 1
  }
  return `${size < 10 ? size.toFixed(1) : Math.round(size)} ${units[at]}`
}

/** How a rate reads, which is a size a second and says so. */
export function rateText(bytesPerSecond: number): string {
  return bytesPerSecond <= 0 ? '' : `${sizeText(bytesPerSecond)}/s`
}

/**
 * How long the rest of a transfer will take, at the rate it is going now. It is **empty while the
 * total is still being counted**, for the reason the bar is: a number worked out from a total that is
 * climbing is a number that goes backwards, and a wrong estimate is worse than none.
 */
export function remainingText(moved: number, total: number, rate: number, counting: boolean): string {
  if (counting || rate <= 0 || total <= moved) return ''

  const seconds = Math.round((total - moved) / rate)
  if (seconds < 60) return `${seconds}s left`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m left`
  return `${(seconds / 3600).toFixed(1)}h left`
}
