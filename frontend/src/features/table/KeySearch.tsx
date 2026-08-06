import { Search } from 'lucide-react'

import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { cn } from '@/lib/cn'

import { KEY_TYPES, type KeySearch as Search_ } from './keySpace'

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
          box that offered to search values would be promising a scan of every value on the server. */}
      <div className="relative w-56 shrink-0">
        <Search
          size={12}
          aria-hidden
          className="pointer-events-none absolute top-1/2 left-1.5 -translate-y-1/2 text-faint"
        />
        <input
          type="search"
          spellCheck={false}
          placeholder="Search keys"
          value={search.text}
          onChange={(event) => onChange({ ...search, text: event.target.value })}
          className={cn(TOOLBAR_CONTROL, 'w-full pl-5')}
        />
      </div>
    </>
  )
}
