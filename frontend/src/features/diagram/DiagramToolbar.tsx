import {
  Circle,
  Copy,
  Download,
  Hand,
  LayoutGrid,
  Magnet,
  Maximize,
  MousePointer2,
  PanelLeft,
  PanelRight,
  Plus,
  Redo2,
  RotateCw,
  Spline,
  Square,
  StickyNote,
  Tag,
  Trash2,
  Type,
  Undo2,
  ZoomIn,
  ZoomOut,
  type LucideIcon,
} from 'lucide-react'

import { cn } from '@/lib/cn'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'
import { MenuButton } from '@/shared/ui/MenuButton'
import { useDiagramsStore, type DiagramView } from '@/stores/diagrams'

import { DISPLAY_MODES, type Diagram, type DisplayMode, type Tool } from './model'

/**
 * Everything a diagram is done *to*, in one strip.
 *
 * It reads left to right as three questions: what a press does — the tools — then what is being
 * looked at — the zoom, the fit, how much of each table is drawn — then what the picture is made of:
 * arranging it, filling it, taking it somewhere. A toggle draws itself pressed rather than saying
 * what it would do, because there is no room for a sentence and the canvas behind it says the rest.
 */
interface DiagramToolbarProps {
  tabId: string
  diagram: Diagram
  view: DiagramView
  /** Whether a relation may be drawn at all — see `EngineInfo.hasForeignKeys`. */
  linkable: boolean
  canUndo: boolean
  canRedo: boolean
  /** Zooms about the middle of what is on screen, which is the only place a button can mean. */
  onZoom: (factor: number) => void
  onFit: () => void
  onArrange: () => void
  onAddAll: () => void
  onRefresh: () => void
  onExport: () => void
  onCopy: () => void
  onClear: () => void
}

const TOOLS: ReadonlyArray<{ value: Tool; label: string; icon: LucideIcon; key: string }> = [
  { value: 'select', label: 'Select', icon: MousePointer2, key: 'V' },
  { value: 'pan', label: 'Pan', icon: Hand, key: 'H' },
  { value: 'relation', label: 'Draw relation', icon: Spline, key: 'R' },
  { value: 'note', label: 'Note', icon: StickyNote, key: 'N' },
  { value: 'rectangle', label: 'Rectangle', icon: Square, key: 'B' },
  { value: 'ellipse', label: 'Ellipse', icon: Circle, key: 'E' },
  { value: 'text', label: 'Text', icon: Type, key: 'T' },
]

export function DiagramToolbar({
  tabId,
  diagram,
  view,
  linkable,
  canUndo,
  canRedo,
  onZoom,
  onFit,
  onArrange,
  onAddAll,
  onRefresh,
  onExport,
  onCopy,
  onClear,
}: DiagramToolbarProps) {
  const change = useDiagramsStore((s) => s.change)
  const look = useDiagramsStore((s) => s.look)
  const undo = useDiagramsStore((s) => s.undo)
  const redo = useDiagramsStore((s) => s.redo)

  return (
    <div className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5">
      {TOOLS.map(({ value, label, icon: Icon, key }) => (
        <IconButton
          key={value}
          label={`${label} (${key})`}
          disabled={value === 'relation' && !linkable}
          icon={<Icon size={14} />}
          onClick={() => look(tabId, { tool: value })}
          className={cn(view.tool === value && 'bg-selected text-fg')}
        />
      ))}

      <Divider />

      <IconButton
        label="Undo (Ctrl-Z)"
        icon={<Undo2 size={14} />}
        disabled={!canUndo}
        onClick={() => undo(tabId)}
      />
      <IconButton
        label="Redo (Ctrl-Shift-Z)"
        icon={<Redo2 size={14} />}
        disabled={!canRedo}
        onClick={() => redo(tabId)}
      />

      <Divider />

      <IconButton label="Zoom out" icon={<ZoomOut size={14} />} onClick={() => onZoom(1 / 1.2)} />
      <button
        type="button"
        title="Actual size"
        onClick={() => onZoom(1 / diagram.zoom)}
        className="h-6 min-w-11 rounded-md px-1 font-mono text-2xs text-muted hover:bg-hover hover:text-fg"
      >
        {`${Math.round(diagram.zoom * 100)}%`}
      </button>
      <IconButton label="Zoom in" icon={<ZoomIn size={14} />} onClick={() => onZoom(1.2)} />
      <IconButton label="Fit to window" icon={<Maximize size={14} />} onClick={onFit} />

      <Divider />

      <select
        aria-label="How much of each table is drawn"
        title="How much of each table is drawn"
        value={diagram.display}
        onChange={(event) =>
          change(tabId, (one) => ({ ...one, display: event.target.value as DisplayMode }))
        }
        className={TOOLBAR_CONTROL}
      >
        {DISPLAY_MODES.map((mode) => (
          <option key={mode.value} value={mode.value}>
            {mode.label}
          </option>
        ))}
      </select>

      <IconButton
        label="Show relation names"
        icon={<Tag size={14} />}
        onClick={() => change(tabId, (one) => ({ ...one, names: !one.names }), false)}
        className={cn(diagram.names && 'bg-selected text-fg')}
      />
      <IconButton
        label="Show the grid"
        icon={<LayoutGrid size={14} />}
        onClick={() => change(tabId, (one) => ({ ...one, grid: !one.grid }), false)}
        className={cn(diagram.grid && 'bg-selected text-fg')}
      />
      <IconButton
        label="Snap to the grid"
        icon={<Magnet size={14} />}
        onClick={() => change(tabId, (one) => ({ ...one, snap: !one.snap }), false)}
        className={cn(diagram.snap && 'bg-selected text-fg')}
      />

      <Divider />

      <IconButton label="Arrange the boxes" icon={<LayoutGrid size={14} />} onClick={onArrange} />
      <IconButton label="Add every table" icon={<Plus size={14} />} onClick={onAddAll} />
      <IconButton label="Re-read the tables" icon={<RotateCw size={14} />} onClick={onRefresh} />

      <MenuButton
        items={[
          { label: 'Export as SVG…', icon: <Download size={13} />, onSelect: onExport },
          { label: 'Copy as SVG', icon: <Copy size={13} />, onSelect: onCopy },
          {
            label: 'Clear the diagram',
            icon: <Trash2 size={13} />,
            danger: true,
            separatorBefore: true,
            onSelect: onClear,
          },
        ]}
      >
        <IconButton label="Take the diagram somewhere" icon={<Download size={14} />} />
      </MenuButton>

      <div className="flex-1" />

      <IconButton
        label="Show the table list"
        icon={<PanelLeft size={14} />}
        onClick={() => look(tabId, { palette: !view.palette })}
        className={cn(view.palette && 'bg-selected text-fg')}
      />
      <IconButton
        label="Show what is selected"
        icon={<PanelRight size={14} />}
        onClick={() => look(tabId, { inspector: !view.inspector })}
        className={cn(view.inspector && 'bg-selected text-fg')}
      />
    </div>
  )
}

const Divider = () => <span className="mx-0.5 h-4 w-px shrink-0 bg-line" />
