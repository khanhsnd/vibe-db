import { autocompletion, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { bracketMatching, indentOnInput } from '@codemirror/language'
import { search, searchKeymap } from '@codemirror/search'
import { Compartment, EditorState, type Extension } from '@codemirror/state'
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
} from '@codemirror/view'
import { useEffect, useImperativeHandle, useRef, type Ref } from 'react'

import { useSettingsStore } from '@/stores/settings'

import { diagnostics, position, showDiagnostics, type Diagnostics } from './diagnostics'
import { editorTheme } from './editorTheme'
import { queryLanguage } from './queryLanguage'
import type { QuerySchema } from './sqlLanguage'

// QueryWorkspace runs the query and lays it out from window listeners, so the editor only has to
// keep defaultKeymap from inserting a blank line on the same chord.
const swallowRunShortcut = { key: 'Mod-Enter', run: () => true }

const baseExtensions: Extension = [
  lineNumbers(),
  highlightActiveLine(),
  highlightActiveLineGutter(),
  history(),
  drawSelection(),
  indentOnInput(),
  bracketMatching(),
  closeBrackets(),
  autocompletion(),
  // Ctrl+F, over the buffer and never over a server. It is bound in the editor rather than on the
  // window like F5, because what it searches is the text the caret is in.
  search({ top: true }),
  diagnostics,
  keymap.of([
    swallowRunShortcut,
    ...closeBracketsKeymap,
    ...searchKeymap,
    ...defaultKeymap,
    ...historyKeymap,
  ]),
  editorTheme,
]

// The schema sits in a compartment so a late-arriving schema reconfigures completion
// instead of rebuilding the editor and losing the caret and the undo history. Word wrap is one
// for the same reason: the reader turns it off in Settings while an editor is open.
const language = new Compartment()
const wrapping = new Compartment()

const lineWrap = (wordWrap: boolean): Extension => (wordWrap ? EditorView.lineWrapping : [])

/** What the panes around the editor can ask of it, which is one thing: show me that offset. */
export interface EditorHandle {
  /** Puts the caret at a **byte** offset — what a review reports — and scrolls it into view. */
  reveal: (offset: number) => void
}

interface EditorPaneProps {
  value: string
  onChange: (value: string) => void
  onSelectionChange: (selected: string) => void
  schema: QuerySchema
  engine: string
  /** What the last review found, marked in the text itself — see `diagnostics.ts`. */
  review: Diagnostics
  ref?: Ref<EditorHandle>
}

export function EditorPane({
  value,
  onChange,
  onSelectionChange,
  schema,
  engine,
  review,
  ref,
}: EditorPaneProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const wordWrap = useSettingsStore((s) => s.settings.wordWrap)

  useImperativeHandle(ref, () => ({
    reveal(offset) {
      const instance = view.current
      if (!instance) return

      const text = instance.state.doc.toString()
      const at = Math.min(position(text)(offset), instance.state.doc.length)
      instance.dispatch({ selection: { anchor: at }, scrollIntoView: true })
      instance.focus()
    },
  }))

  const emit = useRef({ onChange, onSelectionChange })
  emit.current = { onChange, onSelectionChange }

  const seed = useRef({ value, schema, engine, wordWrap })

  useEffect(() => {
    const parent = host.current
    if (!parent) return

    const instance = new EditorView({
      parent,
      state: EditorState.create({
        doc: seed.current.value,
        extensions: [
          baseExtensions,
          language.of(queryLanguage(seed.current.schema, seed.current.engine)),
          wrapping.of(lineWrap(seed.current.wordWrap)),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) emit.current.onChange(update.state.doc.toString())
            if (update.docChanged || update.selectionSet) {
              const { from, to } = update.state.selection.main
              emit.current.onSelectionChange(update.state.sliceDoc(from, to))
            }
          }),
        ],
      }),
    })

    view.current = instance
    return () => {
      instance.destroy()
      view.current = null
    }
  }, [])

  useEffect(() => {
    view.current?.dispatch({ effects: language.reconfigure(queryLanguage(schema, engine)) })
  }, [schema, engine])

  useEffect(() => {
    view.current?.dispatch({ effects: wrapping.reconfigure(lineWrap(wordWrap)) })
  }, [wordWrap])

  useEffect(() => {
    showDiagnostics(view.current, review)
  }, [review])

  // The store owns the buffer, but echoing our own edits back would reset the caret,
  // so only a genuine divergence is written into the document.
  useEffect(() => {
    const instance = view.current
    if (!instance || instance.state.doc.toString() === value) return
    instance.dispatch({ changes: { from: 0, to: instance.state.doc.length, insert: value } })
  }, [value])

  return <div ref={host} className="data-font scroll-thin h-full min-h-0" />
}
