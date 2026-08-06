import type { ReactNode } from 'react'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Loader2, Play, Square } from 'lucide-react'

import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'

/**
 * The frame both transfer wizards live in: one question to a screen, a prompt saying what that
 * question is, and four ways to move between them.
 *
 * **It is a wizard and not one long form** because the answers depend on each other — what a
 * delimiter is worth asking about depends on the format, and what a field can be mapped onto
 * depends on the table — and because the reader is being asked to do something with a lot of rows
 * in it, which is exactly when being shown one decision at a time is worth the extra clicks.
 *
 * The two double arrows are what make that bearable for the reader who has done it before: `>>`
 * runs from wherever they are, so a second import of the same shape of file is Next, Next, `>>`.
 */
export interface WizardStep {
  /** The sentence at the top of the screen, which is the question this step asks. */
  prompt: string
  body: ReactNode
  /** Why the reader cannot move on yet, and absent when they can. Shown under the prompt. */
  blocked?: string | undefined
}

interface WizardProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  icon: ReactNode
  steps: WizardStep[]
  at: number
  onMove: (to: number) => void
  /** The label of the button that does the thing — `Start`. */
  startLabel: string
  running: boolean
  /** Set once the job has run, which is what turns Start into a second run rather than a first. */
  finished: boolean
  onStart: () => void
  onStop: () => void
  /** What the footer says on the left: how far the job got, in the one line there is room for. */
  summary?: string
  error?: string | null | undefined
  width: number
}

export function Wizard({
  open,
  onOpenChange,
  title,
  description,
  icon,
  steps,
  at,
  onMove,
  startLabel,
  running,
  finished,
  onStart,
  onStop,
  summary,
  error,
  width,
}: WizardProps) {
  const step = steps[Math.min(at, steps.length - 1)]
  const last = at >= steps.length - 1

  // A step nobody can leave is the whole point of the check: every step from here on would be asking
  // about an answer that does not exist yet, so `>>` stops here as surely as `Next` does.
  const blocked = Boolean(step?.blocked)
  const canAdvance = !blocked && !last && !running

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      icon={icon}
      width={width}
      footer={
        <>
          <span className="mr-auto text-xs text-muted">{summary ?? ''}</span>

          {/*
            All four navigate, so all four are `default` — a real border and full-strength text.
            `subtle` is what a *disabled* control looks like in this app, and a wizard whose whole
            job is Next, Next, Next must not draw its Next as though it could not be pressed.
            Being unavailable is then a visible difference rather than the resting state.
          */}
          <Button
            icon={<ChevronsLeft size={12} />}
            aria-label="First step"
            disabled={at === 0 || running}
            onClick={() => onMove(0)}
          />
          <Button
            icon={<ChevronLeft size={12} />}
            disabled={at === 0 || running}
            onClick={() => onMove(at - 1)}
          >
            Back
          </Button>
          <Button
            icon={<ChevronRight size={12} />}
            disabled={!canAdvance}
            onClick={() => onMove(at + 1)}
          >
            Next
          </Button>
          <Button
            icon={<ChevronsRight size={12} />}
            aria-label="Last step"
            disabled={!canAdvance}
            onClick={() => onMove(steps.length - 1)}
          />

          {running ? (
            <Button
              variant="danger"
              icon={<Square size={11} fill="currentColor" />}
              onClick={onStop}
            >
              Stop
            </Button>
          ) : null}

          <Button onClick={() => onOpenChange(false)}>
            {finished && !running ? 'Close' : 'Cancel'}
          </Button>
          <Button
            variant="primary"
            icon={
              running ? (
                <Loader2 size={12} className="animate-spin" />
              ) : (
                <Play size={12} fill="currentColor" />
              )
            }
            disabled={running || !last || blocked}
            onClick={onStart}
          >
            {startLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium text-accent">{step?.prompt}</p>
          <StepDots count={steps.length} at={at} />
        </div>

        {step?.blocked ? <Banner tone="warn" text={step.blocked} /> : null}
        {error ? <Banner tone="danger" text={error} /> : null}

        {step?.body}
      </div>
    </Dialog>
  )
}

/** How far through the wizard the reader is, in the least space that can say it. */
function StepDots({ count, at }: { count: number; at: number }) {
  return (
    <div className="flex items-center gap-1" aria-label={`Step ${at + 1} of ${count}`}>
      {Array.from({ length: count }, (_, step) => (
        <span
          key={step}
          className={
            step === at
              ? 'h-1 w-6 rounded-full bg-accent'
              : 'h-1 w-3 rounded-full bg-line-strong'
          }
        />
      ))}
    </div>
  )
}
