export type Theme = 'dark' | 'light'

/** The font data is drawn in: a family the machine has, at the size and weight the reader set. */
export interface DataFont {
  family: string
  size: number
  weight: number
}

/** Sizes a row can hold: `ROW_HEIGHT` is 28 px, and past 16 the descenders start to clip. */
export const FONT_SIZES = [10, 11, 12, 13, 14, 15, 16]

export const FONT_WEIGHTS = [
  { value: 200, label: 'ExtraLight' },
  { value: 300, label: 'Light' },
  { value: 400, label: 'Regular' },
  { value: 500, label: 'Medium' },
]

/**
 * What the picker offers where the host answers with no font list of its own — a browser dev run,
 * or a platform whose fonts are not the registry's to give. Every one is a family Windows ships.
 */
export const FALLBACK_FAMILIES = [
  'Cascadia Mono',
  'Cascadia Code',
  'Consolas',
  'Lucida Console',
  'Courier New',
  'Segoe UI',
]

/** Mirrors the `--font-data*` block in `tokens.css`, which is what draws until a pick is made. */
export const DEFAULT_DATA_FONT: DataFont = { family: 'Cascadia Mono', size: 13, weight: 300 }

export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}

/**
 * Writes the pick onto the root element, ahead of the fallbacks the stylesheet names, so a family
 * the machine turns out not to have still lands on something monospaced. Quotes and the characters
 * that would end a declaration are dropped rather than escaped: no font is named with them, and a
 * value that could close the declaration would be a stylesheet the reader typed.
 */
export function applyDataFont(font: DataFont) {
  const family = font.family.replace(/["'\\;{}]/g, '').trim()
  const root = document.documentElement.style

  root.setProperty('--font-data', `'${family}', var(--font-data-fallback)`)
  root.setProperty('--font-data-size', `${font.size}px`)
  root.setProperty('--font-data-weight', String(font.weight))
}
