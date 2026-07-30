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

export function SplitHandle({ orientation }: { orientation: 'horizontal' | 'vertical' }) {
  return (
    <Separator
      className={cn(
        'shrink-0 bg-line transition-colors hover:bg-accent active:bg-accent',
        orientation === 'horizontal' ? 'w-px cursor-col-resize' : 'h-px cursor-row-resize',
      )}
    />
  )
}
