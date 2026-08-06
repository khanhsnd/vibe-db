import {
  differs,
  type DiffStatus,
  type FieldDiff,
  type ForeignKeyDiff,
  type IndexDiff,
  type StructureDiff,
} from '@/bridge/compare'
import type { TableColumn } from '@/bridge/tables'
import type { NodeKind } from '@/features/connections/tree'

import { pickKey, SECTION } from './migrate'

/**
 * The branch the whole comparison hangs from: the **table**, which each side names for itself.
 * A comparison is of two tables and of nothing else, so there is one root and it is drawn — the
 * sections are what it holds, exactly as the sidebar draws a table.
 */
export const ROOT = 'table'

/**
 * The three lists a table's shape is made of, in the order the sidebar's own tree draws them —
 * fields, then indexes, then constraints. Each carries the `NodeKind` its rows are drawn with, so a
 * field wears the same glyph and the same colour here as it does in the tree.
 */
export const SECTIONS = [
  { id: SECTION.field, label: 'Fields', folder: 'fields', leaf: 'field' },
  { id: SECTION.index, label: 'Indexes', folder: 'indexes', leaf: 'index' },
  { id: SECTION.key, label: 'Foreign Keys', folder: 'foreignKeys', leaf: 'foreignKey' },
] as const satisfies ReadonlyArray<{
  id: string
  label: string
  folder: NodeKind
  leaf: NodeKind
}>

export type SectionId = (typeof SECTIONS)[number]['id']
type Section = (typeof SECTIONS)[number]

/** Every branch there is, which is what folding or opening the whole tree at once names. */
export const BRANCHES: readonly string[] = [ROOT, ...SECTIONS.map((section) => section.id)]

/**
 * What one side of a row holds: what that side calls the thing, and what it declares it as. Both
 * are **blank where the side has not got the thing at all**, which is what the pane opposite a
 * difference draws — and is neither a value nor an empty one.
 */
export interface RowSide {
  name: string
  declaration: string
}

/**
 * One row of the structure tree, flattened. The **root** is the table, a **section** is the folder
 * under it — it declares nothing and stands for everything inside — and a **leaf** is one field,
 * index or constraint.
 *
 * The tree is flattened here rather than in the grid, because a grid has rows and not branches:
 * what it is given is a list, and the `depth` and the caret on each row are what make it read as
 * the tree it came from. See `GridOutline`.
 */
export interface StructureRow {
  kind: NodeKind
  depth: number
  /**
   * Which branch a fold is remembered under — the root, or one of the three sections. A leaf has
   * none. It is never the row's *label*, which is what a reader reads and not what the shape is
   * named.
   */
  branch: string | undefined
  status: DiffStatus
  /** What differs, or how many of a section's things do. */
  detail: string
  /** What each side calls this thing and declares it as. Both panes are drawn from the one row. */
  left: RowSide
  right: RowSide
  /** Open, shut, or `undefined` for a leaf — which is what the caret is drawn from. */
  open: boolean | undefined
  /**
   * Every pick this row stands for: one for a leaf, a **section** for a folder and the **whole
   * comparison** for the root — which is what makes a folder's tick "take all of these" rather
   * than a fourth kind of control.
   */
  picks: string[]
}

/** What each side is called, which is what the root row names each pane's table with. */
export interface SideNames {
  left: string
  right: string
}

interface TreeOptions {
  names: SideNames
  /** The branches the reader has folded shut. */
  shut: ReadonlySet<string>
  differencesOnly: boolean
  /** Whether this pair of engines has referential constraints at all. */
  referential: boolean
}

/**
 * A table's whole shape as one list of rows — the table, its sections, and everything in the ones
 * that are open. One list and not two: both panes are drawn from it, so row *n* of one is row *n*
 * of the other by construction and the two trees fold together rather than drifting apart.
 */
export function structureRows(diff: StructureDiff, options: TreeOptions): StructureRow[] {
  const branches = SECTIONS.filter(
    (section) => section.id !== SECTION.key || options.referential,
  ).map((section) => branchOf(diff, section, options))

  const open = !options.shut.has(ROOT)
  const picks = branches.flatMap((branch) => branch.folder.picks)

  const root: StructureRow = {
    kind: 'table',
    depth: 0,
    branch: ROOT,
    status: 'same',
    detail: countDetail(
      branches.reduce((sum, branch) => sum + branch.total, 0),
      picks.length,
    ),
    left: { name: options.names.left, declaration: '' },
    right: { name: options.names.right, declaration: '' },
    open,
    picks,
  }

  return open ? [root, ...branches.flatMap((branch) => branch.rows)] : [root]
}

/** One section, folded or open, and how many things it holds however it is drawn. */
interface Branch {
  folder: StructureRow
  /** The folder, and the leaves under it where it is open. */
  rows: StructureRow[]
  total: number
}

function branchOf(diff: StructureDiff, section: Section, options: TreeOptions): Branch {
  const pairs = pairsOf(diff, section.id)
  const leaves = pairs.map((pair) => leafRow(pair, section, options))
  const differing = leaves.filter((leaf) => differs(leaf.status))
  const open = !options.shut.has(section.id)

  const folder: StructureRow = {
    kind: section.folder,
    depth: 1,
    branch: section.id,
    // A folder is never itself a difference: what it says is how many of its things are.
    status: 'same',
    detail: countDetail(pairs.length, differing.length),
    left: { name: section.label, declaration: '' },
    right: { name: section.label, declaration: '' },
    open,
    picks: differing.flatMap((leaf) => leaf.picks),
  }

  const shown = options.differencesOnly ? differing : leaves
  return { folder, rows: open ? [folder, ...shown] : [folder], total: pairs.length }
}

function countDetail(total: number, differing: number): string {
  if (total === 0) return 'neither side has any'
  if (differing === 0) return `all ${total} identical`
  return `${differing} of ${total} differ`
}

/** One pair as a row: what each side calls it and declares it as, and what moved between them. */
function leafRow(pair: Pair, section: Section, options: TreeOptions): StructureRow {
  const side = (held: Pair['left']): RowSide =>
    held === null
      ? { name: '', declaration: '' }
      : { name: pair.name, declaration: declare(section.id, held) }

  return {
    kind: section.leaf,
    depth: 2,
    branch: undefined,
    status: pair.status,
    detail: detailOf(pair, options.names),
    left: side(pair.left),
    right: side(pair.right),
    open: undefined,
    picks: differs(pair.status) ? [pickKey(section.id, pair.name)] : [],
  }
}

function detailOf(pair: Pair, names: SideNames): string {
  switch (pair.status) {
    case 'same':
      return ''
    case 'added':
      return `only in ${names.right}`
    case 'removed':
      return `only in ${names.left}`
    default:
      return pair.differs.join(', ')
  }
}

type Pair = FieldDiff | IndexDiff | ForeignKeyDiff

function pairsOf(diff: StructureDiff, section: SectionId): Pair[] {
  switch (section) {
    case SECTION.field:
      return diff.fields
    case SECTION.index:
      return diff.indexes
    default:
      return diff.foreignKeys
  }
}

/**
 * One thing as the one-line declaration a reader tells two of them apart by — a type with its
 * nullability and default, an index with its columns and its method, a key with what it points at.
 * That, and not a row of five cells, is what "these two are different" is read from.
 */
function declare(section: SectionId, one: NonNullable<Pair['left']>): string {
  switch (section) {
    case SECTION.field:
      return declareField(one as TableColumn)
    case SECTION.index:
      return declareIndex(one as IndexShape)
    default:
      return declareKey(one as KeyShape)
  }
}

interface IndexShape {
  columns: string[]
  unique: boolean
  method: string
}

interface KeyShape {
  columns: string[]
  referencedSchema: string
  referencedTable: string
  referencedColumns: string[]
  onUpdate: string
  onDelete: string
}

function declareField(field: TableColumn): string {
  const parts = [field.fullType || field.type, field.nullable ? 'NULL' : 'NOT NULL']
  if (field.default !== null) parts.push(`DEFAULT ${field.default}`)
  if (field.primaryKey) parts.push('PRIMARY KEY')
  return parts.join(' ')
}

function declareIndex(index: IndexShape): string {
  const parts = [`${index.unique ? 'UNIQUE ' : ''}(${index.columns.join(', ')})`]
  if (index.method !== '') parts.push(`USING ${index.method}`)
  return parts.join(' ')
}

function declareKey(key: KeyShape): string {
  const target =
    key.referencedSchema === ''
      ? key.referencedTable
      : `${key.referencedSchema}.${key.referencedTable}`

  const parts = [`(${key.columns.join(', ')}) → ${target}(${key.referencedColumns.join(', ')})`]
  if (key.onUpdate !== '') parts.push(`ON UPDATE ${key.onUpdate}`)
  if (key.onDelete !== '') parts.push(`ON DELETE ${key.onDelete}`)
  return parts.join(' ')
}
