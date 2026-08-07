import {
  Archive,
  ArchiveRestore,
  ArrowDownNarrowWide,
  ArrowUpDown,
  ArrowUpNarrowWide,
  Ban,
  ClipboardPaste,
  Copy,
  CopyPlus,
  Database,
  Download,
  Eraser,
  Eye,
  EyeOff,
  FileCode2,
  FileDown,
  Filter,
  FilterX,
  Fingerprint,
  FolderOpen,
  GitCompareArrows,
  GitMerge,
  Palette,
  PanelBottom,
  Pencil,
  Plug,
  Plus,
  RotateCw,
  Save,
  Settings2,
  Search,
  SlidersHorizontal,
  SquareTerminal,
  Table2,
  TableProperties,
  TextCursorInput,
  Trash2,
  Unplug,
  Upload,
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
  console: <SquareTerminal size={SIZE} />,
  executeFile: <FileCode2 size={SIZE} />,
  dump: <FileDown size={SIZE} />,
  // The two wizards point the way the data moves, which is the one thing a reader has to be sure
  // of before they open either: `import` fills a table, `export` empties one into files.
  importWizard: <Upload size={SIZE} />,
  exportWizard: <Download size={SIZE} />,
  colour: <Palette size={SIZE} />,

  // A backup goes into a shelf this app keeps and a restore comes back off it, which is what the
  // pair of glyphs says. `Extract SQL` deliberately wears `dump`'s: both write the same statements
  // to a file the reader names, and only where they were read from differs.
  backup: <Archive size={SIZE} />,
  restoreBackup: <ArchiveRestore size={SIZE} />,
  openFolder: <FolderOpen size={SIZE} />,

  newDatabase: <Database size={SIZE} />,

  open: <Table2 size={SIZE} />,
  design: <TableProperties size={SIZE} />,
  compare: <GitCompareArrows size={SIZE} />,
  // Looking for text across a whole namespace, which only ever reads — the glyph a search wears
  // everywhere, and the same one its tab wears in the strip.
  find: <Search size={SIZE} />,
  // Applying a comparison is a *merge* and wears the glyph that means one everywhere.
  migrate: <GitMerge size={SIZE} />,
  add: <Plus size={SIZE} />,
  duplicate: <CopyPlus size={SIZE} />,
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

  emptyString: <TextCursorInput size={SIZE} />,
  nullValue: <Ban size={SIZE} />,
  uuid: <Fingerprint size={SIZE} />,
  cellEditor: <PanelBottom size={SIZE} />,
  // The same glyph the tree's `Open View` wears, and for the same reason: both only look.
  viewValue: <Eye size={SIZE} />,
  saveAs: <Save size={SIZE} />,
  filter: <Filter size={SIZE} />,
  filterNone: <FilterX size={SIZE} />,
  filterPanel: <SlidersHorizontal size={SIZE} />,
} as const
