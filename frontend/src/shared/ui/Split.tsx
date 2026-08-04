import type { ReactNode } from 'react'
import { Group, Panel, Separator, useDefaultLayout, usePanelRef } from 'react-resizable-panels'

import { cn } from '@/lib/cn'

export { usePanelRef }
export type { PanelImperativeHandle } from 'react-resizable-panels'

export const SplitPanel = Panel

interface SplitProps {
  id: string
  orientation: 'horizontal' | 'vertical'
  panelIds: string[]
  className?: string
  children: ReactNode
}

export function Split({ id, orientation, panelIds, className, children }: SplitProps) {
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id,
    panelIds,
    storage: localStorage,
    onlySaveAfterUserInteractions: true,
  })

  return (
    <Group
      id={id}
      orientation={orientation}
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
      className={className}
    >
      {children}
    </Group>
  )
}

/**
 * The line between two panels, and the thing that is dragged.
 *
 * **The highlight follows `data-separator`, not `:hover`.** The library grabs the pointer within ten
 * pixels of a one-pixel line and reports that as a state on the element — so a CSS hover rule would
 * only fire on the pixel itself, and a reader who is *already dragging* would never have seen the
 * handle light up. `focus` is the same line under the keyboard, which resizes with the arrow keys.
 */
export function SplitHandle({ orientation }: { orientation: 'horizontal' | 'vertical' }) {
  return (
    <Separator
      className={cn(
        'shrink-0 bg-line transition-colors outline-none',
        'data-[separator=hover]:bg-accent',
        'data-[separator=active]:bg-accent',
        'data-[separator=focus]:bg-accent',
        orientation === 'horizontal' ? 'w-px cursor-col-resize' : 'h-px cursor-row-resize',
      )}
    />
  )
}
