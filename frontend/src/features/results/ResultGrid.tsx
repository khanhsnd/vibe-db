import { useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

import { cn } from '@/lib/cn'
import { ROW_HEIGHT } from '@/shared/theme/density'

import { isNumeric, type Cell, type ResultSet } from './resultSet'

const GUTTER_WIDTH = 52
const DEFAULT_COLUMN_WIDTH = 160
const MIN_COLUMN_WIDTH = 60

export function ResultGrid({ result }: { result: ResultSet }) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [widths, setWidths] = useState<number[]>(() =>
    result.columns.map(() => DEFAULT_COLUMN_WIDTH),
  )

  const virtualizer = useVirtualizer({
    count: result.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 16,
  })

  const totalWidth = GUTTER_WIDTH + widths.reduce((sum, w) => sum + w, 0)

  const resizeColumn = (index: number, width: number) =>
    setWidths((prev) => prev.map((w, i) => (i === index ? Math.max(MIN_COLUMN_WIDTH, width) : w)))

  return (
    <div ref={scrollRef} className="scroll-thin h-full overflow-auto bg-canvas">
      <div className="w-full" style={{ minWidth: totalWidth }}>
        <HeaderRow columns={result.columns} widths={widths} onResize={resizeColumn} />

        <div className="relative" style={{ height: virtualizer.getTotalSize() }}>
          {virtualizer.getVirtualItems().map((item) => {
            const row = result.rows[item.index]
            if (!row) return null

            return (
              <div
                key={item.key}
                className="absolute top-0 left-0 flex w-full border-b border-line/60 hover:bg-hover"
                style={{ height: ROW_HEIGHT, transform: `translateY(${item.start}px)` }}
              >
                <div
                  className="sticky left-0 z-1 flex shrink-0 items-center justify-end border-r border-line bg-panel pr-2 font-mono text-[11px] text-faint"
                  style={{ width: GUTTER_WIDTH }}
                >
                  {item.index + 1}
                </div>

                {result.columns.map((column, i) => (
                  <CellView
                    key={column.name}
                    value={row[i] ?? null}
                    numeric={isNumeric(column.type)}
                    width={widths[i] ?? DEFAULT_COLUMN_WIDTH}
                  />
                ))}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

interface HeaderRowProps {
  columns: ResultSet['columns']
  widths: number[]
  onResize: (index: number, width: number) => void
}

function HeaderRow({ columns, widths, onResize }: HeaderRowProps) {
  return (
    <div className="sticky top-0 z-2 flex border-b border-line-strong bg-panel">
      <div
        className="sticky left-0 z-1 shrink-0 border-r border-line bg-panel"
        style={{ width: GUTTER_WIDTH, height: ROW_HEIGHT }}
      />

      {columns.map((column, i) => (
        <div
          key={column.name}
          className="relative flex shrink-0 items-center gap-1.5 border-r border-line px-2"
          style={{ width: widths[i] ?? DEFAULT_COLUMN_WIDTH, height: ROW_HEIGHT }}
        >
          <span className="truncate text-[12px] font-medium text-fg">{column.name}</span>
          <span className="ml-auto shrink-0 font-mono text-[10px] text-faint uppercase">
            {column.type}
          </span>
          <ResizeHandle
            onResize={(delta) => onResize(i, (widths[i] ?? DEFAULT_COLUMN_WIDTH) + delta)}
          />
        </div>
      ))}
    </div>
  )
}

function ResizeHandle({ onResize }: { onResize: (delta: number) => void }) {
  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    event.stopPropagation()
    let lastX = event.clientX

    const onMove = (moveEvent: PointerEvent) => {
      onResize(moveEvent.clientX - lastX)
      lastX = moveEvent.clientX
    }
    const onUp = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  return (
    <div
      onPointerDown={onPointerDown}
      className="absolute top-0 -right-px z-1 h-full w-1.5 cursor-col-resize hover:bg-accent"
    />
  )
}

function CellView({ value, numeric, width }: { value: Cell; numeric: boolean; width: number }) {
  return (
    <div
      className={cn(
        'flex shrink-0 items-center overflow-hidden border-r border-line/60 px-2',
        'selectable font-mono text-[12px] whitespace-nowrap',
        numeric && 'justify-end',
      )}
      style={{ width }}
    >
      {value === null ? (
        <span className="text-[11px] tracking-wide text-[var(--syn-null)] italic">NULL</span>
      ) : (
        <span className="truncate text-fg">{value}</span>
      )}
    </div>
  )
}
