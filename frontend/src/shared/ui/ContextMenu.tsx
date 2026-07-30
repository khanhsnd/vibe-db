import type { ReactNode } from 'react'
import * as Radix from '@radix-ui/react-context-menu'

import { cn } from '@/lib/cn'

export interface MenuItem {
  label: string
  onSelect: () => void
  disabled?: boolean
  danger?: boolean
  separatorBefore?: boolean
}

interface ContextMenuProps {
  items: MenuItem[]
  children: ReactNode
}

export function ContextMenu({ items, children }: ContextMenuProps) {
  if (items.length === 0) return <>{children}</>

  return (
    <Radix.Root>
      <Radix.Trigger asChild>{children}</Radix.Trigger>
      <Radix.Portal>
        <Radix.Content className="z-50 min-w-[168px] rounded-md border border-line-strong bg-elevated py-1 shadow-[var(--shadow-overlay)]">
          {items.map((item) => (
            <div key={item.label}>
              {item.separatorBefore ? <Radix.Separator className="my-1 h-px bg-line" /> : null}
              <Radix.Item
                disabled={item.disabled ?? false}
                onSelect={item.onSelect}
                className={cn(
                  'mx-1 cursor-default rounded px-2 py-1.5 text-sm outline-none',
                  'data-[disabled]:pointer-events-none data-[disabled]:opacity-40',
                  item.danger
                    ? 'text-danger data-[highlighted]:bg-danger-subtle'
                    : 'text-fg data-[highlighted]:bg-hover',
                )}
              >
                {item.label}
              </Radix.Item>
            </div>
          ))}
        </Radix.Content>
      </Radix.Portal>
    </Radix.Root>
  )
}
