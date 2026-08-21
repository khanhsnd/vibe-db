import { OnFileDrop } from '@wailsjs/runtime/runtime'

/**
 * A file dragged onto the window, as the **path** of what was let go and never its bytes. That is the
 * whole reason a drop can be an upload: `internal/objectstore` opens the file itself, so dropping a
 * forty-gigabyte archive costs exactly what picking it in a dialog costs, and nothing on this side
 * ever holds a `File`.
 *
 * There is **one** registration and **one** handler, and both are the runtime's shape rather than a
 * choice of ours: Wails adds its window listeners once and refuses a second `OnFileDrop`, while
 * `OnFileDropOff` would take them away from everybody. Only the tab in front is mounted, so there is
 * never a second view wanting a drop.
 *
 * `startFileDrop` runs at startup whether or not anything is listening, because those listeners are
 * also what stop the webview doing what a browser does with a dropped file — navigate to it, taking
 * the app off the screen. That is not the bucket view's to switch on.
 */
type FileDropHandler = (paths: string[]) => void

let handler: FileDropHandler | null = null
let listening = false

export function startFileDrop(): void {
  // The runtime is injected by the host rather than imported, so it is asked about and not typed —
  // under a plain `pnpm dev` there is no Wails and an unguarded call would take the window down.
  if (listening || !('runtime' in globalThis)) return
  listening = true

  // The `true` is drop-target mode: a drop is delivered only where it landed inside an element the
  // `.drop-zone` class marks, so a view says it takes files by drawing one and nothing here listens
  // for a drag. A path the host could not resolve arrives empty and is dropped on the floor.
  OnFileDrop((_x, _y, paths) => {
    const real = paths.filter((path) => path !== '')
    if (real.length > 0) handler?.(real)
  }, true)
}

/** The one slot. Subscribing replaces whoever held it; the returned function gives it up. */
export function onFileDrop(handle: FileDropHandler): () => void {
  startFileDrop()
  handler = handle
  return () => {
    if (handler === handle) handler = null
  }
}
