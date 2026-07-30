let context: CanvasRenderingContext2D | null = null

function measurer(): CanvasRenderingContext2D | null {
  context ??= document.createElement('canvas').getContext('2d')
  return context
}

export function gridFont(): string {
  const style = getComputedStyle(document.documentElement)
  const size = style.getPropertyValue('--text-sm').trim()
  const family = style.getPropertyValue('--font-mono').trim()
  return `${size} ${family}`
}

export function widestText(texts: string[], font: string): number {
  const target = measurer()
  if (!target) return 0

  target.font = font

  let widest = 0
  for (const text of texts) {
    widest = Math.max(widest, target.measureText(text).width)
  }
  return Math.ceil(widest)
}
