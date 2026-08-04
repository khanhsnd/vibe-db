import { AlertTriangle, ShieldAlert } from 'lucide-react'

import { environmentInfo } from '@/bridge/connections'
import { needsConfirming, type ScriptReview } from '@/bridge/query'

/**
 * The strip between the toolbar and the editor. It says two things and only when there is
 * something to say: what is **wrong** with the script, and what running it would **do** that the
 * connection's environment says has to be answered for.
 *
 * It is one line each and it never grows: the text itself is where a fault is located — the review
 * underlines it — and a list of twenty repeated messages above the editor would push the editor
 * off the screen at exactly the moment the reader wants to look at it.
 */
export function ScriptProblems({
  review,
  onReveal,
}: {
  review: ScriptReview
  /** Puts the caret on a statement or a fault, which is what makes the strip worth clicking. */
  onReveal: (offset: number) => void
}) {
  const [problem] = review.problems
  const guarded = needsConfirming(review)

  if (!problem && guarded.length === 0) return null

  const environment = environmentInfo(review.environment)

  return (
    <div className="flex shrink-0 flex-col border-b border-line bg-bar">
      {problem ? (
        <button
          type="button"
          onClick={() => onReveal(problem.start)}
          className="flex items-center gap-1.5 px-2 py-1 text-left text-xs text-danger hover:bg-hover"
        >
          <AlertTriangle size={12} className="shrink-0" />
          <span className="truncate">{problem.message}</span>
          {review.problems.length > 1 ? (
            <span className="shrink-0 text-faint">+{review.problems.length - 1} more</span>
          ) : null}
        </button>
      ) : null}

      {guarded.length > 0 ? (
        <button
          type="button"
          onClick={() => guarded[0] && onReveal(guarded[0].start)}
          className="flex items-center gap-1.5 px-2 py-1 text-left text-xs text-warn hover:bg-hover"
        >
          <ShieldAlert size={12} className="shrink-0" />
          <span className="truncate">
            {summarise(guarded.length)}
            {environment.value === '' ? '' : ` on ${environment.label}`}. Execute will ask first.
          </span>
        </button>
      ) : null}
    </div>
  )
}

function summarise(count: number): string {
  return count === 1 ? '1 statement here changes data' : `${count} statements here change data`
}
