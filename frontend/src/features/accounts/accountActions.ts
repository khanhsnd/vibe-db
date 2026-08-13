import type { AccountRef } from '@/bridge/accounts'
import type { TreeNode } from '@/features/connections/tree'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'

interface AccountHandlers {
  open: (profileId: string, account?: AccountRef) => void
  refresh: () => void
  remove: (account: AccountRef) => void
  /** What this engine calls an account — `User` on five of them, `Role` on PostgreSQL. */
  label: string
}

/**
 * The right-click on the `Users` folder and on the accounts under it. It is one function for both
 * rows because they offer the same three things with one addition, exactly as the backup folder and
 * a backup do.
 *
 * A `New …` is offered on the folder rather than on a row, and the row an account is *edited* on is
 * the editor in the tab: there is no dialog here, because an account is not one box — a password, a
 * handful of attributes and a matrix of privileges do not fit in one.
 */
export function accountItems(node: TreeNode, on: AccountHandlers): MenuItem[] {
  const refresh: MenuItem = {
    label: 'Refresh',
    icon: ICON.refresh,
    separatorBefore: true,
    onSelect: on.refresh,
  }

  if (node.kind === 'accounts') {
    return [
      {
        label: `New ${on.label}…`,
        icon: ICON.add,
        onSelect: () => on.open(node.profileId),
      },
      {
        label: `${on.label} List`,
        icon: ICON.open,
        onSelect: () => on.open(node.profileId),
      },
      refresh,
    ]
  }

  const account = node.account
  if (!account) return [refresh]

  return [
    {
      label: `Edit ${on.label}`,
      icon: ICON.design,
      onSelect: () => on.open(node.profileId, account),
    },
    {
      label: `New ${on.label}…`,
      icon: ICON.add,
      onSelect: () => on.open(node.profileId),
    },
    {
      label: `Delete ${on.label}`,
      icon: ICON.delete,
      separatorBefore: true,
      danger: true,
      writes: true,
      onSelect: () => on.remove(account),
    },
    refresh,
  ]
}
