import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import { snippets, type Snippet, type SnippetInput } from '@/bridge/snippets'

/**
 * The statements the reader has named, held once for the whole window.
 *
 * It is a store rather than a hook per pane — which is what the History pane is — because the library
 * is read in **two places at once**: the Snippets pane lists it, and every open editor offers it in
 * completion. Two readers of one file would mean a snippet saved in the pane not reaching the editor
 * beside it until something re-read, and a reader whose completion offered a name they had just
 * renamed.
 *
 * That is also why every write here takes the list the Go side answers with rather than re-reading:
 * one round trip, and no window in which the two disagree.
 */
interface SnippetsState {
  library: Snippet[]
  loading: boolean
  error: AppError | null

  /** Reads the library. Called once when the window opens, and again by the pane's Refresh. */
  load: () => Promise<void>
  /**
   * Keeps one. It **throws** on a rejection, because what asked is a dialog with the boxes still in
   * front of the reader — the same reason a schema change does; see `changeSchema`.
   */
  save: (input: SnippetInput) => Promise<void>
  remove: (name: string) => Promise<void>
}

export const useSnippetsStore = create<SnippetsState>((set) => ({
  library: [],
  loading: false,
  error: null,

  load: async () => {
    set({ loading: true })
    try {
      set({ library: await snippets.list(), error: null })
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    } finally {
      set({ loading: false })
    }
  },

  save: async (input) => {
    set({ library: await snippets.save(input), error: null })
  },

  remove: async (name) => {
    set({ library: await snippets.remove(name), error: null })
  },
}))

/**
 * The snippets one editor may offer: the ones tagged with this engine, and the ones tagged with none.
 *
 * It is not a comparison against an engine's *capabilities* — which is the thing a component may
 * never do — but against the tag the snippet itself carries, exactly as the History pane narrows the
 * log by the `profileId` on each entry.
 */
export function snippetsFor(library: Snippet[], engine: string): Snippet[] {
  return library.filter((one) => one.engine === '' || one.engine === engine)
}
