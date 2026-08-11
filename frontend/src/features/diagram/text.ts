import type { TableColumn } from '@/bridge/tables'
import { textWidth } from '@/features/results/measureText'

import { DEFAULT_NODE_WIDTH, MIN_NODE_WIDTH } from './model'

/**
 * How wide a label is on the canvas, and what to draw instead when it does not fit.
 *
 * A box is a fixed width the reader drags, so a name longer than it has to be **shortened rather
 * than left to run out of the box**: SVG text does not wrap and does not clip. Everything here
 * measures against the same font the canvas draws in, which is the UI font — a diagram is chrome
 * about the data rather than the data, so it is not the reader's data font. See docs/frontend.md.
 */

/** The three sizes anything on the canvas is drawn at. */
export const TITLE_SIZE = 12
export const FIELD_SIZE = 11
export const NOTE_SIZE = 12

const family = () => getComputedStyle(document.documentElement).getPropertyValue('--font-ui').trim()

export function canvasFont(size: number, weight: number | string = 400): string {
  return `${weight} ${size}px ${family()}`
}

const ELLIPSIS = '…'

/** The text as it fits, with an ellipsis where it did not. An empty budget draws nothing at all. */
export function ellipsed(text: string, font: string, max: number): string {
  if (max <= 0) return ''
  if (textWidth(text, font) <= max) return text

  let low = 0
  let high = text.length

  while (low < high) {
    const middle = Math.ceil((low + high) / 2)
    if (textWidth(text.slice(0, middle) + ELLIPSIS, font) <= max) low = middle
    else high = middle - 1
  }
  return low === 0 ? ELLIPSIS : text.slice(0, low) + ELLIPSIS
}

/** How a field's type reads in a box: the declared one, which is what a designer would show. */
export const fieldType = (column: TableColumn) => column.fullType || column.type

/**
 * What a note holds, laid out into the lines it is drawn as: the lines it was typed as, each broken
 * again where it is wider than the shape. SVG text does not wrap, so the wrapping is ours to do —
 * and a word longer than the whole shape is left to be shortened rather than broken mid-word.
 */
export function wrapped(text: string, font: string, max: number): string[] {
  return text.split('\n').flatMap((paragraph) => {
    if (paragraph === '') return ['']

    const out: string[] = []
    let line = ''

    for (const word of paragraph.split(' ')) {
      const next = line === '' ? word : `${line} ${word}`
      if (line !== '' && textWidth(next, font) > max) {
        out.push(line)
        line = word
      } else {
        line = next
      }
    }
    out.push(line)
    return out
  })
}

/**
 * How wide the box would have to be for nothing in it to be shortened, within reason: a table with
 * one four-hundred-pixel column name is a box nobody wants that wide, so the fit is capped.
 */
const MAX_FIT_WIDTH = 420

export function fitWidth(table: string, columns: readonly TableColumn[]): number {
  const title = canvasFont(TITLE_SIZE, 600)
  const field = canvasFont(FIELD_SIZE)

  // The header carries a glyph, the name and the field count; a row carries a glyph, the name and
  // the type. Both are measured with the padding they are drawn with.
  let widest = textWidth(table, title) + 62

  for (const column of columns) {
    widest = Math.max(widest, textWidth(column.name, field) + textWidth(fieldType(column), field) + 46)
  }

  if (columns.length === 0) widest = Math.max(widest, DEFAULT_NODE_WIDTH)
  return Math.min(MAX_FIT_WIDTH, Math.max(MIN_NODE_WIDTH, Math.ceil(widest)))
}
