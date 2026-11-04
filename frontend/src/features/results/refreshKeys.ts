import { useEffect, useRef } from 'react'

/**
 * `F5` and `Ctrl`+`R` (`⌘`+`R`) press the Refresh that is in front, the way they do in every
 * browser and every database client. In a WebView both would otherwise **reload the app**, which
 * throws away every open tab's rows and every edit not yet applied, so the default is refused for
 * both keys wherever focus is, whether or not there is a Refresh to press.
 *
 * Only the active tab is mounted, and only one pane of a query tab's output, so what is registered
 * is almost always one footer. Where it is more, the most recently mounted wins, since that is the
 * one that has just come into view.
 *
 * A query tab keeps `F5` for **Execute**, its meaning in every SQL editor, and says so with
 * `useClaimF5`. There, `Ctrl`+`R` is still Refresh: it replays the statement behind the rows.
 */

interface Target {
  refresh: () => void
  busy: boolean
}

const targets: { current: Target }[] = []
let f5Claims = 0
let installed = false

function isRefreshKey(event: KeyboardEvent): 'f5' | 'r' | null {
  if (event.key === 'F5') return 'f5'
  if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'r') return 'r'
  return null
}

/** Inside an open dialog the grid behind it is not what the reader is looking at. */
function inDialog(): boolean {
  const focused = document.activeElement
  if (!(focused instanceof Element)) return false
  return focused.closest('dialog, [role="dialog"], [aria-modal="true"]') !== null
}

function onKeyDown(event: KeyboardEvent) {
  const key = isRefreshKey(event)
  if (key === null) return

  event.preventDefault()
  if (event.repeat) return
  if (key === 'f5' && f5Claims > 0) return
  if (inDialog()) return

  const target = targets[targets.length - 1]?.current
  if (target === undefined || target.busy) return
  target.refresh()
}

/** Called once at startup, so the WebView never reloads itself on either key. */
export function installRefreshKeys() {
  if (installed) return
  installed = true
  window.addEventListener('keydown', onKeyDown)
}

/** Makes `refresh` what `F5` and `Ctrl`+`R` press while this component is mounted. */
export function useRefreshKeys(refresh: () => void, busy: boolean) {
  const latest = useRef<Target>({ refresh, busy })
  latest.current = { refresh, busy }

  useEffect(() => {
    const entry = latest
    targets.push(entry)
    return () => {
      const at = targets.lastIndexOf(entry)
      if (at >= 0) targets.splice(at, 1)
    }
  }, [])
}

/** Keeps `F5` for the caller's own use (Execute) while it is mounted; `Ctrl`+`R` still refreshes. */
export function useClaimF5() {
  useEffect(() => {
    f5Claims += 1
    return () => {
      f5Claims -= 1
    }
  }, [])
}
