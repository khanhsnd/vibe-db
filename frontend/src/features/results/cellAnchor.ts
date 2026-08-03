import { useLayoutEffect, useState, type CSSProperties, type RefObject } from 'react'

/**
 * Where an editor's cell is on screen. An overlay over the grid is portalled and fixed — the grid
 * scrolls and would otherwise clip it — so it has to be told where its cell is, and told again on
 * every scroll under it, which is what keeps the two together instead of dismissing one of them.
 */
export function useCellAnchor(box: RefObject<HTMLElement | null>): DOMRect | null {
  const [anchor, setAnchor] = useState<DOMRect | null>(null)

  useLayoutEffect(() => {
    const place = () => setAnchor(box.current?.getBoundingClientRect() ?? null)

    place()
    window.addEventListener('scroll', place, true)
    window.addEventListener('resize', place)
    return () => {
      window.removeEventListener('scroll', place, true)
      window.removeEventListener('resize', place)
    }
  }, [box])

  return anchor
}

/** An overlay hangs under its cell, and flips above it where the screen below has no room. */
export function overlayStyle(anchor: DOMRect, height: number, minWidth: number): CSSProperties {
  return {
    left: anchor.left,
    minWidth: Math.max(anchor.width, minWidth),
    maxHeight: height,
    ...(anchor.bottom + height > window.innerHeight && anchor.top > height
      ? { bottom: window.innerHeight - anchor.top }
      : { top: anchor.bottom }),
  }
}
