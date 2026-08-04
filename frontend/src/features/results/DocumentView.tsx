import { javascriptLanguage } from '@codemirror/lang-javascript'
import { json } from '@codemirror/lang-json'
import { foldGutter, HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { EditorState, type Extension } from '@codemirror/state'
import { EditorView, lineNumbers } from '@codemirror/view'
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

const extensions = (notation: Notation): Extension => [
  EditorState.readOnly.of(true),
  EditorView.editable.of(false),
  lineNumbers(),
  foldGutter(),
  GRAMMARS[notation],
  editorTheme,
  syntaxHighlighting(keyStyle),
]

/**
 * A document read as one: numbered lines, a fold arrow beside every object and array, and the
 * editor's own colours. It is the SQL editor's engine rather than a `<pre>` because a document worth
 * opening is often far larger than a screen, and CodeMirror lays out the lines in view and none of
 * the rest — a `<pre>` of a megabyte freezes the window.
 *
 * Read-only, as every rendering in the viewer is — see `ValueViewer`. The text is still selectable,
 * so a document can be copied out of it.
 */
export function DocumentView({ text, notation }: { text: string; notation: Notation }) {
  const host = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const parent = host.current
    if (!parent) return

    // A different value is a different document with no caret of its own to keep, so it is a fresh
    // view rather than a dispatched change.
    const view = new EditorView({
      parent,
      state: EditorState.create({ doc: text, extensions: extensions(notation) }),
    })
    return () => view.destroy()
  }, [text, notation])

  return <div ref={host} className="data-font scroll-thin min-h-0 flex-1" />
}
