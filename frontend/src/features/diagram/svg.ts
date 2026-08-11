import type { Rect } from './model'

/**
 * The diagram as a file that can be opened anywhere.
 *
 * What is on the screen is styled by a stylesheet — Tailwind classes over the theme's tokens — and a
 * file carries no stylesheet with it, so every colour, font and width is **read back off the live
 * element and written onto the copy**. That is why this takes the element rather than the model: the
 * one thing an export must not do is draw the picture a second way and disagree with the screen.
 */

/** The properties that decide what a shape looks like, and the whole of what is copied. */
const INHERITED = [
  'fill',
  'fill-opacity',
  'stroke',
  'stroke-width',
  'stroke-opacity',
  'stroke-dasharray',
  'stroke-linecap',
  'stroke-linejoin',
  'opacity',
  'font-family',
  'font-size',
  'font-weight',
  'text-anchor',
  'dominant-baseline',
]

export function toSvgFile(source: SVGSVGElement, box: Rect, background: string): string {
  const copy = source.cloneNode(true) as SVGSVGElement

  const originals = [source, ...source.querySelectorAll('*')]
  const copies = [copy, ...copy.querySelectorAll('*')]

  for (let at = 0; at < originals.length; at += 1) {
    const original = originals[at]
    const clone = copies[at]
    if (!(original instanceof Element) || !(clone instanceof Element)) continue

    const computed = getComputedStyle(original)
    const style = INHERITED.map((property) => `${property}:${computed.getPropertyValue(property)}`)
    clone.setAttribute('style', style.join(';'))
    clone.removeAttribute('class')
  }

  // The grid is drawn in screen space and the scene is drawn panned and zoomed; a file is neither,
  // so the first goes and the second is unwound in favour of a viewBox round what is actually there.
  copy.querySelector('[data-grid]')?.remove()
  copy.querySelector('defs')?.remove()
  copy.querySelector('[data-scene]')?.removeAttribute('transform')
  // Nothing being dragged belongs in a file: an export taken mid-drag would carry the marquee.
  copy.querySelector('[data-overlay]')?.remove()

  copy.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
  copy.setAttribute('viewBox', `${box.x} ${box.y} ${box.width} ${box.height}`)
  copy.setAttribute('width', String(Math.round(box.width)))
  copy.setAttribute('height', String(Math.round(box.height)))
  copy.removeAttribute('style')

  const paper = document.createElementNS('http://www.w3.org/2000/svg', 'rect')
  paper.setAttribute('x', String(box.x))
  paper.setAttribute('y', String(box.y))
  paper.setAttribute('width', String(box.width))
  paper.setAttribute('height', String(box.height))
  paper.setAttribute('fill', background)
  copy.prepend(paper)

  return `<?xml version="1.0" encoding="UTF-8"?>\n${new XMLSerializer().serializeToString(copy)}`
}

/** The canvas colour as the theme has it right now, so an export is light or dark like the app. */
export function canvasColour(): string {
  return getComputedStyle(document.documentElement).getPropertyValue('--surface-canvas').trim()
}
