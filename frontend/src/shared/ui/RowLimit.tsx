import { useState } from 'react'

import { ALL_ROWS } from '@/bridge/rows'

import { TOOLBAR_CONTROL } from './Field'

const CUSTOM = 'custom'
const numbers = new Intl.NumberFormat('en-US')

const isPreset = (rows: number, presets: readonly number[]) =>
  rows === ALL_ROWS || presets.some((preset) => preset === rows)

interface RowLimitProps {
  label: string
  presets: readonly number[]
  /** Seeded into the input when Custom… is picked while the current value is All. */
  fallback: number
  value: number
  onChange: (rows: number) => void
}

/**
 * Presets, All, and a typed count. Whether the input shows is state rather than derived from
 * the value, because picking Custom… while the value happens to equal a preset must still open
 * it; the draft text is separate so that clearing the input does not fight the last valid count.
 */
export function RowLimit({ label, presets, fallback, value, onChange }: RowLimitProps) {
  const [custom, setCustom] = useState(() => !isPreset(value, presets))
  const [draft, setDraft] = useState(() => String(value))

  const pick = (choice: string) => {
    if (choice !== CUSTOM) {
      setCustom(false)
      onChange(Number(choice))
      return
    }

    // Seed and apply at once, so the number on screen is always the one that will run.
    const seed = value > 0 ? value : fallback
    setCustom(true)
    setDraft(String(seed))
    onChange(seed)
  }

  const type = (text: string) => {
    setDraft(text)
    const rows = Number(text)
    if (Number.isInteger(rows) && rows > 0) onChange(rows)
  }

  return (
    <div className="flex shrink-0 items-center gap-1.5">
      {label}
      <select
        aria-label={label}
        value={custom ? CUSTOM : String(value)}
        onChange={(e) => pick(e.target.value)}
        className={`${TOOLBAR_CONTROL} font-mono`}
      >
        {presets.map((preset) => (
          <option key={preset} value={preset}>
            {numbers.format(preset)}
          </option>
        ))}
        <option value={ALL_ROWS}>All</option>
        <option value={CUSTOM}>Custom…</option>
      </select>

      {custom ? (
        <input
          type="number"
          min={1}
          step={100}
          aria-label={`${label}, row count`}
          value={draft}
          onChange={(e) => type(e.target.value)}
          className={`${TOOLBAR_CONTROL} w-20 font-mono`}
        />
      ) : null}
    </div>
  )
}
