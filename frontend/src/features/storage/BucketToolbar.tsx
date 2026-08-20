import { ListTree } from 'lucide-react'

import type { ObjectRow, ObjectRule, ObjectRules } from '@/bridge/objects'
import { Button } from '@/shared/ui/Button'
import { ICON } from '@/shared/ui/menuIcons'
import type { BucketView } from '@/stores/bucketViews'

import { describe, type ObjectActionSet } from './useObjectActions'

/**
 * What a reader does to what a bucket holds. Every button here calls the **same action set** the
 * right-click menu does (`useObjectActions`), so a button and a menu item cannot come to mean
 * different things — which matters most for the one that deletes.
 *
 * Whether an action has to be answered for, and whether it will be sent at all, are decided by
 * `domain` and travel with the connection as `ObjectRules`. So nothing in this file compares an
 * environment against anything or asks what read-only means, and an action a read-only connection
 * refuses is **left out** rather than greyed — the rule the tree's own menu follows.
 */
export function BucketToolbar({
  view,
  rules,
  picked,
  on,
  onRefresh,
  onUp,
  onFlatten,
}: {
  view: BucketView
  rules: ObjectRules
  picked: readonly ObjectRow[]
  on: ObjectActionSet
  onRefresh: () => void
  onUp: () => void
  onFlatten: (flat: boolean) => void
}) {
  const one = picked.length === 1 ? picked[0] : undefined

  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
    >
      <Button variant="subtle" icon={ICON.refresh} onClick={onRefresh}>
        Refresh
      </Button>
      <Button variant="subtle" icon={ICON.openFolder} disabled={view.prefix === ''} onClick={onUp}>
        Up
      </Button>

      <span className="mx-1 h-4 w-px bg-line" />

      <Button
        variant="subtle"
        icon={ICON.download}
        disabled={picked.length === 0}
        onClick={on.download}
      >
        Download
      </Button>

      {offered(rules.upload) ? (
        <>
          <Button variant="subtle" icon={ICON.upload} onClick={on.upload}>
            Upload
          </Button>
          <Button variant="subtle" icon={ICON.newFolder} onClick={on.newFolder}>
            New Folder
          </Button>
        </>
      ) : null}

      {offered(rules.delete) ? (
        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={picked.length === 0}
          onClick={on.remove}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete
        </Button>
      ) : null}

      <span className="mx-1 h-4 w-px bg-line" />

      <Button
        variant="subtle"
        icon={ICON.link}
        disabled={one === undefined || one.folder}
        title="Copy a link that reads this object without a credential, good for fifteen minutes"
        onClick={() => {
          if (one && !one.folder) on.copyLink(one)
        }}
      >
        Copy Link
      </Button>

      {/* Flatten is the same word a key space uses for the same idea, and it is the one control here
          that changes what the *service* is asked: no delimiter, so it answers with every key under
          the folder and no folders at all. */}
      <Button
        variant="subtle"
        icon={<ListTree size={13} />}
        aria-pressed={view.flat}
        onClick={() => onFlatten(!view.flat)}
        className={view.flat ? 'bg-hover text-fg' : undefined}
      >
        Flatten
      </Button>

      <span className="flex-1" />

      {picked.length > 0 ? (
        <span className="shrink-0 font-mono text-2xs text-faint">{describe(picked)}</span>
      ) : null}
    </div>
  )
}

/** An action a read-only connection refuses is not drawn — see the note on this component. */
function offered(rule: ObjectRule): boolean {
  return !rule.refused
}
