import { useEffect, useMemo, useState } from 'react'
import { Database } from 'lucide-react'

import { engineInfo } from '@/bridge/connections'
import {
  databases,
  emptyDatabaseSpec,
  NO_DATABASE_OPTIONS,
  type DatabaseOptions,
  type DatabaseSpec,
} from '@/bridge/databases'
import { ResultTabs, type ResultPaneDef } from '@/features/results/ResultTabs'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Checkbox, Field, SelectInput, TextArea, TextInput } from '@/shared/ui/Field'
import { ICON } from '@/shared/ui/menuIcons'
import { useConnectionsStore } from '@/stores/connections'
import type { DatabaseRequest } from '@/stores/schema'

import { useSchemaSubmit } from './useSchemaSubmit'

/** Tall enough that every pane is the same size, so switching tabs never moves the footer. */
const PANE_HEIGHT = 320

/**
 * A new database, declared. It is modal where a table's shape is a tab, because a database has
 * no tab of its own to open and its answers are one fixed screen rather than a list to work in.
 *
 * How much of it there is to fill in is the engine's: PostgreSQL's database is an object with an
 * owner, a locale and a connection limit, and can carry extensions and a comment; MySQL's is a
 * namespace with a character set; MongoDB's is neither, and is made by making its first collection —
 * a document store has no way to hold an empty database. **What the statement will be is never
 * written here** — the preview pane asks the driver for the statements it would run, so what is
 * shown and what runs are the same list.
 */
export function DatabaseDialog({
  request,
  onClose,
}: {
  request: DatabaseRequest
  onClose: () => void
}) {
  const sessionId = useConnectionsStore((s) => s.sessions[request.profileId]?.id ?? '')
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === request.profileId)?.engine ?? '',
  )
  const changeDatabases = useConnectionsStore((s) => s.changeDatabases)

  const [spec, setSpec] = useState<DatabaseSpec>(emptyDatabaseSpec)
  const [options, setOptions] = useState<DatabaseOptions>(NO_DATABASE_OPTIONS)
  const [sql, setSql] = useState<string[]>([])
  const { submit, running, error, fieldError } = useSchemaSubmit(onClose)

  const {
    databaseHasOwner: owned,
    databaseNeedsCollection: needsCollection,
    documents,
  } = engineInfo(engine)
  const set = <K extends keyof DatabaseSpec>(key: K, value: DatabaseSpec[K]) =>
    setSpec((prev) => ({ ...prev, [key]: value }) as DatabaseSpec)

  useEffect(() => {
    if (!sessionId) return
    let live = true
    void databases
      .options(sessionId)
      .then((read) => live && setOptions(read))
      .catch(() => {})
    return () => {
      live = false
    }
  }, [sessionId])

  // The preview follows every keystroke because it costs no round trip to the server: the driver
  // spells the statement from the spec alone.
  useEffect(() => {
    if (!sessionId) return
    let live = true
    void databases
      .preview(sessionId, spec)
      .then((statements) => live && setSql(statements))
      .catch(() => {})
    return () => {
      live = false
    }
  }, [sessionId, spec])

  // A MySQL collation is named after the character set it belongs to — `utf8mb4_general_ci` —
  // which narrows the second list to the first without asking the server again.
  const collations = useMemo(
    () =>
      spec.encoding === ''
        ? options.collations
        : options.collations.filter((name) => name.startsWith(`${spec.encoding}_`)),
    [options.collations, spec.encoding],
  )

  const panes: ResultPaneDef[] = [
    {
      id: 'general',
      label: 'General',
      content: (
        <div className="flex flex-col gap-2.5 px-1 py-3">
          <Field inline label="Database Name" error={fieldError('database')}>
            {(id, invalid) => (
              <TextInput
                id={id}
                invalid={invalid}
                value={spec.name}
                autoFocus
                onChange={(e) => set('name', e.target.value)}
              />
            )}
          </Field>

          {needsCollection ? (
            <Field
              inline
              label="First Collection"
              hint="a database exists only while it holds one"
              error={fieldError('collection')}
            >
              {(id, invalid) => (
                <TextInput
                  id={id}
                  invalid={invalid}
                  value={spec.firstCollection}
                  placeholder="data"
                  onChange={(e) => set('firstCollection', e.target.value)}
                />
              )}
            </Field>
          ) : owned ? (
            <>
              <Choice label="Owner" value={spec.owner} options={options.owners} onChange={(v) => set('owner', v)} />
              <Choice
                label="Template"
                value={spec.template}
                options={options.templates}
                onChange={(v) => set('template', v)}
              />
              <Choice
                label="Encoding"
                value={spec.encoding}
                options={options.encodings}
                onChange={(v) => set('encoding', v)}
              />
              <Field inline label="Collation Order">
                {(id) => (
                  <TextInput
                    id={id}
                    value={spec.collation}
                    onChange={(e) => set('collation', e.target.value)}
                  />
                )}
              </Field>
              <Field inline label="Character Classification">
                {(id) => (
                  <TextInput
                    id={id}
                    value={spec.ctype}
                    onChange={(e) => set('ctype', e.target.value)}
                  />
                )}
              </Field>
              <Choice
                label="Tablespace"
                value={spec.tablespace}
                options={options.tablespaces}
                onChange={(v) => set('tablespace', v)}
              />
              <Field inline label="Connection Limit" hint="-1 is no limit">
                {(id) => (
                  <TextInput
                    id={id}
                    value={String(spec.connectionLimit)}
                    onChange={(e) => set('connectionLimit', connectionLimit(e.target.value))}
                  />
                )}
              </Field>

              <div className="mt-1 flex flex-col gap-2">
                <Checkbox
                  label="Allow Connection"
                  checked={spec.allowConnections}
                  onChange={(e) => set('allowConnections', e.target.checked)}
                />
                <Checkbox
                  label="Is Template"
                  checked={spec.isTemplate}
                  onChange={(e) => set('isTemplate', e.target.checked)}
                />
              </div>
            </>
          ) : (
            <>
              <Choice
                label="Character Set"
                value={spec.encoding}
                options={options.encodings}
                onChange={(v) => setSpec((prev) => ({ ...prev, encoding: v, collation: '' }))}
              />
              <Choice
                label="Collation"
                value={spec.collation}
                options={collations}
                onChange={(v) => set('collation', v)}
              />
            </>
          )}
        </div>
      ),
    },
  ]

  if (owned) {
    panes.push(
      {
        id: 'extensions',
        label: 'Extensions',
        content: (
          <ExtensionPicker
            available={options.extensions}
            chosen={spec.extensions}
            onChange={(names) => set('extensions', names)}
          />
        ),
      },
      {
        id: 'comment',
        label: 'Comment',
        content: (
          <div className="h-full py-3">
            <TextArea
              value={spec.comment}
              placeholder="What this database is for"
              className="h-full"
              onChange={(e) => set('comment', e.target.value)}
            />
          </div>
        ),
      },
    )
  }

  panes.push({
    id: 'sql',
    label: documents ? 'Command Preview' : 'SQL Preview',
    content: (
      <div className="scroll-thin h-full overflow-auto py-3">
        {sql.length === 0 ? (
          <p className="text-sm text-faint">Name the database to see the statement.</p>
        ) : (
          <pre className="selectable font-mono text-xs leading-relaxed text-fg">
            {sql.map((statement) => `${statement};`).join('\n\n')}
          </pre>
        )}
      </div>
    ),
  })

  return (
    <Dialog
      open
      onOpenChange={onClose}
      title="New Database"
      icon={<Database size={15} />}
      width={620}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            icon={ICON.add}
            disabled={running}
            onClick={() =>
              void submit(() =>
                changeDatabases(request.profileId, (session) => databases.create(session, spec)),
              )
            }
          >
            {running ? 'Creating…' : 'Create'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <div style={{ height: PANE_HEIGHT }}>
          <ResultTabs panes={panes} />
        </div>
        {error && error.code !== 'INVALID_NAME' ? (
          <Banner tone="danger" text={error.message} />
        ) : null}
      </div>
    </Dialog>
  )
}

/**
 * One of the server's own names, or none: the blank entry is what leaves the clause out of the
 * statement altogether, so the server applies its own default rather than this dialog guessing it.
 */
function Choice({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: string[]
  onChange: (value: string) => void
}) {
  return (
    <Field inline label={label}>
      {(id) => (
        <SelectInput id={id} value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">Server default</option>
          {options.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </SelectInput>
      )}
    </Field>
  )
}

/** The extensions the server has to offer, of which the new database gets whichever are ticked. */
function ExtensionPicker({
  available,
  chosen,
  onChange,
}: {
  available: string[]
  chosen: string[]
  onChange: (names: string[]) => void
}) {
  const [filter, setFilter] = useState('')
  const shown = useMemo(() => {
    const wanted = filter.trim().toLowerCase()
    return wanted === '' ? available : available.filter((name) => name.toLowerCase().includes(wanted))
  }, [available, filter])

  return (
    <div className="flex h-full min-h-0 flex-col gap-2 py-3">
      <TextInput
        value={filter}
        placeholder={`Filter ${available.length} extensions`}
        onChange={(e) => setFilter(e.target.value)}
      />
      <div className="scroll-thin min-h-0 flex-1 overflow-y-auto rounded-md border border-line bg-canvas px-2 py-1.5">
        {shown.map((name) => (
          <div key={name} className="py-0.5">
            <Checkbox
              label={name}
              checked={chosen.includes(name)}
              onChange={(e) =>
                onChange(
                  e.target.checked ? [...chosen, name] : chosen.filter((held) => held !== name),
                )
              }
            />
          </div>
        ))}
      </div>
    </div>
  )
}

// A count that keeps its leading minus, since -1 is the value that means no limit. Anything that
// is not a number at all leaves the box at no limit rather than at zero, which would be a ban.
function connectionLimit(text: string): number {
  const digits = text.replace(/(?!^-)\D/g, '')
  const count = Number(digits)
  return digits === '' || digits === '-' || Number.isNaN(count) ? -1 : count
}
