import type { ComponentType, ReactNode } from 'react'
import * as Radix from '@radix-ui/react-context-menu'
import { ChevronRight } from 'lucide-react'

import { cn } from '@/lib/cn'

export interface MenuItem {
  label: string
  /** What the item does. An item carrying `items` opens them instead and needs none. */
  onSelect?: () => void
  icon?: ReactNode
  /** The key that invokes this item while its owner has focus. */
  shortcut?: string
  disabled?: boolean
  danger?: boolean
  separatorBefore?: boolean
  /** A submenu, for an action that has to be told which of several things it means. */
  items?: MenuItem[]
}

/**
 * The surface an overlay hung off a control sits on, without any padding of its own: a menu pads by
 * the row, a panel by the edge. Kept apart from `MENU_CONTENT` so a panel can wear the same shade,
 * border and shadow as the menu beside it without inheriting a menu's spacing — two padding
 * utilities on one element would be settled by the stylesheet's order rather than by the call site.
 */
export const MENU_SURFACE =
  'z-50 rounded-md border border-line-strong bg-elevated shadow-[var(--shadow-overlay)]'

/**
 * The look of a menu, in one place, so a menu raised by a right-click and one raised by a button
 * are the same object — see `MenuButton`, which draws the other half.
 */
export const MENU_CONTENT = `${MENU_SURFACE} min-w-[164px] py-1`

/**
 * A menu row is deliberately tighter than a form's: a right-click menu is read by scanning a
 * column of short labels, and a list of fourteen of them at a form's spacing is a list that runs
 * off the window and has to be scrolled to be read.
 */
export const menuItemClass = (danger: boolean | undefined) =>
  cn(
    'mx-1 flex cursor-default items-center gap-2 rounded px-2 py-1 text-xs outline-none',
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
    <span className={cn('grid h-3.5 w-3.5 shrink-0 place-items-center', !item.danger && 'text-muted')}>
      {item.icon}
    </span>
  )
}

export const isIconic = (items: MenuItem[]) => items.some((item) => item.icon)

/** What a row of a menu is given. A submenu's trigger is a div and takes no `onSelect`. */
interface RowProps {
  disabled?: boolean
  className?: string
  children?: ReactNode
}

/**
 * The Radix parts a menu is drawn from. `@radix-ui/react-context-menu` and
 * `@radix-ui/react-dropdown-menu` name the same set, so one renderer draws both and a submenu
 * added here is a submenu in either.
 */
export interface MenuParts {
  Item: ComponentType<RowProps & { onSelect?: (event: Event) => void }>
  Separator: ComponentType<{ className?: string }>
  Sub: ComponentType<{ children?: ReactNode }>
  SubTrigger: ComponentType<RowProps>
  /** A submenu is portalled like the menu itself, so nothing it hangs past can clip it. */
  Portal: ComponentType<{ children?: ReactNode }>
  SubContent: ComponentType<RowProps & { sideOffset?: number }>
}

/** An item that carries a submenu does nothing itself; Radix still wants a handler. */
const doNothing = () => {}

/** Every item of one menu, and — through itself — of every submenu under it. */
export function MenuItems({ parts, items }: { parts: MenuParts; items: MenuItem[] }) {
  const iconic = isIconic(items)

  return (
    <>
      {items.map((item, at) => (
        // By position as well as by label: a list of tabs is a list of menu items, and two of them
        // can be called the same thing.
        <div key={`${at}-${item.label}`}>
          {item.separatorBefore ? <parts.Separator className="my-0.5 h-px bg-line" /> : null}

          {item.items ? (
            <parts.Sub>
              <parts.SubTrigger
                disabled={item.disabled ?? false}
                className={menuItemClass(item.danger)}
              >
                <MenuGlyph item={item} iconic={iconic} />
                {item.label}
                {item.shortcut ? (
                  <span className="ml-auto text-2xs text-faint">{item.shortcut}</span>
                ) : null}
                <ChevronRight
                  size={12}
                  className={cn('shrink-0 text-faint', !item.shortcut && 'ml-auto')}
                />
              </parts.SubTrigger>
              <parts.Portal>
                <parts.SubContent sideOffset={2} className={MENU_CONTENT}>
                  <MenuItems parts={parts} items={item.items} />
                </parts.SubContent>
              </parts.Portal>
            </parts.Sub>
          ) : (
            <parts.Item
              disabled={item.disabled ?? false}
              onSelect={item.onSelect ?? doNothing}
              className={menuItemClass(item.danger)}
            >
              <MenuGlyph item={item} iconic={iconic} />
              {item.label}
              {item.shortcut ? (
                <span className="ml-auto text-2xs text-faint">{item.shortcut}</span>
              ) : null}
            </parts.Item>
          )}
        </div>
      ))}
    </>
  )
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
        <Radix.Content className={MENU_CONTENT}>
          <MenuItems parts={Radix} items={items} />
        </Radix.Content>
      </Radix.Portal>
    </Radix.Root>
  )
}
