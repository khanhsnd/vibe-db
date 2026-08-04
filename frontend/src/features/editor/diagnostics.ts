import { RangeSetBuilder, StateEffect, StateField, type Extension } from '@codemirror/state'
import { Decoration, EditorView, type DecorationSet } from '@codemirror/view'

import type { ScriptProblem, StatementReview } from '@/bridge/query'

/**
 * What the editor marks in the text itself: the faults the lexer proved, and the statements that
 * will have to be answered for before they run. Both come from one `QueryAPI.Review` — see
 * `bridge/query.ts` — so what is underlined here and what the guard dialog lists cannot disagree.
 */
export interface Diagnostics {
  problems: readonly ScriptProblem[]
  statements: readonly StatementReview[]
}

export const NO_DIAGNOSTICS: Diagnostics = { problems: [], statements: [] }

/** The effect a fresh review is dispatched as. It is a field rather than a compartment because
 * marks change on nearly every keystroke, and reconfiguring the editor that often would throw
 * away work the extensions are holding. */
const setDiagnostics = StateEffect.define<Diagnostics>()

const problemMark = Decoration.mark({ class: 'cm-problem' })
const hazardMark = Decoration.mark({ class: 'cm-hazard' })

const diagnosticsField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(marks, transaction) {
    for (const effect of transaction.effects) {
      if (effect.is(setDiagnostics)) return build(transaction.state.doc.toString(), effect.value)
    }
    // Between reviews the document keeps moving, so the marks move with it rather than sitting
    // where the text used to be — a squiggle a character behind the typing is worse than none.
    return marks.map(transaction.changes)
  },
  provide: (field) => EditorView.decorations.from(field),
})

/**
 * A statement is marked by its **first line alone**, not end to end. A `DELETE` of twenty lines
 * underlined whole is a wall of colour that says nothing more than a single line does, and it
 * makes the text underneath it harder to read at exactly the moment the reader is checking it.
 */
function build(text: string, diagnostics: Diagnostics): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  const at = position(text)

  const ranges = [
    ...diagnostics.statements
      .filter((statement) => statement.confirm)
      .map((statement) => ({
        from: at(statement.start),
        to: at(Math.min(statement.end, statement.start + firstLineBytes(statement.sql))),
        mark: hazardMark,
      })),
    ...diagnostics.problems.map((problem) => ({
      from: at(problem.start),
      to: at(problem.end),
      mark: problemMark,
    })),
  ]
    // A RangeSetBuilder takes its ranges in order and throws on one out of it, and the two lists
    // above interleave: the review reports every statement first and then every fault.
    .filter((range) => range.from < range.to && range.to <= text.length)
    .sort((a, b) => a.from - b.from || a.to - b.to)

  for (const range of ranges) builder.add(range.from, range.to, range.mark)
  return builder.finish()
}

/**
 * Go counts a string in **bytes** and CodeMirror counts it in UTF-16 units, so every offset a
 * review carries has to be translated before it is a position in this document. It matters the
 * moment a script holds anything outside ASCII — a Vietnamese column name, a comment, a string
 * literal — and without it every mark after the first such character lands short of its text.
 *
 * A script that is entirely ASCII needs no translation at all, which is the common case and is
 * why the scan is skipped for one.
 */
export function position(text: string): (offset: number) => number {
  // An all-ASCII script needs no translation, and one byte per unit is exactly what that
  // means — which is cheaper to ask than to scan for, and is the common case.
  if (new TextEncoder().encode(text).length === text.length) return (offset) => offset

  const units = new Map<number, number>()
  let bytes = 0
  for (let i = 0; i < text.length; ) {
    units.set(bytes, i)
    const code = text.codePointAt(i) ?? 0
    bytes += utf8Length(code)
    i += code > 0xffff ? 2 : 1
  }
  units.set(bytes, text.length)

  // An offset the map has no entry for is inside a character rather than at its edge, which a
  // review never reports — clamping to the end is the harmless answer if one ever did.
  return (offset) => units.get(offset) ?? text.length
}

function utf8Length(code: number): number {
  if (code < 0x80) return 1
  if (code < 0x800) return 2
  if (code < 0x10000) return 3
  return 4
}

function firstLineBytes(sql: string): number {
  const end = sql.indexOf('\n')
  return new TextEncoder().encode(end < 0 ? sql : sql.slice(0, end)).length
}

/** Puts a fresh review in force. A no-op on a view that has gone, which a debounce can outlive. */
export function showDiagnostics(view: EditorView | null, diagnostics: Diagnostics) {
  view?.dispatch({ effects: setDiagnostics.of(diagnostics) })
}

// A wavy underline for a fault and a flat one for a hazard, both under the text rather than over
// it: a background tint on a statement fights the syntax colouring, and a statement about to
// delete every row of a table is exactly where the colouring still has to be readable.
const diagnosticsTheme = EditorView.baseTheme({
  '.cm-problem': {
    textDecoration: 'underline wavy var(--danger)',
    textDecorationSkipInk: 'none',
    textUnderlineOffset: '3px',
  },
  '.cm-hazard': {
    textDecoration: 'underline var(--warn)',
    textDecorationThickness: '2px',
    textUnderlineOffset: '3px',
  },
})

export const diagnostics: Extension = [diagnosticsField, diagnosticsTheme]
