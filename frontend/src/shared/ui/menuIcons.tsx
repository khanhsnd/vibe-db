import {
  ArrowDownNarrowWide,
  ArrowUpDown,
  ArrowUpNarrowWide,
  ClipboardPaste,
  Copy,
  Database,
  Eraser,
  Eye,
  EyeOff,
  Pencil,
  Plug,
  Plus,
  RotateCw,
  Settings2,
  Table2,
  TableProperties,
  Trash2,
  Unplug,
  Zap,
} from 'lucide-react'

const SIZE = 13

/**
 * The glyph every context menu draws beside a label, in one place: the same action wears the same
 * icon wherever it is offered, and a menu built in a `.ts` file — `useTreeMenu` is one — can name
 * an icon without becoming a component file.
 *
 * `design` and `editField` are deliberately the same glyph: both open the designer.
 */
export const ICON = {
  connect: <Plug size={SIZE} />,
  disconnect: <Unplug size={SIZE} />,
  editConnection: <Settings2 size={SIZE} />,

  newDatabase: <Database size={SIZE} />,

  open: <Table2 size={SIZE} />,
  design: <TableProperties size={SIZE} />,
  add: <Plus size={SIZE} />,
  rename: <Pencil size={SIZE} />,
  empty: <Eraser size={SIZE} />,
  truncate: <Zap size={SIZE} />,
  delete: <Trash2 size={SIZE} />,
  refresh: <RotateCw size={SIZE} />,

  sortAscending: <ArrowUpNarrowWide size={SIZE} />,
  sortDescending: <ArrowDownNarrowWide size={SIZE} />,
  sortNone: <ArrowUpDown size={SIZE} />,
  copy: <Copy size={SIZE} />,
  paste: <ClipboardPaste size={SIZE} />,
  hide: <EyeOff size={SIZE} />,
  show: <Eye size={SIZE} />,
} as const
