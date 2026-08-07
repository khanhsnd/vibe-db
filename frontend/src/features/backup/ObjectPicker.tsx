import { useMemo, useState } from 'react'

import { Button } from '@/shared/ui/Button'
import { Checkbox } from '@/shared/ui/Field'

/**
 * Which tables a backup covers, at either end of one.
 *
 * **Taking a backup and running one back ask the same question**, so they ask it with the same list:
 * New Backup narrows what goes into the file and Restore narrows what comes back out of it, and a
 * reader who has ticked one recognises the other. Where the names come from is all that differs — a
 * catalogue read there, the file's own header here.
 */

/** What is left out is what is remembered, so a name the list gains is still ticked. */
export function useObjectPicker(tables: string[]) {
  const [excluded, setExcluded] = useState<ReadonlySet<string>>(new Set())

  const chosen = useMemo(() => tables.filter((table) => !excluded.has(table)), [tables, excluded])

  return {
    excluded,
    chosen,
    /** Every table as `[]`, which both bound methods read as all of them — whatever those are. */
    wire: chosen.length === tables.length ? [] : chosen,
    toggle: (table: string) =>
      setExcluded((current) => {
        const next = new Set(current)
        if (next.has(table)) next.delete(table)
        else next.add(table)
        return next
      }),
    all: () => setExcluded(new Set()),
    none: () => setExcluded(new Set(tables)),
  }
}

interface ObjectPickerProps {
  tables: string[]
  excluded: ReadonlySet<string>
  onToggle: (table: string) => void
  onAll: () => void
  onNone: () => void
}

export function ObjectPicker({ tables, excluded, onToggle, onAll, onNone }: ObjectPickerProps) {
  return (
    <div className="flex h-full flex-col gap-2">
      <div className="flex items-center gap-2">
        <Button variant="subtle" onClick={onAll}>
          Select All
        </Button>
        <Button variant="subtle" onClick={onNone}>
          Select None
        </Button>
      </div>

      <div className="scroll-thin max-h-[15rem] flex-1 overflow-auto rounded-md border border-line p-2">
        {tables.map((table) => (
          <div key={table} className="py-0.5">
            <Checkbox
              label={table}
              checked={!excluded.has(table)}
              onChange={() => onToggle(table)}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
