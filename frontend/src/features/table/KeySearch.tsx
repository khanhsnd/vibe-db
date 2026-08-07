import { Check, ChevronDown, Search } from 'lucide-react'

import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { MenuButton } from '@/shared/ui/MenuButton'
import { cn } from '@/lib/cn'

import { KEY_MATCHES, KEY_TYPES, type KeySearch as Search_ } from './keySpace'

interface KeySearchProps {
  search: Search_
  onChange: (search: Search_) => void
}

/**
 * How a key space is narrowed: the type, and a fragment of the key's name. It stands where Filter &
 * Sort stands on every other table and replaces it outright, because the twenty operators of the
 * builder are twenty ways of saying something a key space cannot answer — Redis narrows a read one
 * way, a glob against the key's name, and has no opinion at all about a value, a clock or a cost.
 *
 * What it writes is still the ordinary condition tree (see `writeSearch`), so the read that follows is
 * the same read the builder would have asked for and the driver is unchanged by any of this.
 */
export function KeySearch({ search, onChange }: KeySearchProps) {
  const picked = KEY_MATCHES.find((one) => one.match === search.match) ?? KEY_MATCHES[0]!

  return (
    <>
      <select
        aria-label="Key type"
        value={search.type}
        onChange={(event) => onChange({ ...search, type: event.target.value })}
        className={cn(TOOLBAR_CONTROL, 'w-28 shrink-0')}
      >
        <option value="">All Types</option>
        {KEY_TYPES.map((type) => (
          <option key={type} value={type}>
            {type}
          </option>
        ))}
      </select>

      {/* One box, and it searches the *name*: it is the only thing the server will narrow by, so a
          box that offered to search values would be promising a scan of every value on the server.
          The glyph in front of it is the picker for how that name is read, which is where a mode
          belongs — the same three characters mean three things and the box is where they are typed. */}
      <div className="relative w-56 shrink-0">
        <MenuButton
          items={KEY_MATCHES.map((one) => ({
            label: one.label,
            icon: one.match === picked.match ? <Check size={13} /> : undefined,
            onSelect: () => onChange({ ...search, match: one.match }),
          }))}
        >
          <button
            type="button"
            aria-label={`How keys are matched: ${picked.label}`}
            title={picked.label}
            className={cn(
              'absolute top-1/2 left-0.5 flex h-5 -translate-y-1/2 items-center gap-px rounded px-1',
              'text-faint transition-colors hover:bg-hover hover:text-fg',
            )}
          >
            <Search size={12} />
            <ChevronDown size={9} />
          </button>
        </MenuButton>

        <input
          type="search"
          spellCheck={false}
          placeholder={picked.placeholder}
          value={search.text}
          onChange={(event) => onChange({ ...search, text: event.target.value })}
          className={cn(TOOLBAR_CONTROL, 'w-full pl-8')}
        />
      </div>
    </>
  )
}
