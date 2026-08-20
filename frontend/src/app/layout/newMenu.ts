import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

/**
 * The two things this app can be pointed at: a **database** and an [object
 * store](../../../../docs/features/object-storage.md). It is one list in one place because it is
 * offered from three surfaces — the command bar, the sidebar's `+`, and the empty state a first run
 * opens on — and a pair spelled three times is a pair that will disagree with itself.
 *
 * It is a *menu* and not a second button beside the first. They are one gesture — "point the app at
 * something" — so a second glyph next to the first would be a second guess about which is which;
 * what a reader is choosing is the **kind of thing**, which is a choice and therefore a list.
 *
 * This existed as a menu on the sidebar alone at first, which was the wrong number of places: the
 * button a reader actually reaches for is the one that says `New Connection` in the command bar, and
 * a store reachable only from a 24-pixel `+` is a store nobody finds.
 */
export function newConnectionItems(
  onNewConnection: () => void,
  onNewStore: () => void,
): MenuItem[] {
  return [
    { label: 'New Connection…', icon: ICON.connect, onSelect: onNewConnection },
    { label: 'New Object Store…', icon: ICON.bucket, onSelect: onNewStore },
  ]
}
