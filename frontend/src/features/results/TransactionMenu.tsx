import { Check, GitBranch, Play, Undo2 } from 'lucide-react'

import { engineInfo, type Namespace } from '@/bridge/connections'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { MenuButton } from '@/shared/ui/MenuButton'
import { useTransactionsStore } from '@/stores/transactions'

interface TransactionMenuProps {
  sessionId: string
  /** Where the transaction is opened, so an unqualified name inside it resolves where the reader is. */
  namespace: Namespace
  /** Which engine this connection speaks, so an engine without transactions is not offered them. */
  engine: string
  /** Run after a Commit or a Rollback, for a surface whose rows are now about a different truth. */
  onSettled?: (() => void) | undefined
}

/**
 * The one way in to a connection's manual transaction, in one component and therefore in both
 * places a reader writes: the table tab's toolbar beside Cell Editor, and the query tab's beside
 * Execute — exactly as `CellEditorButton` and `ExportMenu` sit in two.
 *
 * It draws its state from the **session**, not from the tab, because that is what a transaction is:
 * one connection, held open, with every statement of every tab on that connection inside it. A
 * pressed button therefore means "this connection is not autocommitting", which is a stronger claim
 * than a toggle in one tab could make and the reason the label says so in words.
 *
 * An engine with no such notion is offered nothing — `EngineInfo.hasTransactions`, never a
 * comparison against an engine's name.
 */
export function TransactionMenu({
  sessionId,
  namespace,
  engine,
  onSettled,
}: TransactionMenuProps) {
  const open = useTransactionsStore((s) => s.open[sessionId] ?? false)
  const busy = useTransactionsStore((s) => s.busy[sessionId] ?? false)
  const begin = useTransactionsStore((s) => s.begin)
  const commit = useTransactionsStore((s) => s.commit)
  const rollback = useTransactionsStore((s) => s.rollback)

  if (!engineInfo(engine).hasTransactions) return null

  // Committing and rolling back both leave the rows on screen about a state that has moved on, so
  // the surface is told once either of them lands rather than per outcome.
  const end = (action: (id: string) => Promise<void>) => () => {
    void action(sessionId).then(() => onSettled?.())
  }

  return (
    <MenuButton
      items={[
        {
          label: 'Begin Transaction',
          icon: <Play size={13} />,
          disabled: open || busy,
          onSelect: () => void begin(sessionId, namespace),
        },
        {
          label: 'Commit',
          icon: <Check size={13} />,
          disabled: !open || busy,
          separatorBefore: true,
          onSelect: end(commit),
        },
        {
          label: 'Rollback',
          icon: <Undo2 size={13} />,
          disabled: !open || busy,
          danger: true,
          onSelect: end(rollback),
        },
      ]}
    >
      <Button
        variant="subtle"
        icon={<GitBranch size={14} className={open ? 'text-warn' : 'text-accent'} />}
        aria-pressed={open}
        title={
          open
            ? 'A transaction is open on this connection: nothing is committed until you say so.'
            : 'Autocommit. Every statement is true the moment it runs.'
        }
        className={cn(open && 'bg-selected text-fg')}
      >
        {open ? 'Transaction · Open' : 'Transaction'}
      </Button>
    </MenuButton>
  )
}
