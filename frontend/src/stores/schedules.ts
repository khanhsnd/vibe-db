import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import { tasks as api, type Task, type TaskRequest } from '@/bridge/schedule'

/**
 * Every job on the clock, held once for the whole window.
 *
 * A store rather than a hook in the view, for the reason the snippet library is one: a job fires
 * whether or not anybody is looking at the list, and the event that says so has to land somewhere
 * that outlives the tab. It is also the one list in the app that changes **by itself** — see
 * `follow`, which is what keeps a window open at two in the morning honest.
 *
 * Every write takes the task the Go side answers with rather than re-reading, so there is no window
 * in which the two disagree — except a delete, which answers with nothing and re-reads.
 */
interface SchedulesState {
  tasks: Task[]
  loading: boolean
  error: AppError | null
  /** The job the dialog is editing, and non-null exactly while that dialog is up. */
  editing: TaskRequest | null

  load: () => Promise<void>
  /** Opens the dialog on a job — a new one, or one already saved. */
  edit: (task: TaskRequest) => void
  closeEditor: () => void

  /**
   * Keeps one. It **throws** on a rejection, because what asked is a dialog with the boxes still in
   * front of the reader — the same reason a snippet's save does.
   */
  save: (request: TaskRequest) => Promise<Task>
  remove: (id: string) => Promise<void>
  enable: (id: string, enabled: boolean) => Promise<void>
  /** Fires one now. The list is re-read afterwards: a run moves the job's next firing. */
  run: (id: string) => Promise<void>
}

export const useSchedulesStore = create<SchedulesState>((set, get) => ({
  tasks: [],
  loading: false,
  error: null,
  editing: null,

  load: async () => {
    set({ loading: true })
    try {
      set({ tasks: await api.list(), error: null })
    } catch (thrown) {
      set({ error: toAppError(thrown) })
    } finally {
      set({ loading: false })
    }
  },

  edit: (task) => set({ editing: task }),
  closeEditor: () => set({ editing: null }),

  save: async (request) => {
    const saved = await api.save(request)
    set((state) => ({ tasks: replace(state.tasks, saved), error: null }))
    return saved
  },

  remove: async (id) => {
    await api.remove(id)
    set((state) => ({ tasks: state.tasks.filter((task) => task.id !== id), error: null }))
  },

  enable: async (id, enabled) => {
    const saved = await api.enable(id, enabled)
    set((state) => ({ tasks: replace(state.tasks, saved), error: null }))
  },

  run: async (id) => {
    try {
      await api.run(id)
    } finally {
      // Whatever the run came to, the job now holds one more firing and a new next one — and a
      // refused Run (one already going) is exactly the case where the list is out of date here.
      await get().load()
    }
  },
}))

/** One task replaced where it stands, or added where it is new. The order of the file is kept. */
function replace(tasks: Task[], saved: Task): Task[] {
  return tasks.some((task) => task.id === saved.id)
    ? tasks.map((task) => (task.id === saved.id ? saved : task))
    : [...tasks, saved]
}
