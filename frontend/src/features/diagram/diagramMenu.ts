import { CONNECTION_COLOURS } from '@/bridge/connections'
import { colourSwatch } from '@/features/connections/ConnectionColour'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'
import type { DiagramView } from '@/stores/diagrams'

import {
  nodeKey,
  pickedOf,
  shapeKey,
  type Diagram,
  type DiagramNode,
  type Point,
  type Relation,
} from './model'
import { fitWidth } from './text'
import type { DiagramData } from './useDiagram'

/**
 * Every right-click on a diagram, in one place — the arrangement `useTreeMenu` already follows, and
 * for the same reason: a menu is a list of *decisions*, and reading them beside each other is the
 * only way to see that the same action is offered the same way wherever it appears.
 *
 * The rule the whole file turns on is that **nothing here changes a schema except the one item that
 * says so**. Removing a box, colouring it, folding it shut and lining it up are all the picture's;
 * `Delete Foreign Key` is the single item with a statement behind it, and it is drawn in the danger
 * colour and confirmed like every other one in the app.
 */
interface DiagramMenuDeps {
  tabId: string
  diagram: Diagram
  /** What is picked, so an action on one box can be an action on all of them — see `many`. */
  selection: readonly string[]
  data: DiagramData
  /** What this engine calls one table. */
  word: string
  change: (tabId: string, edit: (diagram: Diagram) => Diagram, record?: boolean) => void
  look: (tabId: string, edit: Partial<DiagramView>) => void
  on: {
    openTable: (table: string) => void
    designTable: (table: string) => void
    /** Puts tables on the canvas, at a point or wherever there is room. */
    addTables: (tables: readonly string[], at: Point | null) => void
    centre: () => Point
    /** Every table the named one is joined to by a key, either way round. */
    joinedTo: (table: string) => string[]
    dropRelation: (relation: Relation) => void
    arrange: () => void
    fit: () => void
    refresh: () => void
    exportSvg: () => void
    clear: () => void
  }
}

export function diagramMenu(deps: DiagramMenuDeps): (target: string | null) => MenuItem[] {
  const { tabId, diagram, selection, data, word, change, look, on } = deps

  const canvasMenu = (): MenuItem[] => [
    { label: `Add Every ${word}`, icon: ICON.add, onSelect: () => on.addTables(data.tables, on.centre()) },
    { label: 'Arrange', icon: ICON.filterPanel, onSelect: on.arrange },
    { label: 'Fit to Window', icon: ICON.open, onSelect: on.fit },
    {
      label: 'Select All',
      icon: ICON.copy,
      separatorBefore: true,
      onSelect: () =>
        look(tabId, {
          selection: [
            ...diagram.nodes.map((one) => nodeKey(one.table)),
            ...diagram.shapes.map((one) => shapeKey(one.id)),
          ],
        }),
    },
    { label: 'Export as SVG…', icon: ICON.exportWizard, separatorBefore: true, onSelect: on.exportSvg },
    { label: 'Refresh', icon: ICON.refresh, onSelect: on.refresh },
    {
      label: 'Clear the Diagram',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: on.clear,
    },
  ]

  const relationMenu = (id: string): MenuItem[] => {
    const relation = data.relations.find((one) => one.id === id)
    if (!relation) return canvasMenu()

    return [
      {
        label: `${relation.from} → ${relation.to}`,
        icon: ICON.open,
        onSelect: () => on.openTable(relation.to),
      },
      {
        label: 'Delete Foreign Key',
        icon: ICON.delete,
        danger: true,
        separatorBefore: true,
        onSelect: () => on.dropRelation(relation),
      },
    ]
  }

  // A shape's order among the other shapes is the whole of what these two change: shapes are drawn
  // under every box always, so neither can put one over a table.
  const reorder = (id: string, front: boolean) =>
    change(tabId, (one) => {
      const shape = one.shapes.find((held) => held.id === id)
      if (!shape) return one

      const rest = one.shapes.filter((held) => held.id !== id)
      return { ...one, shapes: front ? [...rest, shape] : [shape, ...rest] }
    })

  const shapeMenu = (id: string): MenuItem[] => [
    { label: 'Edit Text', icon: ICON.rename, onSelect: () => look(tabId, { editing: id }) },
    { label: 'Bring to Front', icon: ICON.show, separatorBefore: true, onSelect: () => reorder(id, true) },
    { label: 'Send to Back', icon: ICON.hide, onSelect: () => reorder(id, false) },
    colourItem(
      (colour) =>
        change(tabId, (one) => ({
          ...one,
          shapes: one.shapes.map((shape) => (shape.id === id ? { ...shape, colour } : shape)),
        })),
      diagram.shapes.find((shape) => shape.id === id)?.colour ?? '',
    ),
    {
      label: 'Delete',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: () => {
        change(tabId, (one) => ({ ...one, shapes: one.shapes.filter((shape) => shape.id !== id) }))
        look(tabId, { selection: [] })
      },
    },
  ]

  const nodeMenu = (table: string): MenuItem[] => {
    const node = diagram.nodes.find((one) => one.table === table)
    const picked = pickedOf(selection, 'node')
    const many = picked.length > 1

    // An action on a box is an action on **every box picked**, so colouring four of them is one
    // gesture rather than four; with one picked the two are the same thing.
    const onEach = (edit: (node: DiagramNode) => DiagramNode) =>
      change(tabId, (one) => ({
        ...one,
        nodes: one.nodes.map((held) =>
          (many ? picked.includes(held.table) : held.table === table) ? edit(held) : held,
        ),
      }))

    return [
      { label: `Open ${word}`, icon: ICON.open, onSelect: () => on.openTable(table) },
      { label: `Design ${word}`, icon: ICON.design, onSelect: () => on.designTable(table) },
      {
        label: 'Add Referenced Tables',
        icon: ICON.add,
        separatorBefore: true,
        onSelect: () => on.addTables(on.joinedTo(table), on.centre()),
      },
      {
        label: node?.collapsed ? 'Expand' : 'Collapse',
        icon: node?.collapsed ? ICON.show : ICON.hide,
        onSelect: () => onEach((held) => ({ ...held, collapsed: !node?.collapsed })),
      },
      {
        label: 'Fit Width to Fields',
        icon: ICON.filterPanel,
        onSelect: () =>
          onEach((held) => ({
            ...held,
            width: fitWidth(held.table, data.structureOf(held.table)?.columns ?? []),
          })),
      },
      colourItem((colour) => onEach((held) => ({ ...held, colour })), node?.colour ?? ''),
      {
        // Off the picture, never off the server: dropping a table is a schema change and is asked
        // for where every other one is.
        label: many ? `Remove ${picked.length} from Diagram` : 'Remove from Diagram',
        icon: ICON.hide,
        separatorBefore: true,
        onSelect: () => {
          const going = new Set(many ? picked : [table])
          change(tabId, (one) => ({
            ...one,
            nodes: one.nodes.filter((held) => !going.has(held.table)),
          }))
          look(tabId, { selection: [] })
        },
      },
      { label: 'Refresh', icon: ICON.refresh, separatorBefore: true, onSelect: on.refresh },
    ]
  }

  return (target) => {
    if (target === null) return canvasMenu()

    const [kind, ...rest] = target.split(':')
    const id = rest.join(':')

    if (kind === 'relation') return relationMenu(id)
    if (kind === 'shape') return shapeMenu(id)
    return nodeMenu(id)
  }
}

/** The colour submenu every drawable thing is offered, drawn from the connection palette. */
function colourItem(onPick: (colour: string) => void, current: string): MenuItem {
  return {
    label: 'Colour',
    icon: ICON.colour,
    items: CONNECTION_COLOURS.map((one) => ({
      label: one.label,
      icon: colourSwatch(one.value, one.value === current),
      onSelect: () => onPick(one.value),
    })),
  }
}
