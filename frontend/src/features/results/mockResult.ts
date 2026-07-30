import type { ResultSet } from './resultSet'

function buildRows(count: number) {
  const rows: ResultSet['rows'] = []
  for (let i = 1; i <= count; i++) {
    rows.push([
      String(i),
      `user${i}@example.com`,
      i % 7 === 0 ? null : `User ${i}`,
      i % 3 === 0 ? '1' : '0',
      `2026-0${(i % 9) + 1}-1${i % 9} 09:${String(i % 60).padStart(2, '0')}:00`,
    ])
  }
  return rows
}

export const MOCK_RESULT: ResultSet = {
  columns: [
    { name: 'id', type: 'BIGINT' },
    { name: 'email', type: 'VARCHAR' },
    { name: 'display_name', type: 'VARCHAR' },
    { name: 'is_active', type: 'TINYINT' },
    { name: 'created_at', type: 'DATETIME' },
  ],
  rows: buildRows(1000),
  elapsedMs: 8,
  truncated: true,
}
