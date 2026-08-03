import type { ReactNode } from 'react'
import * as Radix from '@radix-ui/react-context-menu'

import { cn } from '@/lib/cn'

export interface MenuItem {
  label: string
  onSelect: () => void
  icon?: ReactNode
  disabled?: boolean
  danger?: boolean
  separatorBefore?: boolean
}

/**
 * The look of a menu, in one place, so a menu raised by a right-click and one raised by a button
 * are the same object — see `MenuButton`, which draws the other half.
 */
export const MENU_CONTENT =
  'z-50 min-w-[168px] rounded-md border border-line-strong bg-elevated py-1 shadow-[var(--shadow-overlay)]'

export const menuItemClass = (danger: boolean | undefined) =>
  cn(
    'mx-1 flex cursor-default items-center gap-2 rounded px-2 py-1.5 text-sm outline-none',
    'data-[disabled]:pointer-events-none data-[disabled]:opacity-40',
    danger ? 'text-danger data-[highlighted]:bg-danger-subtle' : 'text-fg data-[highlighted]:bg-hover',
  )

/**
 * One menu draws one column of icons: an item without a glyph keeps the gutter so every label
 * starts on the same line, but a menu with no glyphs at all keeps its old width.
 */
export function MenuGlyph({ item, iconic }: { item: MenuItem; iconic: boolean }) {
  if (!iconic) return null

  return (
    <span className={cn('grid h-3.5 w-3.5 shrink-0 place-items-center', !item.danger && 'text-faint')}>
      {item.icon}
    </span>
  )
}

export const isIconic = (items: MenuItem[]) => items.some((item) => item.icon)

interface ContextMenuProps {
  items: MenuItem[]
  children: ReactNode
}

export function ContextMenu({ items, children }: ContextMenuProps) {
  if (items.length === 0) return <>{children}</>

  const iconic = isIconic(items)

  return (
    <Radix.Root>
      <Radix.Trigger asChild>{children}</Radix.Trigger>
      <Radix.Portal>
        <Radix.Content className={MENU_CONTENT}>
          {items.map((item) => (
            <div key={item.label}>
              {item.separatorBefore ? <Radix.Separator className="my-1 h-px bg-line" /> : null}
              <Radix.Item
                disabled={item.disabled ?? false}
                onSelect={item.onSelect}
                className={menuItemClass(item.danger)}
              >
                <MenuGlyph item={item} iconic={iconic} />
                {item.label}
              </Radix.Item>
            </div>
          ))}
        </Radix.Content>
      </Radix.Portal>
    </Radix.Root>
  )
}
