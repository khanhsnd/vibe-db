import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import type { Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { tags } from '@lezer/highlight'

// Font and size are inherited from the host element so they stay Tailwind's business;
// only the colour tokens are named here, and they resolve live on a theme switch.
const layout = EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: 'var(--surface-canvas)',
    color: 'var(--fg)',
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'inherit', lineHeight: '1.55' },
  '.cm-content': { padding: '6px 0', caretColor: 'var(--accent)' },
  '.cm-line': { padding: '0 10px' },

  '.cm-gutters': {
    backgroundColor: 'var(--surface-canvas)',
    color: 'var(--fg-faint)',
    borderRight: '1px solid var(--line)',
  },
  // Translucent on purpose: the selection layer sits behind the lines, so an opaque
  // active line would paint over the selection the user just made.
  '.cm-activeLine': { backgroundColor: 'var(--line-highlight)' },
  '.cm-activeLineGutter': {
    backgroundColor: 'var(--surface-hover)',
    color: 'var(--fg-muted)',
  },
  // What a folded object or array collapses to in the JSON view. CodeMirror's own placeholder is a
  // light-grey chip, which reads as a hole punched in the line under a dark theme.
  '.cm-foldPlaceholder': {
    backgroundColor: 'var(--surface-hover)',
    border: '1px solid var(--line)',
    color: 'var(--fg-muted)',
  },

  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--accent)' },

  // CodeMirror's own focused-selection rule is five classes deep, so a shorter
  // selector loses the cascade to it and the default lavender wins.
  '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': {
    background: 'var(--selection)',
  },
  '.cm-selectionLayer .cm-selectionBackground': { background: 'var(--selection)' },

  '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': {
    backgroundColor: 'var(--surface-active)',
    outline: '1px solid var(--line-strong)',
  },
  '.cm-placeholder': { color: 'var(--fg-faint)' },

  // The Find panel is CodeMirror's own DOM, and its base theme paints it for a light editor — a
  // white bar with black text under this one — so every colour it wears is named here.
  '.cm-panels.cm-panels-top': {
    backgroundColor: 'var(--surface-bar)',
    color: 'var(--fg)',
    borderBottom: '1px solid var(--line)',
  },
  '.cm-panel.cm-search': { padding: '5px 24px 5px 8px' },
  '.cm-panel.cm-search label': { fontSize: '11px', color: 'var(--fg-muted)' },
  '.cm-panel.cm-search input[type=checkbox]': { accentColor: 'var(--accent)' },
  '.cm-panel.cm-search .cm-textfield': {
    fontFamily: 'inherit',
    fontSize: '12px',
    padding: '2px 6px',
    borderRadius: '4px',
    backgroundColor: 'var(--surface-canvas)',
    border: '1px solid var(--line-strong)',
    color: 'var(--fg)',
  },
  '.cm-panel.cm-search .cm-textfield:focus': { outline: '1px solid var(--accent)' },
  '.cm-panel.cm-search .cm-button, .cm-panel.cm-search .cm-button:active': {
    fontFamily: 'inherit',
    fontSize: '11px',
    padding: '2px 8px',
    borderRadius: '4px',
    backgroundImage: 'none',
    backgroundColor: 'var(--surface-hover)',
    border: '1px solid var(--line-strong)',
    color: 'var(--fg)',
  },
  '.cm-panel.cm-search .cm-button:hover': { backgroundColor: 'var(--surface-active)' },
  '.cm-panel.cm-search [name=close]': { color: 'var(--fg-muted)', fontSize: '15px' },

  '.cm-searchMatch': { backgroundColor: 'var(--search-match)' },
  '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: 'var(--search-match-active)' },

  '.cm-tooltip': {
    backgroundColor: 'var(--surface-elevated)',
    border: '1px solid var(--line)',
    borderRadius: '6px',
    boxShadow: 'var(--shadow-overlay)',
    overflow: 'hidden',
  },
  '.cm-tooltip.cm-tooltip-autocomplete > ul': {
    fontFamily: 'inherit',
    maxHeight: '15em',
    minWidth: '18em',
  },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li': { padding: '3px 8px' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: 'var(--surface-selected)',
    color: 'var(--fg)',
  },
  '.cm-completionIcon': { color: 'var(--fg-faint)', paddingRight: '14px' },
  '.cm-completionMatchedText': {
    textDecoration: 'none',
    color: 'var(--accent)',
    fontWeight: '600',
  },
  '.cm-completionDetail': {
    color: 'var(--fg-faint)',
    fontStyle: 'normal',
    marginLeft: '1.5em',
  },
})

const syntax = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--syn-keyword)' },
  { tag: [tags.string, tags.special(tags.string)], color: 'var(--syn-string)' },
  { tag: [tags.number, tags.bool], color: 'var(--syn-number)' },
  { tag: tags.null, color: 'var(--syn-null)' },
  { tag: [tags.lineComment, tags.blockComment], color: 'var(--syn-comment)', fontStyle: 'italic' },
  { tag: [tags.typeName, tags.standard(tags.name)], color: 'var(--syn-func)' },
  { tag: tags.special(tags.name), color: 'var(--syn-func)' },
  { tag: [tags.operator, tags.punctuation], color: 'var(--fg-muted)' },
  { tag: [tags.paren, tags.brace, tags.squareBracket], color: 'var(--fg-muted)' },

  // The shell's tokens. A document store writes no keywords, so what carries the colour is the
  // shape of a command: `db` is the only bare name one ever begins with, a call is a call whether
  // it is `find(…)` or `ObjectId(…)`, and a document's keys stay the colour of the data they
  // name — bar the operators among them, which `shellLanguage.ts` marks.
  { tag: tags.variableName, color: 'var(--syn-keyword)' },
  {
    tag: [tags.function(tags.variableName), tags.function(tags.propertyName), tags.className],
    color: 'var(--syn-func)',
  },
  { tag: tags.regexp, color: 'var(--syn-string)' },
])

export const editorTheme: Extension = [layout, syntaxHighlighting(syntax)]
