import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

const TONES = {
  success: 'border-success/40 bg-success/10 text-success',
  danger: 'border-danger/40 bg-danger-subtle text-danger',
  warn: 'border-warn/40 bg-warn/10 text-warn',
  muted: 'border-line bg-panel text-muted',
} as const

interface BannerProps {
  tone: keyof typeof TONES
  icon?: ReactNode
  text: string
}

/** One line of outcome inside a dialog or a panel: a probe's result, a rejected statement. */
export function Banner({ tone, icon, text }: BannerProps) {
  return (
    <div
      role="status"
      className={cn(
        'flex items-start gap-2 rounded-md border px-2.5 py-2 text-xs',
        TONES[tone],
      )}
    >
      {icon ? <span className="mt-px shrink-0">{icon}</span> : null}
      <span className="selectable min-w-0">{text}</span>
    </div>
  )
}
