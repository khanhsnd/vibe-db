import { useEffect } from 'react'
import { ChevronDown, ChevronUp, Download, FolderOpen, Upload, X } from 'lucide-react'

import { finished, objects, type TransferJob } from '@/bridge/objects'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { IconButton } from '@/shared/ui/IconButton'
import { useTransfersStore } from '@/stores/transfers'

import { rateText, remainingText, sizeText } from './objectRows'

/**
 * Every transfer this app has been asked for, as a dock at the foot of the work area.
 *
 * It belongs to the **window** and not to a tab, and that is the first thing a reader finds out by
 * closing the tab they started a download from: the bytes are still moving. Nothing here polls —
 * the report is pushed while anything is going and read once on open — and nothing here holds a
 * byte: what crosses the bridge is how far each job has got.
 */
export function TransferPanel() {
  const report = useTransfersStore((s) => s.report)
  const shown = useTransfersStore((s) => s.shown)
  const unseen = useTransfersStore((s) => s.unseen)
  const show = useTransfersStore((s) => s.show)
  const watch = useTransfersStore((s) => s.watch)
  const refresh = useTransfersStore((s) => s.refresh)
  const cancel = useTransfersStore((s) => s.cancel)
  const cancelAll = useTransfersStore((s) => s.cancelAll)
  const clear = useTransfersStore((s) => s.clear)

  useEffect(() => {
    watch()
  }, [watch])

  useEffect(() => {
    if (shown) void refresh()
  }, [shown, refresh])

  const moving = report.jobs.filter((job) => !finished(job.state))

  // Shut, with something to say. It is a **strip and not nothing**, because the alternative is a
  // download running with no way back to it: the panel is where a transfer is stopped, and a reader
  // who closed it has not cancelled anything.
  if (!shown) {
    if (report.jobs.length === 0) return null

    return (
      <button
        type="button"
        onClick={() => show(true)}
        className="flex h-6 shrink-0 items-center gap-2 border-t border-line bg-bar px-2 text-2xs text-muted transition-colors hover:text-fg"
      >
        <ChevronUp size={12} className="shrink-0" />
        <span>
          {moving.length > 0
            ? `${moving.length} transfer${moving.length === 1 ? '' : 's'} going`
            : 'Transfers'}
        </span>
        {moving.length > 0 ? (
          <span className="font-mono">
            {sizeText(moving.reduce((total, job) => total + job.moved, 0))} moved ·{' '}
            {rateText(moving.reduce((total, job) => total + job.rate, 0))}
          </span>
        ) : null}
        {unseen > 0 ? (
          <span className="rounded-full bg-accent px-1.5 font-mono text-on-accent">{unseen}</span>
        ) : null}
      </button>
    )
  }

  return (
    <div className="flex h-56 shrink-0 flex-col border-t border-line bg-panel">
      <div className="flex h-7 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5">
        <span className="flex-1 text-xs font-medium">
          Transfers
          {moving.length > 0 ? (
            <span className="pl-1.5 font-mono text-2xs text-muted">{moving.length} going</span>
          ) : null}
        </span>

        {moving.length > 0 ? (
          <Button variant="subtle" onClick={() => void cancelAll()}>
            Stop All
          </Button>
        ) : null}
        <Button
          variant="subtle"
          disabled={report.jobs.length === moving.length}
          onClick={() => void clear()}
        >
          Clear Finished
        </Button>
        <IconButton label="Hide transfers" icon={<ChevronDown size={13} />} onClick={() => show(false)} />
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {report.jobs.length === 0 ? (
          <Empty
            icon={<Download size={22} strokeWidth={1.5} />}
            title="Nothing moving"
            hint="Download or Upload from a bucket puts a job here."
          />
        ) : (
          <ul className="divide-y divide-line">
            {report.jobs.map((job) => (
              <JobRow
                key={job.id}
                job={job}
                /** The file this job has in flight, of however many the queue is moving at once. */
                file={report.files.find((one) => one.jobId === job.id) ?? null}
                onCancel={() => void cancel(job.id)}
                onReveal={() => void objects.revealFolder(job.root)}
              />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function JobRow({
  job,
  file,
  onCancel,
  onReveal,
}: {
  job: TransferJob
  file: { key: string; moved: number; size: number } | null
  onCancel: () => void
  onReveal: () => void
}) {
  const over = finished(job.state)

  return (
    <li className="flex flex-col gap-1 px-2 py-1.5">
      <div className="flex min-w-0 items-center gap-1.5">
        {job.kind === 'upload' ? (
          <Upload size={13} className="shrink-0 text-kind-object" />
        ) : (
          <Download size={13} className="shrink-0 text-kind-object" />
        )}

        <span className="min-w-0 flex-1 truncate text-xs">{job.label}</span>
        <span className="shrink-0 font-mono text-2xs text-muted">{state(job)}</span>

        {!over ? (
          <IconButton label="Stop" icon={<X size={12} />} onClick={onCancel} />
        ) : job.kind === 'download' ? (
          <IconButton label="Open the folder" icon={<FolderOpen size={12} />} onClick={onReveal} />
        ) : null}
      </div>

      {/* A bar is drawn **only** where the total is known. While the listing that finds the files is
          still running, `counting` is true and what is drawn is a count — a bar from a climbing
          total is a bar that goes backwards, which is worse than no bar at all. */}
      {!over ? (
        job.counting ? (
          <p className="font-mono text-2xs text-faint">
            {job.files.toLocaleString()} found so far · {sizeText(job.moved)} moved ·{' '}
            {rateText(job.rate)}
          </p>
        ) : (
          <>
            <div className="h-1 overflow-hidden rounded-full bg-active">
              <div
                className="h-full rounded-full bg-accent transition-[width] duration-300"
                style={{ width: `${percent(job)}%` }}
              />
            </div>
            <p className="flex items-center gap-2 font-mono text-2xs text-faint">
              <span>
                {job.done + job.skipped}/{job.files} · {sizeText(job.moved)} of{' '}
                {sizeText(job.bytes)}
              </span>
              <span>{rateText(job.rate)}</span>
              <span>{remainingText(job.moved, job.bytes, job.rate, job.counting)}</span>
            </p>
          </>
        )
      ) : (
        <p className={cn('text-2xs', job.state === 'done' ? 'text-muted' : 'text-danger')}>
          {outcome(job)}
        </p>
      )}

      {/* The one file in flight, named, because a job of ten thousand files that says only "43%"
          gives a reader nothing to recognise when it stops on one of them. */}
      {file !== null ? (
        <p className="truncate font-mono text-2xs text-faint">
          {file.key}
          {file.size > 0 ? ` · ${sizeText(file.moved)} of ${sizeText(file.size)}` : ''}
        </p>
      ) : null}
    </li>
  )
}

function percent(job: TransferJob): number {
  if (job.bytes <= 0) return job.files > 0 ? ((job.done + job.skipped) / job.files) * 100 : 0
  return Math.min(100, (job.moved / job.bytes) * 100)
}

function state(job: TransferJob): string {
  switch (job.state) {
    case 'running':
      return job.counting ? 'listing' : 'moving'
    case 'waiting':
      return 'waiting'
    default:
      return job.state
  }
}

/**
 * What a finished job came to, and there are four sentences because there are four outcomes — the
 * one that matters most being **partial**: some of it went, and a reader has to be able to see that
 * without opening anything.
 */
function outcome(job: TransferJob): string {
  const kept = job.skipped > 0 ? `, ${job.skipped} left alone` : ''

  switch (job.state) {
    case 'done':
      return `${job.done} of ${job.files}${kept} · ${sizeText(job.moved)}`
    case 'partial':
      return `${job.done} of ${job.files} done, ${job.failed} failed${kept} — ${job.detail}`
    case 'cancelled':
      return `Stopped after ${job.done} of ${job.files}. What was written is still there.`
    default:
      return job.detail === '' ? 'Failed' : job.detail
  }
}
