import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { CalendarClock, FolderOpen, Loader2, Save } from 'lucide-react'

import { namespaceLabel, type Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { cadenceText, TASK_NOUN, WEEKDAYS, type TaskEvery, type TaskKind, type TaskRequest } from '@/bridge/schedule'
import { transfer, type TransferFormat } from '@/bridge/transfer'
import { ObjectPicker } from '@/features/backup/ObjectPicker'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Checkbox, Field, SelectInput, TextArea, TextInput } from '@/shared/ui/Field'
import { TabStrip } from '@/shared/ui/TabStrip'
import { databaseKey, namespaceKey, useConnectionsStore } from '@/stores/connections'
import { useSchedulesStore } from '@/stores/schedules'

const TABS = ['General', 'Schedule', 'What It Does'] as const
type TabName = (typeof TABS)[number]

const KINDS: TaskKind[] = ['backup', 'export', 'dump', 'compare']
const EVERY: Array<{ value: TaskEvery; label: string }> = [
  { value: 'minutes', label: 'Every N minutes' },
  { value: 'hourly', label: 'Hourly' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
]

const NO_NAMES: string[] = []

/**
 * One job on the clock, as the reader fills it in.
 *
 * **Tabs and not a wizard**, for the reason `New Backup` is: every tab is already answered, so a
 * reader who wants a nightly backup of everything presses Save on the screen it opened on. What each
 * tab holds is decided by the job's *kind* — the third one is the only place the four differ, and it
 * asks nothing the kind does not read, which is exactly what `schedule.Options.validate` refuses on.
 *
 * Nothing here re-derives what the Go side already decides: what a cadence *means* is
 * `schedule.Cadence`, and this only says it back in words.
 */
export function TaskDialog() {
  const editing = useSchedulesStore((s) => s.editing)
  const close = useSchedulesStore((s) => s.closeEditor)

  return (
    <Dialog
      open={editing !== null}
      onOpenChange={(open) => {
        if (!open) close()
      }}
      title={editing?.id === '' ? 'New Job' : 'Edit Job'}
      icon={<CalendarClock size={14} />}
      width={720}
      bodyClassName=""
      footer={null}
    >
      {/* Keyed on the job, so a dialog raised on a second one starts over rather than opening on
          the first one's answers. */}
      {editing ? <TaskForm key={editing.id || 'new'} initial={editing} onClose={close} /> : null}
    </Dialog>
  )
}

function TaskForm({ initial, onClose }: { initial: TaskRequest; onClose: () => void }) {
  const save = useSchedulesStore((s) => s.save)

  const [task, setTask] = useState<TaskRequest>(initial)
  const [at, setAt] = useState<TabName>('General')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const change = (edit: Partial<TaskRequest>) => setTask((current) => ({ ...current, ...edit }))

  const profiles = useConnectionsStore((s) => s.profiles)
  const connected = useConnectionsStore((s) => s.sessions[task.profileId] !== undefined)

  // What stops Save, said in one line rather than left for the Go side to refuse. It is the same
  // question `schedule.Task.Validate` answers — said here first because the boxes are still in front
  // of the reader, and answered there because that is where it is decided.
  const blocked =
    task.name.trim() === ''
      ? 'A job needs a name.'
      : task.profileId === ''
        ? 'Pick the connection this job runs against.'
        : task.namespace.database === ''
          ? 'Name the database this job runs in.'
          : blockedByKind(task)

  const submit = async () => {
    if (blocked !== '') return

    setSaving(true)
    setError(null)
    try {
      await save(task)
      onClose()
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col">
      <TabStrip tabs={TABS} at={at} onMove={setAt} />

      <div className="scroll-thin max-h-[60vh] min-h-[21rem] overflow-y-auto px-4 py-3.5">
        {at === 'General' ? (
          <GeneralTab task={task} profiles={profiles} onChange={change} />
        ) : at === 'Schedule' ? (
          <ScheduleTab task={task} onChange={change} />
        ) : (
          <WhatItDoesTab task={task} connected={connected} onChange={change} />
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
        <span className="mr-auto truncate text-xs text-muted">
          {error ? error.message : blocked !== '' ? blocked : cadenceText(task)}
        </span>

        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="primary"
          icon={saving ? <Loader2 size={12} className="animate-spin" /> : <Save size={12} />}
          disabled={saving || blocked !== ''}
          onClick={() => void submit()}
        >
          Save
        </Button>
      </div>
    </div>
  )
}

/** What the third tab still needs, per kind. The other two tabs' answers are checked above. */
function blockedByKind(task: TaskRequest): string {
  switch (task.kind) {
    case 'backup':
      return task.withStructure || task.withData
        ? ''
        : 'A backup has to hold the structure, the records, or both.'
    case 'export':
      return task.folder === '' ? 'Pick the folder the files go into.' : ''
    case 'dump':
      return task.folder === '' ? 'Pick the folder the dump goes into.' : ''
    case 'compare':
      return task.table === '' || task.right.table === ''
        ? 'Name both tables to compare.'
        : task.right.profileId === '' || task.right.database === ''
          ? 'Say where the other table is.'
          : task.withStructure || task.withData
            ? ''
            : 'A comparison has to read the structures, the records, or both.'
    default:
      return ''
  }
}

function GeneralTab({
  task,
  profiles,
  onChange,
}: {
  task: TaskRequest
  profiles: Array<{ id: string; name: string }>
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <Field label="Name" inline hint="What this job is called in the list.">
        {(id) => (
          <TextInput
            id={id}
            value={task.name}
            placeholder="Nightly backup of shop"
            onChange={(event) => onChange({ name: event.target.value })}
          />
        )}
      </Field>

      <Field label="Job" inline>
        {(id) => (
          <SelectInput
            id={id}
            value={task.kind}
            onChange={(event) => onChange({ kind: event.target.value as TaskKind })}
          >
            {KINDS.map((kind) => (
              <option key={kind} value={kind}>
                {TASK_NOUN[kind]}
              </option>
            ))}
          </SelectInput>
        )}
      </Field>

      <Field label="Connection" inline>
        {(id) => (
          <SelectInput
            id={id}
            value={task.profileId}
            onChange={(event) => onChange({ profileId: event.target.value })}
          >
            <option value="">Pick a connection…</option>
            {profiles.map((profile) => (
              <option key={profile.id} value={profile.id}>
                {profile.name}
              </option>
            ))}
          </SelectInput>
        )}
      </Field>

      <NamespaceFields
        profileId={task.profileId}
        namespace={task.namespace}
        onChange={(namespace) => onChange({ namespace })}
      />

      <div className="mt-1 flex flex-col gap-2.5 rounded-md border border-line p-3">
        <Checkbox
          label="Enabled — fire on the schedule"
          checked={task.enabled}
          onChange={(event) => onChange({ enabled: event.target.checked })}
        />
        <Checkbox
          label="Run a firing that was missed while the app was closed"
          checked={task.catchUp}
          onChange={(event) => onChange({ catchUp: event.target.checked })}
        />
        <p className="text-xs text-faint">
          A job fires only while vibe-db is open — nothing is installed on this machine and no
          service is left behind. Ticked, a job whose moment went by while the app was shut runs the
          next time it opens; left clear, it waits for its next moment.
        </p>
      </div>

      {/* A job dials with whatever the credential store holds, since nobody is there to be
          prompted. Saying so here is cheaper than a failed run at two in the morning. */}
      <PasswordNote profileId={task.profileId} />
    </div>
  )
}

/**
 * Where the job runs. A live connection offers the names it already read; a closed one is typed
 * into, because a job may perfectly well be written down for a connection nobody has opened today.
 */
function NamespaceFields({
  profileId,
  namespace,
  onChange,
}: {
  profileId: string
  namespace: Namespace
  onChange: (namespace: Namespace) => void
}) {
  const databases = useConnectionsStore((s) => s.databases[profileId] ?? NO_NAMES)
  const schemas = useConnectionsStore(
    (s) => s.schemas[databaseKey(profileId, namespace.database)] ?? NO_NAMES,
  )
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)

  useEffect(() => {
    if (profileId !== '' && namespace.database !== '') void loadSchemas(profileId, namespace.database)
  }, [loadSchemas, profileId, namespace.database])

  return (
    <>
      <Field label="Database" inline>
        {(id) => (
          <NameInput
            id={id}
            value={namespace.database}
            options={databases}
            placeholder="shop"
            onChange={(database) => onChange({ ...namespace, database })}
          />
        )}
      </Field>

      <Field
        label="Schema"
        inline
        hint={schemas.length === 0 ? 'Leave empty where the engine has no schemas.' : undefined}
      >
        {(id) => (
          <NameInput
            id={id}
            value={namespace.schema}
            options={schemas}
            placeholder="public"
            onChange={(schema) => onChange({ ...namespace, schema })}
          />
        )}
      </Field>
    </>
  )
}

/**
 * A name the app may already know: a list to pick from where it does, a box to type in where it does
 * not. One control rather than two, so every "which database" and "which table" in this dialog reads
 * the same whether or not the connection happens to be open.
 */
function NameInput({
  id,
  value,
  options,
  placeholder,
  onChange,
}: {
  id: string
  value: string
  options: string[]
  placeholder: string
  onChange: (value: string) => void
}) {
  if (options.length === 0) {
    return (
      <TextInput
        id={id}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    )
  }

  return (
    <SelectInput id={id} value={value} onChange={(event) => onChange(event.target.value)}>
      <option value="">Pick one…</option>
      {/* A name saved when the connection was open but gone from the server since still shows,
          rather than silently reading as the first name in the list. */}
      {(options.includes(value) || value === '' ? options : [value, ...options]).map((name) => (
        <option key={name} value={name}>
          {name}
        </option>
      ))}
    </SelectInput>
  )
}

function PasswordNote({ profileId }: { profileId: string }) {
  const profile = useConnectionsStore((s) => s.profiles.find((one) => one.id === profileId) ?? null)
  if (!profile || profile.hasPassword || profile.auth !== 'password') return null

  return (
    <Banner
      tone="warn"
      text="This connection has no saved password, so the job will dial it without one — nobody is there to be prompted. Save the password on the connection if the server needs one."
    />
  )
}

function ScheduleTab({
  task,
  onChange,
}: {
  task: TaskRequest
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <Field label="Repeat" inline>
        {(id) => (
          <SelectInput
            id={id}
            value={task.every}
            onChange={(event) => onChange({ every: event.target.value as TaskEvery })}
          >
            {EVERY.map((one) => (
              <option key={one.value} value={one.value}>
                {one.label}
              </option>
            ))}
          </SelectInput>
        )}
      </Field>

      {task.every === 'minutes' ? (
        <Field label="Minutes apart" inline>
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={1}
              value={task.minutes}
              onChange={(event) => onChange({ minutes: Number(event.target.value) })}
            />
          )}
        </Field>
      ) : null}

      {task.every === 'weekly' ? (
        <Field label="Day" inline>
          {(id) => (
            <SelectInput
              id={id}
              value={task.weekday}
              onChange={(event) => onChange({ weekday: Number(event.target.value) })}
            >
              {WEEKDAYS.map((day, at) => (
                <option key={day} value={at}>
                  {day}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>
      ) : null}

      {task.every === 'daily' || task.every === 'weekly' ? (
        <Field label="Hour" inline hint="This machine's own clock, on the 24-hour dial.">
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={0}
              max={23}
              value={task.hour}
              onChange={(event) => onChange({ hour: Number(event.target.value) })}
            />
          )}
        </Field>
      ) : null}

      {task.every !== 'minutes' ? (
        <Field label="Minute" inline>
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={0}
              max={59}
              value={task.minute}
              onChange={(event) => onChange({ minute: Number(event.target.value) })}
            />
          )}
        </Field>
      ) : null}

      <p className="mt-2 rounded-md border border-line bg-bar px-3 py-2 text-sm text-accent">
        {cadenceText(task)}
      </p>
      <p className="text-xs text-faint">
        A saved job counts from the moment it is saved, so editing one at noon never fires it at
        noon. Run Now fires it immediately, exactly as the clock would.
      </p>
    </div>
  )
}

function WhatItDoesTab({
  task,
  connected,
  onChange,
}: {
  task: TaskRequest
  connected: boolean
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  switch (task.kind) {
    case 'export':
      return <ExportOptions task={task} connected={connected} onChange={onChange} />
    case 'dump':
      return <DumpOptions task={task} connected={connected} onChange={onChange} />
    case 'compare':
      return <CompareOptions task={task} onChange={onChange} />
    default:
      return <BackupOptions task={task} connected={connected} onChange={onChange} />
  }
}

function BackupOptions({
  task,
  connected,
  onChange,
}: {
  task: TaskRequest
  connected: boolean
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <Halves task={task} onChange={onChange} />

      <Field
        label="Keep"
        inline
        hint="How many backups to leave in this namespace's folder, oldest deleted first. 0 keeps every one."
      >
        {(id) => (
          <TextInput
            id={id}
            type="number"
            min={0}
            value={task.keep}
            onChange={(event) => onChange({ keep: Number(event.target.value) })}
          />
        )}
      </Field>

      <Field label="Comment" inline hint="Kept in the file's own header. Empty uses the job's name.">
        {(id) => (
          <TextArea
            id={id}
            value={task.comment}
            className="min-h-[3.5rem]"
            onChange={(event) => onChange({ comment: event.target.value })}
          />
        )}
      </Field>

      <TableChoice task={task} connected={connected} onChange={onChange} />
    </div>
  )
}

function ExportOptions({
  task,
  connected,
  onChange,
}: {
  task: TaskRequest
  connected: boolean
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  const [formats, setFormats] = useState<TransferFormat[]>([])

  useEffect(() => {
    void transfer
      .listFormats()
      .then((all) => setFormats(all.filter((format) => format.writes)))
      .catch(() => setFormats([]))
  }, [])

  return (
    <div className="flex flex-col gap-3">
      <Field label="Format" inline>
        {(id) => (
          <SelectInput
            id={id}
            value={task.format}
            onChange={(event) => onChange({ format: event.target.value })}
          >
            {formats.length === 0 ? <option value={task.format}>{task.format}</option> : null}
            {formats.map((format) => (
              <option key={format.value} value={format.value}>
                {format.label}
              </option>
            ))}
          </SelectInput>
        )}
      </Field>

      <FolderField
        label="Folder"
        hint="One file per table, named after the table. Each run writes over the last."
        value={task.folder}
        onChange={(folder) => onChange({ folder })}
      />

      <Field label="Rows per table" inline hint="0 is every row.">
        {(id) => (
          <TextInput
            id={id}
            type="number"
            min={0}
            value={task.maxRows}
            onChange={(event) => onChange({ maxRows: Number(event.target.value) })}
          />
        )}
      </Field>

      <p className="text-xs text-faint">
        The file is written with the wizard's own defaults — UTF-8, comma-separated, with a header
        row. A job that needs anything else is an export run by hand.
      </p>

      <TableChoice task={task} connected={connected} onChange={onChange} />
    </div>
  )
}

function DumpOptions({
  task,
  connected,
  onChange,
}: {
  task: TaskRequest
  connected: boolean
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <FolderField
        label="Folder"
        hint="One file per run, named for the namespace and the moment — so two runs never write over each other."
        value={task.folder}
        onChange={(folder) => onChange({ folder })}
      />

      <Halves task={task} onChange={onChange} />

      <Checkbox
        label="Compress, where the program can"
        checked={task.compress}
        onChange={(event) => onChange({ compress: event.target.checked })}
      />

      <p className="text-xs text-faint">
        This runs the engine's own program — mongodump, mysqldump or pg_dump — which has to be on
        this machine. A connection through an SSH tunnel or a cloud identity cannot be dumped this
        way, and the job says so rather than running against the wrong host.
      </p>

      <TableChoice task={task} connected={connected} onChange={onChange} />
    </div>
  )
}

function CompareOptions({
  task,
  onChange,
}: {
  task: TaskRequest
  onChange: (edit: Partial<TaskRequest>) => void
}) {
  const profiles = useConnectionsStore((s) => s.profiles)
  const tables = useConnectionsStore(
    (s) => s.objects[namespaceKey(task.profileId, task.namespace)]?.tables ?? NO_NAMES,
  )
  const rightNamespace = { database: task.right.database, schema: task.right.schema }
  const rightTables = useConnectionsStore(
    (s) => s.objects[namespaceKey(task.right.profileId, rightNamespace)]?.tables ?? NO_NAMES,
  )

  const changeRight = (edit: Partial<TaskRequest['right']>) =>
    onChange({ right: { ...task.right, ...edit } })

  return (
    <div className="flex flex-col gap-3">
      <p className="text-xs text-faint">
        A comparison writes nothing. It reads both tables and reports what has drifted apart, which
        the list draws as <span className="text-fg">Differs</span> rather than as a failure.
      </p>

      <Field label="This table" inline hint={namespaceLabel(task.namespace)}>
        {(id) => (
          <NameInput
            id={id}
            value={task.table}
            options={tables}
            placeholder="users"
            onChange={(table) => onChange({ table })}
          />
        )}
      </Field>

      <div className="flex flex-col gap-3 rounded-md border border-line p-3">
        <p className="text-xs font-medium text-muted">Against</p>

        <Field label="Connection" inline>
          {(id) => (
            <SelectInput
              id={id}
              value={task.right.profileId}
              onChange={(event) => changeRight({ profileId: event.target.value })}
            >
              <option value="">Pick a connection…</option>
              {profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.name}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>

        <NamespaceFields
          profileId={task.right.profileId}
          namespace={rightNamespace}
          onChange={(namespace) =>
            changeRight({ database: namespace.database, schema: namespace.schema })
          }
        />

        <Field label="Table" inline>
          {(id) => (
            <NameInput
              id={id}
              value={task.right.table}
              options={rightTables}
              placeholder="users"
              onChange={(table) => changeRight({ table })}
            />
          )}
        </Field>
      </div>

      <Halves task={task} onChange={onChange} compare />

      <Field label="Rows per side" inline hint="0 reads the first 1000, which is what a comparison is bounded at.">
        {(id) => (
          <TextInput
            id={id}
            type="number"
            min={0}
            value={task.maxRows}
            onChange={(event) => onChange({ maxRows: Number(event.target.value) })}
          />
        )}
      </Field>
    </div>
  )
}

/** The two halves every kind but the export asks about, in one place rather than in four. */
function Halves({
  task,
  onChange,
  compare = false,
}: {
  task: TaskRequest
  onChange: (edit: Partial<TaskRequest>) => void
  compare?: boolean
}) {
  return (
    <div className="flex flex-col gap-2.5">
      <Checkbox
        label={compare ? 'Compare the structures' : 'Include structure'}
        checked={task.withStructure}
        onChange={(event) => onChange({ withStructure: event.target.checked })}
      />
      <Checkbox
        label={compare ? 'Compare the records' : 'Include records'}
        checked={task.withData}
        onChange={(event) => onChange({ withData: event.target.checked })}
      />
    </div>
  )
}

/**
 * Which tables the job covers. **Empty is every table the namespace holds**, read at the moment the
 * job fires — so a table added next week is in next week's backup without anybody editing the job.
 */
function TableChoice({
  task,
  connected,
  onChange,
}: {
  task: TaskRequest
  connected: boolean
  onChange: (edit: Partial<TaskRequest>) => void
}): ReactNode {
  const tables = useConnectionsStore(
    (s) => s.objects[namespaceKey(task.profileId, task.namespace)]?.tables ?? NO_NAMES,
  )
  const loadObjects = useConnectionsStore((s) => s.loadObjects)

  useEffect(() => {
    if (connected && task.namespace.database !== '') {
      void loadObjects(task.profileId, task.namespace)
    }
  }, [connected, loadObjects, task.profileId, task.namespace])

  const excluded = useMemo(
    () => new Set(task.tables.length === 0 ? [] : tables.filter((one) => !task.tables.includes(one))),
    [tables, task.tables],
  )

  if (tables.length === 0) {
    return (
      <p className="text-xs text-faint">
        This job covers every table the namespace holds. Open this connection to pick a few of them
        instead.
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted">Tables</span>
      <ObjectPicker
        tables={tables}
        excluded={excluded}
        onToggle={(table) => onChange({ tables: toggleTable(task.tables, tables, table) })}
        onAll={() => onChange({ tables: [] })}
        onNone={() => onChange({ tables: [] })}
      />
      <p className="text-xs text-faint">
        {task.tables.length === 0
          ? 'Every table, whatever the namespace holds when the job fires.'
          : `${task.tables.length} of ${tables.length} tables.`}
      </p>
    </div>
  )
}

/** One table ticked or unticked, in the encoding the wire uses: **empty is all of them**. */
function toggleTable(picked: string[], all: string[], table: string): string[] {
  const chosen = new Set(picked.length === 0 ? all : picked)
  if (chosen.has(table)) chosen.delete(table)
  else chosen.add(table)

  const kept = all.filter((one) => chosen.has(one))
  return kept.length === all.length ? [] : kept
}

function FolderField({
  label,
  hint,
  value,
  onChange,
}: {
  label: string
  hint: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <Field label={label} inline hint={hint}>
      {(id) => (
        <div className="flex items-center gap-2">
          <TextInput
            id={id}
            value={value}
            placeholder="C:\\backups"
            onChange={(event) => onChange(event.target.value)}
          />
          <Button
            icon={<FolderOpen size={13} />}
            onClick={() => {
              void transfer.chooseFolder().then((folder) => {
                // A dismissed picker changes nothing, which is what an empty path means everywhere
                // in this app.
                if (folder !== '') onChange(folder)
              })
            }}
          >
            Browse
          </Button>
        </div>
      )}
    </Field>
  )
}
