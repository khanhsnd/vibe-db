import type { TransferMessage } from '@/bridge/transfer'
import { cn } from '@/lib/cn'

/**
 * What a transfer did, line by line — the last screen of both wizards.
 *
 * **A transcript and not a grid**, for the reason [`ScriptLog`](../connections/ExecuteFileDialog.tsx)
 * is one: a server's rejection is a paragraph it wrote, and a path is a hundred characters wide. Put
 * in cells, those are two columns clipped to nothing and the one thing the reader opened this to read
 * is the one thing they cannot see. `ResultGrid` is still the only thing in this app that draws rows
 * and columns; this is simply not rows and columns.
 *
 * Selectable and copied as it reads, so a log lifted out of here into a message arrives laid out the
 * way it was shown.
 */
export function TransferLog({
  messages,
  started,
  finished,
}: {
  messages: TransferMessage[]
  /** The first line, which names what the job was — written before anything had happened. */
  started: string
  /** The last line, and empty while the job is still going. */
  finished: string
}) {
  return (
    <div className="scroll-thin selectable data-font h-64 overflow-auto rounded-md border border-line bg-canvas px-3 py-2">
      <Line tone="accent">{started}</Line>

      {messages.map((message, at) => (
        <Line key={at} tone={message.error ? 'danger' : 'muted'}>
          {`${message.error ? '[ERR]' : '[OK] '} ${message.text}`}
        </Line>
      ))}

      {finished ? <Line tone="accent">{finished}</Line> : null}
    </div>
  )
}

const TONES = {
  muted: 'text-muted',
  danger: 'text-danger',
  accent: 'text-accent',
} as const

/**
 * One line, hanging from its tag: `pl-[6ch]` puts the body under the tag and the negative indent of
 * the same width pulls the first line back out to where the tag belongs, so a wrapped path stays
 * under the path rather than under the tag.
 */
function Line({ tone, children }: { tone: keyof typeof TONES; children: string }) {
  return (
    <div className={cn('-indent-[6ch] pl-[6ch] whitespace-pre-wrap', TONES[tone])}>{children}</div>
  )
}
