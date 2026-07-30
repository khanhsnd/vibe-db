import { cn } from '@/lib/cn'

import type { ConnectionStatus } from './tree'

const STYLES: Record<ConnectionStatus, string> = {
  connected: 'bg-accent border-accent',
  connecting: 'bg-warn border-warn animate-pulse',
  error: 'bg-danger border-danger',
  idle: 'bg-transparent border-line-strong',
}

const LABELS: Record<ConnectionStatus, string> = {
  connected: 'Connected',
  connecting: 'Connecting',
  error: 'Last connection failed',
  idle: 'Not connected',
}

export function StatusDot({ status, className }: { status: ConnectionStatus; className?: string }) {
  return (
    <span
      role="img"
      aria-label={LABELS[status]}
      title={LABELS[status]}
      className={cn('h-2 w-2 shrink-0 rounded-full border', STYLES[status], className)}
    />
  )
}
