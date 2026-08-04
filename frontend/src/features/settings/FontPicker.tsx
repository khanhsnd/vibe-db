import { useEffect, useMemo, useRef, useState } from 'react'

import { listFonts } from '@/bridge/fonts'
import { cn } from '@/lib/cn'
import {
  FALLBACK_FAMILIES,
  FONT_SIZES,
  FONT_WEIGHTS,
  type DataFont,
} from '@/shared/theme/appearance'
import { Field, SelectInput, TextInput } from '@/shared/ui/Field'

interface FontPickerProps {
  font: DataFont
  onChange: (part: Partial<DataFont>) => void
}

/**
 * The data font, picked from what the machine actually has: the host answers with every family it
 * has registered — a few hundred, once fonts are installed per user — so the list is searched
 * rather than scrolled. Every row is drawn in its own family, which is the only honest way to show
 * what a pick will look like before it is made.
 */
export function FontPicker({ font, onChange }: FontPickerProps) {
  const [families, setFamilies] = useState<string[]>(FALLBACK_FAMILIES)
  const [query, setQuery] = useState('')
  const list = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let live = true
    void listFonts().then((installed) => {
      if (live && installed.length > 0) setFamilies(installed)
    })
    return () => {
      live = false
    }
  }, [])

  // The pick can sit hundreds of rows down an alphabetical list, so the list opens on it rather
  // than at A. `nearest` and not `center`, or the scroll would take the dialog with it.
  useEffect(() => {
    list.current?.querySelector('[data-picked]')?.scrollIntoView({ block: 'nearest' })
  }, [families])

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return families.filter((family) => family.toLowerCase().includes(needle))
  }, [families, query])

  return (
    <div className="flex flex-col gap-2.5">
      <Field label="Family" inline hint={`${matches.length} of ${families.length} installed`}>
        {(controlId) => (
          <TextInput
            id={controlId}
            value={query}
            placeholder={`Search — ${font.family}`}
            onChange={(event) => setQuery(event.target.value)}
          />
        )}
      </Field>

      <div
        ref={list}
        className="scroll-thin h-44 overflow-y-auto rounded-md border border-line bg-canvas py-1"
      >
        {matches.map((family) => {
          const picked = family === font.family

          return (
            <button
              key={family}
              type="button"
              data-picked={picked ? '' : undefined}
              onClick={() => onChange({ family })}
              style={{ fontFamily: `'${family}', var(--font-data-fallback)` }}
              className={cn(
                'block w-full truncate px-2.5 py-1 text-left text-sm',
                picked ? 'bg-selected text-fg' : 'text-muted hover:bg-hover hover:text-fg',
              )}
            >
              {family}
            </button>
          )
        })}

        {matches.length === 0 ? (
          <p className="px-2.5 py-1 text-sm text-faint">No family matches that.</p>
        ) : null}
      </div>

      <Field label="Size" inline>
        {(controlId) => (
          <div className="w-28">
            <SelectInput
              id={controlId}
              value={String(font.size)}
              onChange={(event) => onChange({ size: Number(event.target.value) })}
            >
              {FONT_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size} px
                </option>
              ))}
            </SelectInput>
          </div>
        )}
      </Field>

      <Field label="Weight" inline hint="A family without the weight draws in the nearest it has.">
        {(controlId) => (
          <div className="w-28">
            <SelectInput
              id={controlId}
              value={String(font.weight)}
              onChange={(event) => onChange({ weight: Number(event.target.value) })}
            >
              {FONT_WEIGHTS.map((weight) => (
                <option key={weight.value} value={weight.value}>
                  {weight.label}
                </option>
              ))}
            </SelectInput>
          </div>
        )}
      </Field>

      <p className="data-font truncate rounded-md border border-line bg-canvas px-2.5 py-2 text-fg">
        {font.family} — SELECT 1,234.56 · Il1O0 · (Null)
      </p>
    </div>
  )
}
