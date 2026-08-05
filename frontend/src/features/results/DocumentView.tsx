import { javascriptLanguage } from '@codemirror/lang-javascript'
import { json } from '@codemirror/lang-json'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { foldGutter, HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { EditorState, type Extension } from '@codemirror/state'
import { EditorView, keymap, lineNumbers } from '@codemirror/view'
import { tags } from '@lezer/highlight'
import { useEffect, useRef } from 'react'

import { editorTheme } from '@/features/editor/editorTheme'

/** Which notation a laid-out value is in, which is the engine's and not the column's. */
export type Notation = 'json' | 'shell'

/**
 * The grammar each notation is read by. Both are the library's — **nothing here scans anything**,
 * which is the rule.
 *
 * `shell` is a document store's own notation, and it is read as a **single expression** rather than
 * as a script: read as a script a bare `{ … }` is a *block*, its first field a label and every one
 * after it a syntax error, which is why a document's keys come out uncoloured and half its lines
 * squiggled. `SingleExpression` is the parser's own top rule for exactly this, and under it the
 * document is the object it plainly is.
 */
const GRAMMARS: Record<Notation, Extension> = {
  json: json(),
  shell: javascriptLanguage.configure({ top: 'SingleExpression' }).extension,
}

/**
 * The one token the shared syntax style leaves out on purpose. In the shell *editor* a document's
 * keys are the colour of the data they name, because a command is mostly not keys; a document being
 * **read** is nothing but keys and values, and telling the two apart is the whole of reading one.
 */
const keyStyle = HighlightStyle.define([{ tag: tags.propertyName, color: 'var(--syn-func)' }])

const extensions = (notation: Notation, editable: boolean): Extension => [
  lineNumbers(),
  foldGutter(),
  GRAMMARS[notation],
  editorTheme,
  syntaxHighlighting(keyStyle),
  editable
    ? [history(), keymap.of([...defaultKeymap, ...historyKeymap])]
    : [EditorState.readOnly.of(true), EditorView.editable.of(false)],
]

/**
 * A document read as one, and typed back in the notation it was read in: numbered lines, a fold
 * arrow beside every object and array, and the editor's own colours. It is the SQL editor's engine
 * rather than a `<pre>` because a document worth opening is often far larger than a screen, and
 * CodeMirror lays out the lines in view and none of the rest — a `<pre>` of a megabyte freezes the
 * window.
 *
 * **Editable wherever the value can be written back**, which is what `onChange` says; without one
 * it is read-only. The text is selectable either way — `selectable`, because `user-select: none` is
 * global and a document nobody can select is a document nobody can copy.
 */
export function DocumentView({
  text,
  notation,
  onChange,
}: {
  text: string
  notation: Notation
  /** Absent for a read-only surface, exactly as `ValueViewer.onChange` is. */
  onChange?: ((next: string) => void) | undefined
}) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const editable = onChange !== undefined

  // Held in refs for the reason `EditorPane` holds its own: the view is built once and must not be
  // rebuilt to learn a new handler or a new line of text.
  const emit = useRef(onChange)
  emit.current = onChange
  const seed = useRef(text)
  seed.current = text

  useEffect(() => {
    const parent = host.current
    if (!parent) return

    const instance = new EditorView({
      parent,
      state: EditorState.create({
        doc: seed.current,
        extensions: [
          extensions(notation, editable),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) emit.current?.(update.state.doc.toString())
          }),
        ],
      }),
    })

    view.current = instance
    return () => {
      instance.destroy()
      view.current = null
    }
  }, [notation, editable])

  // The viewer owns the value, but echoing our own keystroke back into the document would reset
  // the caret on every character typed, so only a genuine divergence is written in.
  useEffect(() => {
    const instance = view.current
    if (!instance || instance.state.doc.toString() === text) return

    instance.dispatch({ changes: { from: 0, to: instance.state.doc.length, insert: text } })
  }, [text])

  return <div ref={host} className="data-font scroll-thin selectable min-h-0 flex-1" />
}
