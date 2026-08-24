let context: CanvasRenderingContext2D | null = null

function measurer(): CanvasRenderingContext2D | null {
  context ??= document.createElement('canvas').getContext('2d')
  return context
}

/** The data font as the reader set it, so a fit measures the cells that are actually drawn. */
export function gridFont(): string {
  const style = getComputedStyle(document.documentElement)
  const weight = style.getPropertyValue('--font-data-weight').trim()
  const size = style.getPropertyValue('--font-data-size').trim()
  const family = style.getPropertyValue('--font-data').trim()
  return `${weight} ${size} ${family}`
}

/** The header's own font, since a column name is drawn as a label rather than as a cell. */
export function headerFont(): string {
  const style = getComputedStyle(document.documentElement)
  const size = style.getPropertyValue('--text-base').trim()
  const family = style.getPropertyValue('--font-ui').trim()
  return `700 ${size} ${family}`
}

/** `tracking-wide`, which is what a header's name is drawn with. */
const HEADER_TRACKING = 0.025

/**
 * A header name's width, its letter-spacing included. `measureText` knows nothing of CSS
 * `letter-spacing`, so a name measured plainly comes back a third of a pixel a character short —
 * which on a name of thirty is ten pixels, and was enough for a long one to be clipped by its own
 * sort arrow on the width measured to hold it.
 */
export function headerNameWidth(name: string): number {
  const style = getComputedStyle(document.documentElement)
  const size = parseFloat(style.getPropertyValue('--text-base'))
  const tracking = Number.isFinite(size) ? size * HEADER_TRACKING : 0

  return textWidth(name, headerFont()) + Math.ceil(name.length * tracking)
}

/** One string in one font, which is what anything that has to *fit* a label is measured by. */
export function textWidth(text: string, font: string): number {
  const target = measurer()
  if (!target) return 0

  target.font = font
  return Math.ceil(target.measureText(text).width)
}

export function widestText(texts: string[], font: string): number {
  let widest = 0
  for (const text of texts) {
    widest = Math.max(widest, textWidth(text, font))
  }
  return widest
}
