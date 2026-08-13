import type { KeyboardEvent } from 'react'

/** What an editor panel calls its Apply, so the keystroke is discoverable from the button itself. */
export const APPLY_HINT = 'Apply (Ctrl+S)'

/**
 * Ctrl+S on an editor panel, which does exactly what that panel's Apply button does and nothing
 * else. It is bound **on the panel** rather than on the window, and that is the whole design: the
 * grid above it has an Apply of its own that writes to the server, so one keystroke that could mean
 * either would be a write nobody asked for. Bound here it fires where the reader is typing, which
 * is inside the panel, and it stages the value exactly as the button does.
 *
 * The default is stopped whether or not there is anything to apply, because the WebView's own Ctrl+S
 * is a Save As dialog and a panel with nothing staged is not a reason to raise one.
 */
export function onApplyKey(event: KeyboardEvent, apply: (() => void) | undefined) {
  if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== 's') return

  event.preventDefault()
  apply?.()
}
