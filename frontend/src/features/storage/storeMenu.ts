import { CONNECTION_COLOURS } from '@/bridge/connections'
import { objects, type Store, type StoreConnection } from '@/bridge/objects'
import { colourSwatch } from '@/features/connections/ConnectionColour'
import type { TreeNode } from '@/features/connections/tree'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

/**
 * What a right-click on an object store's row offers. It is a **second builder beside**
 * `useTreeMenu` rather than more branches inside it, for the reason `buildStoreTree` is a second
 * builder: not one of that file's twenty kinds is a kind here, and every item there names a
 * namespace or a table.
 *
 * What the two share is the *shape* — a `MenuItem` — and one rule, which is the important one:
 * **an item that writes is left out on a read-only connection** rather than greyed. The connection
 * says which those are (`ObjectRules.refused`), decided by `domain` and travelling with the session,
 * so nothing here re-derives it.
 */
export interface StoreMenuHandlers {
  onConnect: (profileId: string) => void
  onDisconnect: (profileId: string) => void
  onEdit: (profileId: string) => void
  onRefresh: (profileId: string) => void
  onColour: (profileId: string, colour: string) => void
  onRename: (profileId: string) => void
  onDelete: (profileId: string) => void
  onOpenBucket: (profileId: string, bucket: string) => void
  onNewBucket: (profileId: string) => void
  onDeleteBucket: (profileId: string, bucket: string) => void
  onEmptyBucket: (profileId: string, bucket: string) => void
}

export function storeMenuItems(
  node: TreeNode,
  session: StoreConnection | null,
  on: StoreMenuHandlers,
): MenuItem[] {
  if (node.kind === 'store') return connectionItems(node, session, on)
  if (node.kind === 'bucket') return bucketItems(node, session, on)
  return []
}

function connectionItems(
  node: TreeNode,
  session: StoreConnection | null,
  on: StoreMenuHandlers,
): MenuItem[] {
  const open = session !== null
  const items: MenuItem[] = []

  items.push(
    open
      ? {
          label: 'Disconnect',
          icon: ICON.disconnect,
          onSelect: () => on.onDisconnect(node.profileId),
        }
      : { label: 'Connect', icon: ICON.connect, onSelect: () => on.onConnect(node.profileId) },
  )

  if (open) {
    items.push({
      label: 'Refresh Buckets',
      icon: ICON.refresh,
      onSelect: () => on.onRefresh(node.profileId),
    })

    // A new bucket is the one action here that is offered on a *connection* rather than on a bucket,
    // and it is left out where the connection refuses a write.
    if (session.rules.createBucket.refused !== true) {
      items.push({
        label: 'New Bucket…',
        icon: ICON.bucket,
        separatorBefore: true,
        writes: true,
        onSelect: () => on.onNewBucket(node.profileId),
      })
    }
  }

  items.push(
    {
      label: 'Edit Connection…',
      icon: ICON.editConnection,
      separatorBefore: true,
      onSelect: () => on.onEdit(node.profileId),
    },
    { label: 'Rename…', icon: ICON.rename, onSelect: () => on.onRename(node.profileId) },
    {
      label: 'Colour',
      icon: ICON.colour,
      items: CONNECTION_COLOURS.map((one) => ({
        label: one.label,
        icon: colourSwatch(one.value, node.colour === one.value),
        onSelect: () => on.onColour(node.profileId, one.value),
      })),
    },
    {
      label: 'Delete Connection',
      icon: ICON.delete,
      danger: true,
      separatorBefore: true,
      onSelect: () => on.onDelete(node.profileId),
    },
  )

  return items
}

function bucketItems(
  node: TreeNode,
  session: StoreConnection | null,
  on: StoreMenuHandlers,
): MenuItem[] {
  const bucket = node.bucket ?? node.label
  const items: MenuItem[] = [
    {
      label: 'Open Bucket',
      icon: ICON.open,
      onSelect: () => on.onOpenBucket(node.profileId, bucket),
    },
  ]

  if (session === null) return items

  // The two that name **nothing**: emptying a bucket and dropping one take everything in it, however
  // much that turns out to be, which is why `domain.ObjectHazard` calls them unfiltered.
  if (session.rules.emptyBucket.refused !== true) {
    items.push({
      label: 'Empty Bucket…',
      icon: ICON.empty,
      danger: true,
      separatorBefore: true,
      writes: true,
      onSelect: () => on.onEmptyBucket(node.profileId, bucket),
    })
  }
  if (session.rules.deleteBucket.refused !== true) {
    items.push({
      label: 'Delete Bucket…',
      icon: ICON.delete,
      danger: true,
      writes: true,
      onSelect: () => on.onDeleteBucket(node.profileId, bucket),
    })
  }

  return items
}

/** Copies a signed link for one object, which is what Copy Link means wherever it is offered. */
export async function signedLink(
  sessionId: string,
  bucket: string,
  key: string,
  seconds: number,
): Promise<string> {
  return objects.presign(sessionId, bucket, key, seconds)
}

/** The profile a store row stands for, which every handler above is given the id of. */
export function storeOf(profiles: readonly Store[], profileId: string): Store | undefined {
  return profiles.find((one) => one.id === profileId)
}
