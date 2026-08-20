import { useState, type ReactNode } from 'react'
import { Check, Search, SlidersHorizontal, X } from 'lucide-react'

import { OBJECT_SEARCH_MODES } from '@/bridge/objects'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { MenuButton } from '@/shared/ui/MenuButton'
import { Checkbox, TextInput } from '@/shared/ui/Field'
import type { BucketSearch } from '@/stores/bucketViews'

import { sizeText } from './objectRows'

/**
 * A bucket is **searched, not filtered**, and this bar is where the Filter & Sort button would be —
 * which is the arrangement a [key space](../../../../docs/features/key-space.md) has, for a related
 * reason and not the same one.
 *
 * A key space's builder is left out because Redis narrows a read one way. Here the builder is left
 * out because there is *nothing to narrow*: a bucket has no columns, so twenty operators over six
 * facts would be twenty ways of asking about a name. What is left is the name, how it is matched,
 * and the three bounds a listing already carries — a size, a date, and nothing else.
 *
 * **It commits rather than narrowing as it is typed**, which is the other half of the difference: a
 * key space's box re-reads one page and this re-reads a bucket, so a letter that started a quarter
 * of a million requests would be a letter nobody could afford. Enter runs it, and the magnifier says
 * how the text is read.
 */
export function ObjectSearchBar({
  search,
  searching,
  scanned,
  onChange,
  onRun,
  onStop,
  onClear,
}: {
  search: BucketSearch
  searching: boolean
  /** What the last search cost, drawn beside the box so a slow one explains itself. */
  scanned: string
  onChange: (edit: Partial<BucketSearch>) => void
  onRun: () => void
  onStop: () => void
  onClear: () => void
}) {
  const [bounds, setBounds] = useState(false)
  const mode = OBJECT_SEARCH_MODES.find((one) => one.value === search.mode) ?? OBJECT_SEARCH_MODES[0]
  const bounded = search.minSize > 0 || search.maxSize > 0 || search.since !== '' || search.until !== ''

  return (
    <div className="shrink-0 border-b border-line bg-bar">
      <div className="flex h-8 items-center gap-1 px-1.5">
        <div className="flex h-6 min-w-0 flex-1 items-center gap-1 rounded-md border border-line bg-canvas px-1 transition-colors focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
          {/* The magnifier is the menu, because the same characters are four different questions
              and this is where they are typed — the arrangement a key space's own box has. */}
          <MenuButton
            items={OBJECT_SEARCH_MODES.map((one) => ({
              label: one.label,
              icon: one.value === search.mode ? <Check size={13} /> : undefined,
              onSelect: () => onChange({ mode: one.value }),
            }))}
          >
            <button
              type="button"
              title={`${mode.label} — ${mode.hint}`}
              aria-label="How the name is matched"
              className="grid h-5 w-5 shrink-0 place-items-center rounded text-faint transition-colors hover:bg-hover hover:text-fg"
            >
              <Search size={12} />
            </button>
          </MenuButton>

          <input
            value={search.text}
            onChange={(e) => onChange({ text: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === 'Enter') onRun()
              if (e.key === 'Escape') onClear()
            }}
            placeholder={
              mode.value === 'prefix' ? 'Start of the whole key…' : 'A file name, or any part of one'
            }
            spellCheck={false}
            className="min-w-0 flex-1 bg-transparent text-xs text-fg outline-none placeholder:text-faint"
          />

          {search.text !== '' ? (
            <button
              type="button"
              aria-label="Clear the search"
              onClick={onClear}
              className="grid shrink-0 place-items-center text-faint hover:text-fg"
            >
              <X size={12} />
            </button>
          ) : null}
        </div>

        <Checkbox
          label="Ignore case"
          checked={search.caseInsensitive}
          onChange={(e) => onChange({ caseInsensitive: e.target.checked })}
          // Keys are case sensitive at every service there is, so ignoring case is the one box here
          // that gives up the service's own narrowing — see `PruneTo`. Saying so is cheaper than
          // letting a reader wonder why the same search took a hundred times longer.
          title="Keys are case sensitive, so this is matched here rather than by the service — and reads every key under the folder."
        />

        <Button
          variant="subtle"
          icon={<SlidersHorizontal size={13} />}
          aria-pressed={bounds}
          onClick={() => setBounds((open) => !open)}
          className={cn(bounded && 'text-accent')}
        >
          Bounds
        </Button>

        {searching ? (
          <Button variant="danger" onClick={onStop}>
            Stop
          </Button>
        ) : (
          <Button variant="primary" disabled={search.text.trim() === ''} onClick={onRun}>
            Search
          </Button>
        )}

        {scanned !== '' ? (
          <span className="shrink-0 pl-1 font-mono text-2xs text-faint">{scanned}</span>
        ) : null}
      </div>

      {/* The three bounds. They are folded away because they are free but rarely wanted: every one
          of them is a fact the listing already carries, so none of them costs a request — which is
          exactly why they are here and not a second read. */}
      {bounds ? (
        <div className="flex flex-wrap items-center gap-2 border-t border-line px-2 py-1.5 text-xs">
          <Bound label="Larger than">
            <SizeBox
              bytes={search.minSize}
              onChange={(minSize) => onChange({ minSize })}
            />
          </Bound>
          <Bound label="Smaller than">
            <SizeBox
              bytes={search.maxSize}
              onChange={(maxSize) => onChange({ maxSize })}
            />
          </Bound>
          <Bound label="Written after">
            <DateBox value={search.since} onChange={(since) => onChange({ since })} />
          </Bound>
          <Bound label="Written before">
            <DateBox value={search.until} onChange={(until) => onChange({ until })} />
          </Bound>
        </div>
      ) : null}
    </div>
  )
}

function Bound({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex items-center gap-1.5">
      <span className="text-muted">{label}</span>
      {children}
    </label>
  )
}

/**
 * A size in the units a reader thinks in. What is typed is megabytes — the size a reader has in mind
 * when they say "the big ones" — and what is shown beside it is what that comes to, so nobody has to
 * trust the arithmetic.
 */
function SizeBox({ bytes, onChange }: { bytes: number; onChange: (bytes: number) => void }) {
  const megabytes = bytes > 0 ? String(Math.round((bytes / (1 << 20)) * 100) / 100) : ''

  return (
    <span className="flex items-center gap-1">
      <TextInput
        value={megabytes}
        inputMode="decimal"
        placeholder="—"
        className="h-6 w-20 text-xs"
        onChange={(e) => {
          const typed = Number(e.target.value)
          onChange(Number.isFinite(typed) && typed > 0 ? Math.round(typed * (1 << 20)) : 0)
        }}
      />
      <span className="w-14 font-mono text-2xs text-faint">
        {bytes > 0 ? sizeText(bytes) : 'MB'}
      </span>
    </span>
  )
}

/**
 * A date, as the host's own picker gives it. What goes over the bridge is RFC 3339, so midnight is
 * spelled out here rather than left for the Go side to guess at.
 */
function DateBox({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <TextInput
      type="date"
      value={value === '' ? '' : value.slice(0, 10)}
      className="h-6 w-32 text-xs"
      onChange={(e) => onChange(e.target.value === '' ? '' : `${e.target.value}T00:00:00Z`)}
    />
  )
}
