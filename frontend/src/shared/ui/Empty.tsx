import type { ReactNode } from 'react'

interface EmptyProps {
  icon?: ReactNode
  title: string
  hint?: string
  action?: ReactNode
}

export function Empty({ icon, title, hint, action }: EmptyProps) {
  return (
    <div className="grid h-full place-items-center p-8">
      <div className="flex max-w-[280px] flex-col items-center gap-2 text-center">
        {icon ? <div className="text-faint">{icon}</div> : null}
        <p className="text-[13px] text-muted">{title}</p>
        {hint ? <p className="text-[12px] leading-relaxed text-faint">{hint}</p> : null}
        {action ? <div className="mt-1">{action}</div> : null}
      </div>
    </div>
  )
}
