import { useState } from 'react'
import { ChevronRight, TextCursorInput } from 'lucide-react'

import { cn } from '@/lib/cn'
import { IconButton } from '@/shared/ui/IconButton'
import { TextInput } from '@/shared/ui/Field'

import { OBJECT_SEPARATOR } from './objectRows'

/**
 * Where in the bucket the reader is standing, drawn as the path it is. Every part of it is a button,
 * so walking back out of six folders is one press rather than six.
 *
 * It is the tab's **only** address bar. A prefix is not a node of the sidebar — a bucket's folders
 * come off the page the reader is looking at rather than from a listing per branch — so this is
 * where the depth is shown, and it is the whole of why the tree stops at a bucket.
 *
 * **It is typed as well as pressed**, and the toggle is what says which. A path is what a bucket has
 * instead of a tree, so a reader who already knows theirs would otherwise have to walk down to it a
 * folder and a listing at a time — which on a root holding thousands of loose objects is several
 * seconds of reading to reach somewhere they could have spelled. The two modes are one control
 * rather than a box beside the crumbs, because they are the same fact written two ways and a reader
 * should never be looking at a stale copy of the other.
 *
 * Typing one is **not** a search: a prefix is where a listing starts, so the path is read as the
 * folder it names and the reader lands there whether or not anything is under it — an empty folder
 * and a folder that was never there are the same answer from a service, and pretending to tell them
 * apart would mean a second request to find out nothing.
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
  /** The path being typed, or null while the crumbs are what is drawn. */
  const [draft, setDraft] = useState<string | null>(null)
  const parts = prefix.split(OBJECT_SEPARATOR).filter((part) => part !== '')

  const type = () => setDraft(prefix)
  const go = () => {
    if (draft !== null) onGoTo(folderPath(draft))
    setDraft(null)
  }

  return (
    <div className="flex h-7 min-w-0 flex-1 items-center gap-1">
      {draft === null ? (
        <nav
          aria-label="Path"
          className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto px-1.5 text-xs"
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
      ) : (
        <TextInput
          // **Enter is the only thing that goes anywhere.** A half-typed prefix is a folder nobody
          // has and reading one costs a request, so nothing commits on a click landing elsewhere —
          // Escape and the toggle both put the crumbs back without moving. The text opens selected,
          // because a reader who reached for this box has a path in mind rather than an edit.
          autoFocus
          aria-label="Path"
          placeholder={`A path inside ${bucket}, such as templateFiles/user/112`}
          value={draft}
          onFocus={(event) => event.target.select()}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') go()
            else if (event.key === 'Escape') setDraft(null)
          }}
          className="h-6 min-w-0 flex-1 font-mono text-xs"
        />
      )}

      <IconButton
        label={draft === null ? 'Type a path' : 'Show the path as folders'}
        icon={<TextCursorInput size={13} />}
        aria-pressed={draft !== null}
        onClick={() => (draft === null ? type() : setDraft(null))}
        className={cn(draft !== null && 'bg-selected text-fg')}
      />
    </div>
  )
}

/**
 * A typed path as the prefix a listing takes. A prefix is a **folder** here, so it ends in the
 * separator and never begins with one — and the empty string is the top of the bucket, which is what
 * a reader who cleared the box asked for.
 *
 * Repeated separators are collapsed because they are what a paste of two joined paths leaves behind,
 * and a key holding an empty segment is one no console can make. Nothing else is corrected: a name
 * is the service's and this app has never known better than a reader what one of theirs is called.
 */
function folderPath(typed: string): string {
  const parts = typed.trim().split(OBJECT_SEPARATOR).filter((part) => part !== '')
  return parts.length === 0 ? '' : parts.join(OBJECT_SEPARATOR) + OBJECT_SEPARATOR
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
