import { cellText, selectedBlocks, type CellBlock, type GridData, type Selection } from './resultSet'

/**
 * The shapes a selection can be copied in beyond a plain tab-separated block. They are the
 * clipboard's half of what `ExportMenu` writes to a file, and treat a NULL the same way: an
 * empty CSV field, a JSON `null` — see features.md, *Exporting a grid*.
 */
export type CopyFormat = 'names' | 'csv' | 'json'

export const COPY_FORMATS: ReadonlyArray<{ value: CopyFormat; label: string }> = [
  { value: 'names', label: 'Tab-separated with field names' },
  { value: 'csv', label: 'CSV' },
  { value: 'json', label: 'JSON' },
]

export function copyAsText(format: CopyFormat, selection: Selection, data: GridData): string {
  const blocks = selectedBlocks(selection, data)

  switch (format) {
    case 'names':
      return blocks.map(withNames).join('\n')
    case 'csv':
      return blocks.map(asCSV).join('\n')
    case 'json':
      return JSON.stringify(blocks.flatMap(asObjects), null, 2)
  }
}

function withNames(block: CellBlock): string {
  const lines = [block.columns.join('\t')]
  for (const row of block.rows) lines.push(row.map(cellText).join('\t'))

  return lines.join('\n')
}

function asCSV(block: CellBlock): string {
  const lines = [block.columns.map(quoted).join(',')]
  for (const row of block.rows) lines.push(row.map((cell) => quoted(cell ?? '')).join(','))

  return lines.join('\n')
}

/** A field is quoted only where it has to be, which is what any CSV reader expects. */
function quoted(value: string): string {
  return /["\n\r,]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value
}

function asObjects(block: CellBlock): Array<Record<string, string | null>> {
  return block.rows.map((row) => {
    const object: Record<string, string | null> = {}
    block.columns.forEach((name, index) => {
      object[name] = row[index] ?? null
    })
    return object
  })
}
