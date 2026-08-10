import { useEffect, useMemo, useState } from 'react'
import { Check, Code2, Copy, X } from 'lucide-react'

import { copyText } from '@/bridge/clipboard'
import type { Namespace } from '@/bridge/connections'
import { Button } from '@/shared/ui/Button'
import { SelectInput } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'

export interface CurlTarget {
  namespace: Namespace
  table: string
  fields: string[]
}

const shellQuote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`

export function curlCommand(target: CurlTarget): string {
  const { namespace, table, fields } = target
  const query = new URLSearchParams({ database: namespace.database, table })
  if (namespace.schema !== '') query.set('schema', namespace.schema)
  const body = JSON.stringify(Object.fromEntries(fields.map((field) => [field, ''])))

  return [
    `curl --request POST --url ${shellQuote(`http://localhost:8080/api/tables?${query}`)}`,
    "  --header 'Content-Type: application/json'",
    `  --data ${shellQuote(body)}`,
  ].join(' \\\n')
}

interface CurlDrawerProps {
  open: boolean
  target: CurlTarget | null
  onOpenChange: (open: boolean) => void
}

export function CurlDrawer({ open, target, onOpenChange }: CurlDrawerProps) {
  const [format, setFormat] = useState('curl')
  const [copied, setCopied] = useState(false)
  const command = useMemo(() => (target ? curlCommand(target) : ''), [target])

  useEffect(() => setCopied(false), [command, open])

  if (!open || target === null) return null

  const copy = () => {
    void copyText(command)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1200)
  }

  return (
    <div className="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-label="Generate cURL">
      <button
        type="button"
        aria-label="Close Generate cURL"
        className="absolute inset-0 bg-black/50"
        onClick={() => onOpenChange(false)}
      />
      <aside className="absolute inset-y-0 right-0 flex w-[min(440px,92vw)] flex-col border-l border-line-strong bg-elevated shadow-[var(--shadow-overlay)]">
        <header className="flex shrink-0 items-center gap-3 border-b border-line bg-bar px-4 py-3">
          <span className="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-accent-subtle text-accent">
            <Code2 size={16} />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold text-fg">Generate cURL</h2>
            <p className="truncate text-xs text-muted">{target.table}</p>
          </div>
          <IconButton label="Close" icon={<X size={14} />} onClick={() => onOpenChange(false)} />
        </header>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-4">
          <div className="max-w-48">
            <label className="block text-xs font-medium text-muted" htmlFor="curl-format">
              Format
              <SelectInput
                id="curl-format"
                value={format}
                onChange={(event) => setFormat(event.target.value)}
                className="mt-1.5"
              >
                <option value="curl">cURL</option>
              </SelectInput>
            </label>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="text-xs font-medium text-muted">Command</div>
              <IconButton
                label={copied ? 'Copied' : 'Copy cURL'}
                icon={copied ? <Check size={14} /> : <Copy size={14} />}
                className="text-accent hover:bg-accent-subtle hover:text-accent"
                onClick={copy}
              />
            </div>
            <pre className="scroll-thin selectable cursor-text overflow-x-auto whitespace-pre-wrap break-words rounded-md border border-line bg-panel p-3 font-mono text-xs leading-relaxed text-fg">
              {command}
            </pre>
          </div>
        </div>

        <footer className="flex shrink-0 justify-end border-t border-line bg-bar px-4 py-2.5">
          <Button onClick={() => onOpenChange(false)}>Close</Button>
        </footer>
      </aside>
    </div>
  )
}
