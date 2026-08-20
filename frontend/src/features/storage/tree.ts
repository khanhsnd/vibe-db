import type { BucketRow, Store, StoreConnection } from '@/bridge/objects'
import type { ConnectionStatus, TreeNode } from '@/features/connections/tree'
import { bucketNodeId, storeNodeId } from '@/stores/objectStores'

/**
 * The object-store half of the sidebar's tree. It is a **second builder rather than a branch of
 * `buildTree`**, and the two never touch: a store has no databases, no schemas and no object
 * folders, so every field of that builder's `TreeInput` would have been unused here.
 *
 * What they share is the *output*, which is the whole point: one `TreeNode`, one `ObjectTree`, one
 * filter, one colour wash, one context menu. `App` concatenates the two lists and nothing below
 * knows there were two.
 *
 * **It is two levels deep and stops there.** A prefix inside a bucket is not a node: it is drawn in
 * the grid, as an [outlined column](../../../../docs/grid.md), for exactly the reason a Redis key
 * space's colons are — the folders come off the page the reader is already looking at and cost the
 * service nothing extra. A tree of prefixes would be a listing per branch, and a bucket has no
 * bottom.
 */
export interface StoreTreeInput {
  profiles: Store[]
  statuses: Record<string, ConnectionStatus>
  sessions: Record<string, StoreConnection>
  buckets: Record<string, BucketRow[]>
}

export function buildStoreTree(input: StoreTreeInput): TreeNode[] {
  return input.profiles.map((profile) => ({
    id: storeNodeId(profile.id),
    kind: 'store',
    label: profile.name,
    profileId: profile.id,
    provider: profile.provider,
    environment: profile.environment,
    colour: profile.colour,
    status: input.statuses[profile.id] ?? 'idle',
    // Where it points, which is the same sentence a database connection's badge makes: two stores
    // named alike differ only in their endpoint, and two on one endpoint only in their name.
    badge: profile.target,
    children: (input.buckets[profile.id] ?? []).map((bucket) => ({
      id: bucketNodeId(profile.id, bucket.name),
      kind: 'bucket',
      label: bucket.name,
      profileId: profile.id,
      bucket: bucket.name,
      // A bucket's region is a fact about the bucket and not a count, so it is drawn as one — faint
      // mono rather than a pill. See `isCount`.
      ...(bucket.region ? { badge: bucket.region } : {}),
    })),
  }))
}

/** Whether this row is an object store's, which is what tells the two halves of the tree apart. */
export function isStoreNode(node: TreeNode): boolean {
  return node.kind === 'store' || node.kind === 'bucket'
}
