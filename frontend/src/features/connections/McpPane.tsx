import { useEffect, useState } from 'react'
import { Check, Copy } from 'lucide-react'

import { copyText } from '@/bridge/clipboard'
import { mcp } from '@/bridge/mcp'
import { Button } from '@/shared/ui/Button'
import { Checkbox, Field, TextInput } from '@/shared/ui/Field'

/**
 * Whether an AI agent may read this connection, and the one line that lets it. What an agent may
 * *do* is not asked here: it is always a read, whatever Access says — see
 * `domain.ConnectionProfile.ForAgent`.
 */
export function McpPane({ enabled, onChange }: { enabled: boolean; onChange: (on: boolean) => void }) {
  const [command, setCommand] = useState('')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    void mcp
      .command()
      .then(setCommand)
      .catch(() => {})
  }, [])

  const copy = () => {
    void copyText(command).then(() => setCopied(true))
  }

  return (
    <>
      <Checkbox
        label="Let AI agents read this connection"
        checked={enabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <p className="text-xs text-muted">
        An agent such as Claude finds it by name through <code>vibe-db mcp</code> and is always held to
        reading: anything that writes is refused before it is sent, whatever Access says. It is shown
        the name, the engine and the address — never a password.
      </p>

      <Field
        label="Register with Claude Code"
        hint="Run once in a terminal. The app does not have to be open for an agent to use it."
      >
        {(id) => (
          <div className="flex items-center gap-2">
            <TextInput id={id} readOnly value={command} className="font-mono text-xs" />
            <Button
              variant="default"
              icon={copied ? <Check size={12} /> : <Copy size={12} />}
              disabled={command === ''}
              onClick={copy}
            >
              {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
        )}
      </Field>
    </>
  )
}
