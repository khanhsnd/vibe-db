import { useEffect, useState } from 'react'
import { Check, Copy } from 'lucide-react'

import { copyText } from '@/bridge/clipboard'

import { IconButton } from './IconButton'

/** Long enough for the tick to be seen, short enough that the button is not left saying it. */
const COPIED_MS = 1200

/**
 * Puts text on the host's clipboard and confirms it on the icon itself, there being no room for a
 * message in a status line or a panel's corner. One component, so copying a statement and copying
 * a server's rejection answer the same way.
 */
export function CopyButton({ text, label }: { text: string; label: string }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const clear = setTimeout(() => setCopied(false), COPIED_MS)
    return () => clearTimeout(clear)
  }, [copied])

  return (
    <IconButton
      label={label}
      icon={copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
      disabled={text === ''}
      onClick={() => {
        void copyText(text)
        setCopied(true)
      }}
    />
  )
}
