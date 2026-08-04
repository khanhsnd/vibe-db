import { autocompletion, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { bracketMatching, indentOnInput } from '@codemirror/language'
import { Compartment, EditorState, type Extension } from '@codemirror/state'
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder,
} from '@codemirror/view'
import { useEffect, useRef } from 'react'

import type { TableSchema } from '@/bridge/connections'
import { useSettingsStore } from '@/stores/settings'

import { editorTheme } from './editorTheme'
import { sqlLanguage } from './sqlLanguage'

// QueryWorkspace runs the query from a window listener, so the editor only has to keep
// defaultKeymap from inserting a blank line on the same chord.
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
  placeholder('SELECT * FROM ...'),
  keymap.of([swallowRunShortcut, ...closeBracketsKeymap, ...defaultKeymap, ...historyKeymap]),
  editorTheme,
]

// The schema sits in a compartment so a late-arriving schema reconfigures completion
// instead of rebuilding the editor and losing the caret and the undo history. Word wrap is one
// for the same reason: the reader turns it off in Settings while an editor is open.
const language = new Compartment()
const wrapping = new Compartment()

const lineWrap = (wordWrap: boolean): Extension => (wordWrap ? EditorView.lineWrapping : [])

interface EditorPaneProps {
  value: string
  onChange: (value: string) => void
  onSelectionChange: (selected: string) => void
  schema: TableSchema[]
  engine: string
}

export function EditorPane({
  value,
  onChange,
  onSelectionChange,
  schema,
  engine,
}: EditorPaneProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const wordWrap = useSettingsStore((s) => s.settings.wordWrap)

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
          language.of(sqlLanguage(seed.current.schema, seed.current.engine)),
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
    view.current?.dispatch({ effects: language.reconfigure(sqlLanguage(schema, engine)) })
  }, [schema, engine])

  useEffect(() => {
    view.current?.dispatch({ effects: wrapping.reconfigure(lineWrap(wordWrap)) })
  }, [wordWrap])

  // The store owns the buffer, but echoing our own edits back would reset the caret,
  // so only a genuine divergence is written into the document.
  useEffect(() => {
    const instance = view.current
    if (!instance || instance.state.doc.toString() === value) return
    instance.dispatch({ changes: { from: 0, to: instance.state.doc.length, insert: value } })
  }, [value])

  return <div ref={host} className="data-font scroll-thin h-full min-h-0" />
}
