import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'
import type { MonitorPane } from '@/stores/tabs'

/** The four panes as a menu offers them, which is the same order the tab's own toolbar draws. */
const PANES: ReadonlyArray<{ pane: MonitorPane; label: string }> = [
  { pane: 'processes', label: 'Process List' },
  { pane: 'slow', label: 'Slow Queries' },
  { pane: 'status', label: 'Server Status' },
  { pane: 'variables', label: 'Server Variables' },
]

/**
 * The right-click on the `Server Monitor` row, and the item a connection's own menu carries. One item
 * per pane rather than one for the tab, because the pane is what the reader is actually after: a
 * reader reaching for `Server Variables` should not have to open the monitor and then find the pane.
 *
 * `hasProfiler` drops the Slow Queries item on an engine that records nothing, so the menu cannot
 * open a tab on a pane its own toolbar does not draw.
 *
 * There is no Refresh here. Nothing about this row is cached — the tab re-reads on its own clock — so a
 * Refresh in the tree would be a button with nothing behind it.
 */
export function monitorItems(
  profileId: string,
  hasProfiler: boolean,
  open: (profileId: string, pane?: MonitorPane) => void,
): MenuItem[] {
  const panes = hasProfiler ? PANES : PANES.filter((one) => one.pane !== 'slow')

  return panes.map(({ pane, label }, at) => ({
    label,
    ...(at === 0 ? { icon: ICON.open } : {}),
    onSelect: () => open(profileId, pane),
  }))
}
