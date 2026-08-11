import { useEffect, useMemo } from 'react'

import type { Namespace } from '@/bridge/connections'
import type { TableStructure } from '@/bridge/schema'
import { namespaceKey, tableKey, useConnectionsStore } from '@/stores/connections'

import { boxOf, relationsOf, type Box, type Diagram, type Relation } from './model'

/**
 * What a diagram is drawn *from*: the namespace's table list, the shape of each table on the canvas,
 * and the relations between the ones that are.
 *
 * Nothing here is stored with the picture. A box names a table, and everything inside it — the
 * fields, the keys, the constraint a line stands for — is the catalogue's, read through the same
 * cache the tree and the designer read, so one Refresh moves all three at once.
 */
export interface DiagramData {
  /** One per node on the canvas, in the order the diagram holds them. */
  boxes: Box[]
  byTable: Map<string, Box>
  relations: Relation[]
  /** Every table of the namespace, which is what the palette lists. */
  tables: string[]
  structureOf: (table: string) => TableStructure | undefined
  /** Whether the table list itself has been read; the palette says so rather than showing nothing. */
  listed: boolean
}

const NO_TABLES: string[] = []

export function useDiagram(profileId: string, namespace: Namespace, diagram: Diagram): DiagramData {
  const structures = useConnectionsStore((s) => s.structures)
  const objects = useConnectionsStore((s) => s.objects)
  const loadObjects = useConnectionsStore((s) => s.loadObjects)
  const loadStructure = useConnectionsStore((s) => s.loadStructure)

  const held = objects[namespaceKey(profileId, namespace)]
  const tables = held?.tables ?? NO_TABLES

  // The list the palette offers. It is the same read the tree's own folders make, so a namespace
  // already opened in the sidebar costs nothing here.
  useEffect(() => {
    if (namespace.database === '') return
    void loadObjects(profileId, namespace)
  }, [loadObjects, profileId, namespace])

  // Every box's shape, read once per table and re-read only when something says it moved. The
  // dependency is the *names* rather than the nodes: dragging a box must not re-read anything.
  const drawn = diagram.nodes.map((node) => node.table).join('\n')

  useEffect(() => {
    if (drawn === '') return
    for (const table of drawn.split('\n')) {
      void loadStructure(profileId, namespace, table)
    }
  }, [drawn, loadStructure, profileId, namespace])

  return useMemo(() => {
    const keyOf = (table: string) => tableKey(profileId, namespace, table)
    const structureOf = (table: string) => structures[keyOf(table)]

    const boxes = diagram.nodes.map((node) => boxOf(node, structureOf(node.table), diagram.display))

    return {
      boxes,
      byTable: new Map(boxes.map((box) => [box.node.table, box])),
      relations: relationsOf(diagram.nodes, structures, keyOf),
      tables,
      structureOf,
      listed: held !== undefined,
    }
  }, [diagram.nodes, diagram.display, structures, tables, held, profileId, namespace])
}
