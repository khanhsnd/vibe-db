import { ShieldAlert } from 'lucide-react'

import { environmentInfo } from '@/bridge/connections'
import { needsConfirming, type Hazard, type ScriptReview } from '@/bridge/query'
import { EnvironmentBadge } from '@/features/connections/EnvironmentDot'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'

/** What each hazard is, said as the thing that will happen rather than as a category. */
const HAZARD: Record<Hazard, string> = {
  '': '',
  write: 'changes rows',
  unfiltered: 'has no WHERE — every row',
  destructive: 'cannot be undone',
}

/**
 * The question asked before a statement that cannot be taken back is run. **Which** statements
 * are listed is not decided here: `confirm` is worked out on the Go side from the statement's own
 * hazard and the connection's environment, so what is warned about is one rule in one place — see
 * `domain.Environment.NeedsConfirmation`.
 *
 * There are two reasons a statement is here, and they are worth telling apart: a write on a
 * **production** connection, which is about *where* it is going; and an **unfiltered** UPDATE or
 * DELETE anywhere at all, which is about what it *is*.
 */
export function RunGuard({
  review,
  onCancel,
  onConfirm,
}: {
  /** Non-null exactly while the question is being asked. */
  review: ScriptReview | null
  onCancel: () => void
  onConfirm: () => void
}) {
  const guarded = review ? needsConfirming(review) : []
  const environment = environmentInfo(review?.environment ?? '')

  return (
    <Dialog
      open={review !== null}
      onOpenChange={(open) => {
        if (!open) onCancel()
      }}
      title="Run this?"
      description={
        environment.guarded
          ? `This connection is ${environment.label}.`
          : 'One of these statements is not narrowed by a WHERE.'
      }
      icon={<ShieldAlert size={15} />}
      tone="danger"
      width={560}
      footer={
        <>
          <div className="flex-1" />
          <Button variant="default" onClick={onCancel}>
            Cancel
          </Button>
          {/* The confirmation is a plain button rather than the accent one. The accent is what a
              reader presses without reading, and that is the whole habit this dialog exists to
              interrupt. */}
          <Button variant="danger" onClick={onConfirm}>
            Run {guarded.length === 1 ? 'It' : `All ${guarded.length}`}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <EnvironmentBadge environment={review?.environment ?? ''} />
          <span className="text-xs text-muted">
            {guarded.length === 1
              ? '1 statement has to be confirmed.'
              : `${guarded.length} statements have to be confirmed.`}
          </span>
        </div>

        {guarded.some((statement) => statement.hazard === 'unfiltered') ? (
          <Banner
            tone="danger"
            text="An UPDATE or DELETE here carries no WHERE, so it will change every row of its table."
          />
        ) : null}

        <ul className="scroll-thin flex max-h-64 flex-col gap-1.5 overflow-y-auto">
          {guarded.map((statement) => (
            <li
              key={statement.start}
              className="rounded-md border border-line bg-canvas px-2.5 py-1.5"
            >
              <div className="flex items-baseline gap-2">
                <span className="shrink-0 font-mono text-2xs text-warn uppercase">
                  {statement.keyword}
                </span>
                <span className="truncate text-2xs text-faint">{HAZARD[statement.hazard]}</span>
              </div>
              {/* Two lines and no more: the reader is checking *which* statement this is, and the
                  whole of it is in the editor behind the dialog. */}
              <p className="data-font mt-1 line-clamp-2 text-xs break-all whitespace-pre-wrap text-fg">
                {statement.sql}
              </p>
            </li>
          ))}
        </ul>
      </div>
    </Dialog>
  )
}
