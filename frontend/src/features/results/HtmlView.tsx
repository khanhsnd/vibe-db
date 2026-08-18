import { useTheme } from '@/shared/theme/useTheme'

/**
 * What the page a cell holds is allowed to do, which is **nothing**. It is somebody else's markup
 * and this window holds the app's own bridge to their database, so it is drawn in a frame with an
 * opaque origin, no scripts, no forms and no way to navigate anything — `sandbox` with nothing
 * allowed back — and the policy below inside it as well, so that nothing in a reader's database is
 * ever sent anywhere to be drawn. An image written into the value as a `data:` URI still shows; one
 * that names a server does not load, exactly as a shape is drawn without a tile server and an image
 * is drawn from the bytes already in hand.
 *
 * A second policy cannot loosen the first, so markup carrying a `<meta>` of its own is no way round
 * this.
 */
const POLICY = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:"

/**
 * The app's own tokens, copied into the frame — a frame inherits no custom property, so a page drawn
 * in one has none unless it is given them. They are declared rather than pasted into rules so that
 * markup built by this app can be styled in the app's own colours: `documentPage` writes a
 * stylesheet over these names, and rule 4 holds — the values are still only ever `tokens.css`'s.
 */
const TOKENS = [
  '--surface-canvas',
  '--fg',
  '--fg-muted',
  '--fg-faint',
  '--line',
  '--line-strong',
  '--accent',
  '--syn-func',
  '--font-ui',
  '--font-data',
]

/**
 * The page a cell holds, drawn. It is a rendering and so is **read-only whatever the surface is** —
 * `Raw` is beside it for typing into, as it is beside a hex dump and a shape.
 */
export function HtmlView({ markup }: { markup: string }) {
  // Called for its subscription and not its answer: a frame inherits no custom property, so the
  // app's tokens are copied into the page below and must be copied again when the reader switches
  // theme under an open preview.
  useTheme()

  return (
    <iframe
      title="The page this cell holds"
      srcDoc={page(markup)}
      sandbox=""
      referrerPolicy="no-referrer"
      className="min-h-0 w-full flex-1 border-0 bg-canvas"
    />
  )
}

/**
 * The markup as a whole document: the policy first, then the app's own tokens, then the value
 * verbatim. A value that is already a whole document is wrapped all the same — the parser drops the
 * second `<html>` and keeps what is inside it — because the policy has to be in the head of the
 * document that is parsed, and that is this one.
 *
 * The tokens are read off the element that has them, since a frame inherits none: they are the
 * *ground* the page is drawn on rather than the page's own look, and each rule below is one the
 * value's own stylesheet overrides.
 */
function page(markup: string): string {
  const style = getComputedStyle(document.documentElement)
  const copied = TOKENS.map((name) => `${name}:${style.getPropertyValue(name).trim()}`)

  return [
    '<!doctype html><html><head><meta charset="utf-8">',
    `<meta http-equiv="Content-Security-Policy" content="${POLICY}">`,
    '<style>',
    `:root{color-scheme:${style.getPropertyValue('color-scheme').trim()};${copied.join(';')}}`,
    'body{margin:12px;font-family:var(--font-ui);font-size:13px;',
    'background:var(--surface-canvas);color:var(--fg)}',
    'a{color:var(--accent)}',
    '</style></head><body>',
    markup,
    '</body></html>',
  ].join('')
}
