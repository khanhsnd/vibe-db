import { ChevronRight } from 'lucide-react'

import { cn } from '@/lib/cn'

import { OBJECT_SEPARATOR } from './objectRows'

/**
 * Where in the bucket the reader is standing, drawn as the path it is. Every part of it is a button,
 * so walking back out of six folders is one press rather than six.
 *
 * It is the tab's **only** address bar. A prefix is not a node of the sidebar — a bucket's folders
 * come off the page the reader is looking at rather than from a listing per branch — so this is
 * where the depth is shown, and it is the whole of why the tree stops at a bucket.
 */
export function BucketBreadcrumb({
  bucket,
  prefix,
  onGoTo,
}: {
  bucket: string
  prefix: string
  onGoTo: (prefix: string) => void
}) {
  const parts = prefix.split(OBJECT_SEPARATOR).filter((part) => part !== '')

  return (
    <nav
      aria-label="Path"
      className="flex h-7 min-w-0 shrink-0 items-center gap-0.5 overflow-x-auto px-1.5 text-xs"
    >
      <Crumb label={bucket} muted={parts.length > 0} onClick={() => onGoTo('')} />
      {parts.map((part, at) => (
        <span key={`${part}-${at}`} className="flex min-w-0 items-center gap-0.5">
          <ChevronRight size={11} className="shrink-0 text-faint" />
          <Crumb
            label={part}
            muted={at < parts.length - 1}
            onClick={() =>
              onGoTo(parts.slice(0, at + 1).join(OBJECT_SEPARATOR) + OBJECT_SEPARATOR)
            }
          />
        </span>
      ))}
    </nav>
  )
}

function Crumb({
  label,
  muted,
  onClick,
}: {
  label: string
  muted: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'max-w-[220px] shrink-0 truncate rounded px-1 py-0.5 transition-colors hover:bg-hover',
        muted ? 'text-muted' : 'font-medium text-fg',
      )}
    >
      {label}
    </button>
  )
}
