import { Boxes, Database } from 'lucide-react'

import { namespaceOf, type Namespace } from '@/bridge/connections'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'

interface NamespacePickerProps {
  databases: string[]
  schemas: string[]
  namespace: Namespace
  /** Whether this engine has a schema below a database, and so a second picker. */
  hasSchemas: boolean
  onChange: (namespace: Namespace) => void
}

/**
 * Where a tab runs, picked on the tab's own strip: the database, and on an engine that has them
 * the schema inside it. It is one component because a query tab and a console ask the same
 * question — and answer it the same way, a database picked dropping the schema with it, since a
 * schema of one database names nothing in another.
 */
export function NamespacePicker({
  databases,
  schemas,
  namespace,
  hasSchemas,
  onChange,
}: NamespacePickerProps) {
  return (
    <>
      <Database size={13} className="shrink-0 text-kind-database" />
      <Select
        label="Database"
        value={namespace.database}
        options={databases}
        empty={databases.length === 0 ? 'Not connected' : 'No database'}
        onChange={(database) => onChange(namespaceOf(database))}
      />

      {hasSchemas ? (
        <>
          <Boxes size={13} className="shrink-0 text-kind-schema" />
          <Select
            label="Schema"
            value={namespace.schema}
            options={schemas}
            empty={namespace.database === '' ? 'No database' : 'No schema'}
            onChange={(schema) => onChange(namespaceOf(namespace.database, schema))}
          />
        </>
      ) : null}
    </>
  )
}

interface SelectProps {
  label: string
  value: string
  options: string[]
  empty: string
  onChange: (value: string) => void
}

function Select({ label, value, options, empty, onChange }: SelectProps) {
  return (
    <select
      aria-label={label}
      value={value}
      disabled={options.length === 0}
      onChange={(e) => onChange(e.target.value)}
      className={`${TOOLBAR_CONTROL} max-w-[180px] font-mono`}
    >
      <option value="">{empty}</option>
      {options.map((name) => (
        <option key={name} value={name}>
          {name}
        </option>
      ))}
    </select>
  )
}
