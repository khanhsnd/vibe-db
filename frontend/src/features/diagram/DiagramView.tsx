import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Network } from 'lucide-react'

import { copyText } from '@/bridge/clipboard'
import { engineInfo, engineWords } from '@/bridge/connections'
import { files } from '@/bridge/files'
import { schema } from '@/bridge/schema'
import { Empty } from '@/shared/ui/Empty'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { diagramOf, IDLE_VIEW, useDiagramsStore } from '@/stores/diagrams'
import { useSchemaStore } from '@/stores/schema'
import { useTabsStore, type DiagramTab } from '@/stores/tabs'

import { DiagramCanvas } from './DiagramCanvas'
import { DiagramInspector } from './DiagramInspector'
import { diagramMenu } from './diagramMenu'
import { DiagramToolbar } from './DiagramToolbar'
import { arranged, freeSpot, rectOf } from './layout'
import {
  bounds,
  clampZoom,
  DEFAULT_NODE_WIDTH,
  EMPTY_DIAGRAM,
  newNode,
  nodeKey,
  snapped,
  type DiagramNode,
  type LinkDraft,
  type Point,
  type Rect,
  type Relation,
} from './model'
import { diagramKey } from './persist'
import { RelationDialog } from './RelationDialog'
import { canvasColour, toSvgFile } from './svg'
import { TablePalette } from './TablePalette'
import { useDiagram } from './useDiagram'

/**
 * A namespace's tables drawn as boxes with the foreign keys between them as lines — the tab
 * `Diagram` opens.
 *
 * Three things about it are worth knowing before reading the code:
 *
 * 1. **The picture is the reader's; the contents are the server's.** What is stored is where each
 *    box sits, how wide it is, what colour it wears and what was written on the notes. Every field,
 *    key and line is read from the same catalogue cache the tree and the designer read, so a
 *    diagram cannot show a column that has been dropped or miss one that was added.
 * 2. **Drawing a line declares a foreign key.** A dragged line raises the dialog, the dialog calls
 *    the same bound method the tree's own Add Foreign Key calls, and the line appears because the
 *    catalogue now says it is there. Nothing is drawn that a server has not agreed to.
 * 3. **Delete takes a table off the diagram and nothing else.** Dropping a table is a schema change
 *    and lives where every other one does: a confirmation with a statement behind it.
 *
 * This file is the wiring — what each action *is*. Where things are drawn is `DiagramCanvas`, what a
 * press means is `useCanvasPointer`, and what a right-click offers is `diagramMenu`.
 */
export function DiagramView({ tab }: { tab: DiagramTab }) {
  const key = diagramKey(tab.profileId, tab.namespace)

  const start = useDiagramsStore((s) => s.start)
  const change = useDiagramsStore((s) => s.change)
  const replace = useDiagramsStore((s) => s.replace)
  const look = useDiagramsStore((s) => s.look)
  const diagram = useDiagramsStore((s) => diagramOf(s.docs, tab.id))
  const view = useDiagramsStore((s) => s.views[tab.id] ?? IDLE_VIEW)
  const undoable = useDiagramsStore((s) => (s.history[tab.id]?.past.length ?? 0) > 0)
  const redoable = useDiagramsStore((s) => (s.history[tab.id]?.future.length ?? 0) > 0)

  const profiles = useConnectionsStore((s) => s.profiles)
  const refreshObjects = useConnectionsStore((s) => s.refreshObjects)
  const refreshStructure = useConnectionsStore((s) => s.refreshStructure)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const reportError = useConnectionsStore((s) => s.reportError)

  const openTable = useTabsStore((s) => s.openTable)
  const openDesign = useTabsStore((s) => s.openDesign)
  const ask = useSchemaStore((s) => s.ask)

  const engine = profiles.find((one) => one.id === tab.profileId)?.engine ?? ''
  const info = engineInfo(engine)
  const words = engineWords(engine)

  const surface = useRef<SVGSVGElement | null>(null)
  const [draft, setDraft] = useState<LinkDraft | null>(null)

  // The tab is given its picture here rather than by whatever opened it, so a diagram tab cannot
  // exist without one — the same rule a table tab's view follows.
  useEffect(() => {
    start(tab.id, key)
  }, [start, tab.id, key])

  const data = useDiagram(tab.profileId, tab.namespace, diagram)
  const drawn = useMemo(() => new Set(diagram.nodes.map((node) => node.table)), [diagram.nodes])

  /** The middle of what is on screen, in the diagram's own coordinates. */
  const centre = useCallback((): Point => {
    const rect = surface.current?.getBoundingClientRect()
    if (!rect) return { x: 0, y: 0 }
    return {
      x: (rect.width / 2 - diagram.panX) / diagram.zoom,
      y: (rect.height / 2 - diagram.panY) / diagram.zoom,
    }
  }, [diagram.panX, diagram.panY, diagram.zoom])

  /**
   * Puts tables on the canvas. One dropped by hand lands where it was dropped; the rest go into the
   * first free space from there, so adding six of them is six boxes rather than six in one place.
   */
  const addTables = useCallback(
    (tables: readonly string[], at: Point | null) => {
      const wanted = tables.filter((table) => !drawn.has(table))
      if (wanted.length === 0) return

      const taken: Rect[] = data.boxes.map(rectOf)
      const added: DiagramNode[] = []
      const guess = { width: DEFAULT_NODE_WIDTH, height: 110 }
      let spot = at ?? { x: 0, y: 0 }

      for (const [index, table] of wanted.entries()) {
        const place =
          at !== null && index === 0
            ? { x: snapped(at.x, diagram.snap), y: snapped(at.y, diagram.snap) }
            : freeSpot(taken, guess.width, guess.height, spot)

        added.push(newNode(table, place))
        taken.push({ ...place, ...guess })
        spot = place
      }

      change(tab.id, (one) => ({ ...one, nodes: [...one.nodes, ...added] }))
      look(tab.id, { selection: wanted.map(nodeKey) })
    },
    [change, look, tab.id, drawn, data.boxes, diagram.snap],
  )

  const addAt = useCallback(
    (table: string, at: { clientX: number; clientY: number } | null) => {
      const rect = surface.current?.getBoundingClientRect()
      const point =
        at && rect
          ? {
              x: (at.clientX - rect.left - diagram.panX) / diagram.zoom,
              y: (at.clientY - rect.top - diagram.panY) / diagram.zoom,
            }
          : centre()
      addTables([table], point)
    },
    [addTables, centre, diagram.panX, diagram.panY, diagram.zoom],
  )

  /** Zooms about the middle of the pane, which is the only point a button on a toolbar can mean. */
  const zoomBy = useCallback(
    (factor: number) => {
      const rect = surface.current?.getBoundingClientRect()

      change(
        tab.id,
        (one) => {
          const zoom = clampZoom(one.zoom * factor)
          if (!rect) return { ...one, zoom }

          const scale = zoom / one.zoom
          const x = rect.width / 2
          const y = rect.height / 2
          return { ...one, zoom, panX: x - (x - one.panX) * scale, panY: y - (y - one.panY) * scale }
        },
        false,
      )
    },
    [change, tab.id],
  )

  const fit = useCallback(() => {
    const rect = surface.current?.getBoundingClientRect()
    const box = bounds(data.boxes, diagram.shapes)
    if (!rect || box.width === 0 || box.height === 0) return

    const zoom = clampZoom(Math.min(rect.width / box.width, rect.height / box.height))
    change(
      tab.id,
      (one) => ({
        ...one,
        zoom,
        panX: (rect.width - box.width * zoom) / 2 - box.x * zoom,
        panY: (rect.height - box.height * zoom) / 2 - box.y * zoom,
      }),
      false,
    )
  }, [change, tab.id, data.boxes, diagram.shapes])

  const arrange = useCallback(() => {
    if (data.boxes.length === 0) return
    change(tab.id, (one) => ({ ...one, nodes: arranged(data.boxes, data.relations) }))
  }, [change, tab.id, data.boxes, data.relations])

  const refresh = useCallback(() => {
    void refreshObjects(tab.profileId, tab.namespace)
    for (const node of diagram.nodes) {
      void refreshStructure(tab.profileId, tab.namespace, node.table)
    }
  }, [refreshObjects, refreshStructure, tab.profileId, tab.namespace, diagram.nodes])

  /** The whole diagram as a standalone file — everything drawn, not just what is on screen. */
  const picture = useCallback(() => {
    const element = surface.current
    if (!element) return ''
    return toSvgFile(element, bounds(data.boxes, diagram.shapes), canvasColour())
  }, [data.boxes, diagram.shapes])

  const exportSvg = useCallback(() => {
    const text = picture()
    if (text === '') return

    const named = `${tab.namespace.schema || tab.namespace.database || 'diagram'}.svg`
    void files.saveText('SVG image', '*.svg', named, text).catch(reportError)
  }, [picture, tab.namespace, reportError])

  const copySvg = useCallback(() => {
    const text = picture()
    if (text !== '') void copyText(text)
  }, [picture])

  const clear = useCallback(() => {
    ask({
      kind: 'confirm',
      title: 'Clear the Diagram',
      message:
        'Take every box, line and note off this diagram? Nothing on the server changes — the ' +
        'tables stay exactly as they are, and they can be drawn again from the list.',
      submitLabel: 'Clear',
      danger: true,
      onSubmit: async () => {
        // What the reader was *looking* through stays: the display mode, the grid and the snap are
        // how they read a diagram rather than part of this one.
        replace(tab.id, {
          ...EMPTY_DIAGRAM,
          display: diagram.display,
          snap: diagram.snap,
          grid: diagram.grid,
          names: diagram.names,
        })
        look(tab.id, { selection: [] })
      },
    })
  }, [ask, replace, look, tab.id, diagram.display, diagram.snap, diagram.grid, diagram.names])

  const dropRelation = useCallback(
    (relation: Relation) => {
      ask({
        kind: 'confirm',
        title: 'Delete Foreign Key',
        message: `Drop the constraint ${relation.name}? The rows stay; nothing checks them again.`,
        submitLabel: 'Delete',
        danger: true,
        onSubmit: () =>
          changeSchema(
            tab.profileId,
            { namespace: tab.namespace, table: relation.from },
            (sessionId) =>
              schema.dropForeignKey(sessionId, tab.namespace, relation.from, relation.name),
          ),
      })
    },
    [ask, changeSchema, tab.profileId, tab.namespace],
  )

  /** Every table the named one is joined to by a key, either way round — what Add Referenced adds. */
  const joinedTo = useCallback(
    (table: string): string[] => {
      const structure = data.structureOf(table)
      const out = new Set((structure?.foreignKeys ?? []).map((one) => one.referencedTable))

      for (const other of data.tables) {
        const held = data.structureOf(other)
        if (held?.foreignKeys.some((one) => one.referencedTable === table)) out.add(other)
      }
      out.delete(table)
      return [...out].filter((one) => data.tables.includes(one))
    },
    [data],
  )

  // Built on the render rather than memoised: it is a closure over what is already here, and a
  // dependency list naming every action would be longer than the menu it guards.
  const menuFor = diagramMenu({
    tabId: tab.id,
    diagram,
    selection: view.selection,
    data,
    word: words.table,
    change,
    look,
    on: {
      openTable: (table) => openTable({ profileId: tab.profileId, namespace: tab.namespace, table }),
      designTable: (table) =>
        openDesign({ profileId: tab.profileId, namespace: tab.namespace, table }),
      addTables,
      centre,
      joinedTo,
      dropRelation,
      arrange,
      fit,
      refresh,
      exportSvg,
      clear,
    },
  })

  // The ids of the panels actually mounted, so the divider between two of them is remembered per
  // *combination* — opening the inspector must not resize the canvas by a layout saved without it.
  const panels = [
    ...(view.palette ? ['palette'] : []),
    'canvas',
    ...(view.inspector ? ['inspector'] : []),
  ]

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <DiagramToolbar
        tabId={tab.id}
        diagram={diagram}
        view={view}
        linkable={info.hasForeignKeys}
        canUndo={undoable}
        canRedo={redoable}
        onZoom={zoomBy}
        onFit={fit}
        onArrange={arrange}
        onAddAll={() => addTables(data.tables, centre())}
        onRefresh={refresh}
        onExport={exportSvg}
        onCopy={copySvg}
        onClear={clear}
      />

      <Split
        id="vibe-db.diagram"
        orientation="horizontal"
        panelIds={panels}
        className="min-h-0 flex-1"
      >
        {view.palette ? (
          <>
            <SplitPanel
              id="palette"
              defaultSize={200}
              minSize={140}
              maxSize="40%"
              className="min-h-0"
            >
              <TablePalette
                tables={data.tables}
                onCanvas={drawn}
                listed={data.listed}
                words={words}
                onAdd={addAt}
                onReveal={(table) => look(tab.id, { selection: [nodeKey(table)] })}
              />
            </SplitPanel>

            <SplitHandle orientation="horizontal" />
          </>
        ) : null}

        <SplitPanel id="canvas" minSize="30%" className="min-h-0">
          {/* The canvas is always there, even with nothing on it: it is what a table is dropped
              onto, and a hint standing in its place would be a hint nothing could be dropped on. */}
          <div className="relative h-full min-h-0">
            <DiagramCanvas
              tabId={tab.id}
              diagram={diagram}
              view={view}
              data={data}
              linkable={info.hasForeignKeys}
              word={words.table}
              surface={surface}
              onLink={setDraft}
              onOpenTable={(table) =>
                openTable({ profileId: tab.profileId, namespace: tab.namespace, table })
              }
              menuFor={menuFor}
            />

            {diagram.nodes.length === 0 && diagram.shapes.length === 0 ? (
              <div className="pointer-events-none absolute inset-0">
                <Empty
                  icon={<Network size={26} strokeWidth={1.5} />}
                  title="Nothing drawn yet"
                  hint={`Drag a ${words.table.toLowerCase()} in from the left, or add every one of them from the toolbar.`}
                />
              </div>
            ) : null}
          </div>
        </SplitPanel>

        {view.inspector ? (
          <>
            <SplitHandle orientation="horizontal" />
            <SplitPanel
              id="inspector"
              defaultSize={260}
              minSize={200}
              maxSize="45%"
              className="min-h-0"
            >
              <DiagramInspector
                tabId={tab.id}
                diagram={diagram}
                view={view}
                data={data}
                words={words}
                onOpenTable={(table) =>
                  openTable({ profileId: tab.profileId, namespace: tab.namespace, table })
                }
                onDesignTable={(table) =>
                  openDesign({ profileId: tab.profileId, namespace: tab.namespace, table })
                }
                onDropRelation={dropRelation}
              />
            </SplitPanel>
          </>
        ) : null}
      </Split>

      <div className="flex h-6 shrink-0 items-center gap-3 border-t border-line bg-panel px-2 text-2xs text-muted">
        <span>{`${diagram.nodes.length} of ${data.tables.length} ${words.tables.toLowerCase()}`}</span>
        <span>{`${data.relations.length} relations`}</span>
        {diagram.shapes.length > 0 ? <span>{`${diagram.shapes.length} notes`}</span> : null}
        <span className="flex-1" />
        {info.hasForeignKeys ? null : (
          <span className="text-faint">
            This engine has no foreign keys, so no line can be drawn.
          </span>
        )}
        <span className="font-mono text-faint">{selectionLabel(view.selection)}</span>
      </div>

      <RelationDialog
        draft={draft}
        profileId={tab.profileId}
        namespace={tab.namespace}
        structureOf={data.structureOf}
        onClose={() => setDraft(null)}
      />
    </div>
  )
}

/** What the bar says is picked: the thing itself where there is one, and a count where there are more. */
function selectionLabel(selection: readonly string[]): string {
  if (selection.length === 0) return ''
  if (selection.length > 1) return `${selection.length} selected`

  const [only = ''] = selection
  return only.split(':').slice(1).join(':')
}
