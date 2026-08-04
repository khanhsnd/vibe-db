import type { ReactNode } from 'react'
import * as Radix from '@radix-ui/react-dropdown-menu'

import { MenuItems, MENU_CONTENT, type MenuItem } from './ContextMenu'

interface MenuButtonProps {
  items: MenuItem[]
  /** Which edge of the trigger the menu hangs from; `end` for a trigger against the right. */
  align?: 'start' | 'end'
  /** The control that opens it — an `IconButton` or a `Button`, given the trigger's behaviour. */
  children: ReactNode
}

/**
 * The same menu a right-click raises, raised by a button instead. It shares its item type and its
 * look with `ContextMenu`, so an action offered both ways is one object drawn one way.
 */
export function MenuButton({ items, align = 'start', children }: MenuButtonProps) {
  if (items.length === 0) return <>{children}</>

  return (
    <Radix.Root>
      <Radix.Trigger asChild>{children}</Radix.Trigger>
      <Radix.Portal>
        <Radix.Content align={align} sideOffset={4} className={MENU_CONTENT}>
          <MenuItems parts={Radix} items={items} />
        </Radix.Content>
      </Radix.Portal>
    </Radix.Root>
  )
}
