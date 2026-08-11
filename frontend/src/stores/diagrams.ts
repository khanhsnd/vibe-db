import { create } from 'zustand'

import { EMPTY_DIAGRAM, type Diagram, type Tool } from '@/features/diagram/model'
import { loadDiagram, saveDiagram } from '@/features/diagram/persist'

/**
 * One diagram tab's picture and everything the reader has done to it.
 *
 * It is here rather than in the component for the reason a table tab's view is: **only the tab in
 * front is mounted**, so a canvas looked away from is taken down and anything it was holding goes
 * with it. What is drawn outlives that — and outlives the window, through `persist.ts` — while what
 * is merely *picked* does not, which is the one line between the two halves below.
 */

export interface DiagramView {
  tool: Tool
  /** What is picked, as `node:` / `shape:` / `relation:` keys — see `model.pickKey`. */
  selection: readonly string[]
  /** The shape whose words are being typed, and `null` while none is. */
  editing: string | null
  palette: boolean
  inspector: boolean
}

const NOTHING_PICKED: readonly string[] = []

/** A tab nobody has drawn on, and a stable snapshot for a selector to fall back to. */
export const IDLE_VIEW: DiagramView = {
  tool: 'select',
  selection: NOTHING_PICKED,
  editing: null,
  palette: true,
  inspector: false,
}

/** How many steps back a diagram remembers. A step is the whole picture, so the bound is real. */
const HISTORY_DEPTH = 60

interface Past {
  past: Diagram[]
  future: Diagram[]
}

const NO_HISTORY: Past = { past: [], future: [] }

interface DiagramsState {
  docs: Record<string, Diagram>
  views: Record<string, DiagramView>
  /** Which stored diagram each tab is, so a change knows what to write down. */
  keys: Record<string, string>
  history: Record<string, Past>

  /** Gives a newly opened tab the diagram stored for its namespace. A tab reused keeps its own. */
  start: (tabId: string, key: string) => void
  /**
   * One change to the picture. `record` is what tells an *edit* from a *look*: moving a box is
   * undoable and panning the canvas is not, or a reader's Ctrl-Z would undo the scrolling they did
   * to find what they wanted to undo.
   */
  change: (tabId: string, edit: (diagram: Diagram) => Diagram, record?: boolean) => void
  /** Replaces the whole picture — what Clear and a fresh Arrange do. */
  replace: (tabId: string, diagram: Diagram) => void
  /**
   * Marks the picture a drag started from as a step to come back to. A drag writes sixty times a
   * second and none of those may be an undo step, so it writes without recording and says *here*
   * that the whole drag was one.
   */
  commit: (tabId: string, before: Diagram) => void
  look: (tabId: string, edit: Partial<DiagramView>) => void
  undo: (tabId: string) => void
  redo: (tabId: string) => void
  discard: (tabId: string) => void
}

export const useDiagramsStore = create<DiagramsState>((set, get) => ({
  docs: {},
  views: {},
  keys: {},
  history: {},

  start: (tabId, key) => {
    if (get().docs[tabId]) return

    set((s) => ({
      docs: { ...s.docs, [tabId]: loadDiagram(key) },
      views: { ...s.views, [tabId]: IDLE_VIEW },
      keys: { ...s.keys, [tabId]: key },
      history: { ...s.history, [tabId]: NO_HISTORY },
    }))
  },

  change: (tabId, edit, record = true) =>
    set((s) => {
      const held = s.docs[tabId]
      if (!held) return s

      const next = edit(held)
      if (next === held) return s

      const key = s.keys[tabId]
      if (key) saveDiagram(key, next)

      if (!record) return { docs: { ...s.docs, [tabId]: next } }

      const kept = s.history[tabId] ?? NO_HISTORY
      return {
        docs: { ...s.docs, [tabId]: next },
        history: {
          ...s.history,
          [tabId]: { past: [...kept.past, held].slice(-HISTORY_DEPTH), future: [] },
        },
      }
    }),

  replace: (tabId, diagram) => get().change(tabId, () => diagram),

  commit: (tabId, before) =>
    set((s) => {
      if (s.docs[tabId] === before) return s

      const kept = s.history[tabId] ?? NO_HISTORY
      return {
        history: {
          ...s.history,
          [tabId]: { past: [...kept.past, before].slice(-HISTORY_DEPTH), future: [] },
        },
      }
    }),

  look: (tabId, edit) =>
    set((s) => ({ views: { ...s.views, [tabId]: { ...(s.views[tabId] ?? IDLE_VIEW), ...edit } } })),

  undo: (tabId) =>
    set((s) => {
      const kept = s.history[tabId] ?? NO_HISTORY
      const held = s.docs[tabId]
      const previous = kept.past.at(-1)
      if (!held || !previous) return s

      const key = s.keys[tabId]
      if (key) saveDiagram(key, previous)

      return {
        docs: { ...s.docs, [tabId]: previous },
        history: {
          ...s.history,
          [tabId]: { past: kept.past.slice(0, -1), future: [held, ...kept.future] },
        },
      }
    }),

  redo: (tabId) =>
    set((s) => {
      const kept = s.history[tabId] ?? NO_HISTORY
      const held = s.docs[tabId]
      const [next, ...rest] = kept.future
      if (!held || !next) return s

      const key = s.keys[tabId]
      if (key) saveDiagram(key, next)

      return {
        docs: { ...s.docs, [tabId]: next },
        history: { ...s.history, [tabId]: { past: [...kept.past, held], future: rest } },
      }
    }),

  discard: (tabId) =>
    set((s) => {
      const docs = { ...s.docs }
      const views = { ...s.views }
      const keys = { ...s.keys }
      const history = { ...s.history }
      delete docs[tabId]
      delete views[tabId]
      delete keys[tabId]
      delete history[tabId]
      return { docs, views, keys, history }
    }),
}))

/** The diagram a tab holds, or the empty one — stable, so a selector cannot loop the renderer. */
export const diagramOf = (docs: Record<string, Diagram>, tabId: string): Diagram =>
  docs[tabId] ?? EMPTY_DIAGRAM
