import { useCallback, useEffect, useMemo, useState } from 'react'
import { FolderOpen, Upload } from 'lucide-react'

import { engineWords, namespaceLabel, type Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { queries } from '@/bridge/query'
import {
  defaultTransferOptions,
  FIELD_DELIMITERS,
  IMPORT_MODES,
  importMode,
  transfer,
  TEXT_QUALIFIERS,
  type FieldMapping,
  type ImportMode,
  type ImportPreview,
  type ImportResult,
  type TransferEncoding,
  type TransferFormat,
  type TransferOptions,
} from '@/bridge/transfer'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { CellRef, GridData } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { Empty } from '@/shared/ui/Empty'
import { namespaceKey, tableKey, useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'

import { RadioList } from './RadioList'
import { TransferLog } from './TransferLog'
import { Wizard, type WizardStep } from './Wizard'

/** Where the wizard is pointed, and non-null exactly while it is up. */
export interface ImportTarget {
  profileId: string
  namespace: Namespace
  /** The table to import into, and empty for a wizard opened on the `Tables` folder. */
  table: string
}

const WIDTH = 820
const NO_TABLES: string[] = []

/**
 * Reads a file into a table, one question to a screen.
 *
 * The order of the questions is the order the answers depend on each other in: what the file *is*,
 * then where it is, then how its bytes are cut into fields, then which of those fields goes into
 * which column, and only then what the rows are to *do* when they get there. Nothing reaches a
 * server until Start — every step before it reads the file and nothing else, which is why the
 * preview needs no session at all.
 *
 * **Nothing here builds a statement.** The rows go to the Go side as text and come back out of the
 * driver as quoted literals, which is what stands between a spreadsheet a colleague mailed over and
 * the server it is being poured into.
 */
export function ImportWizard({
  target,
  onOpenChange,
}: {
  target: ImportTarget | null
  onOpenChange: (open: boolean) => void
}) {
  const profile = useConnectionsStore((s) => s.profiles.find((p) => p.id === target?.profileId))
  const sessionId = useConnectionsStore((s) =>
    target ? (s.sessions[target.profileId]?.id ?? null) : null,
  )
  const tables = useConnectionsStore((s) =>
    target ? (s.objects[namespaceKey(target.profileId, target.namespace)]?.tables ?? NO_TABLES) : NO_TABLES,
  )
  const loadStructure = useConnectionsStore((s) => s.loadStructure)
  const ask = useSchemaStore((s) => s.ask)

  const [formats, setFormats] = useState<TransferFormat[]>([])
  const [encodings, setEncodings] = useState<TransferEncoding[]>([])

  const [format, setFormat] = useState('csv')
  const [path, setPath] = useState('')
  const [options, setOptions] = useState<TransferOptions>(defaultTransferOptions)
  const [sheets, setSheets] = useState<string[]>([])
  const [table, setTable] = useState('')
  const [mappings, setMappings] = useState<FieldMapping[]>([])
  const [mode, setMode] = useState<ImportMode>('append')
  const [batch, setBatch] = useState(200)
  const [continueOnError, setContinueOnError] = useState(true)

  const [at, setAt] = useState(0)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [runId, setRunId] = useState('')
  const [result, setResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const structure = useConnectionsStore((s) =>
    target && table ? s.structures[tableKey(target.profileId, target.namespace, table)] : undefined,
  )

  const words = engineWords(profile?.engine ?? '')
  const chosen = formats.find((one) => one.value === format)
  const running = runId !== ''

  useEffect(() => {
    void transfer.listFormats().then(setFormats).catch(() => {})
    void transfer.listEncodings().then(setEncodings).catch(() => {})
  }, [])

  // Opening starts where the reader already is — the table they right-clicked, if they did — and
  // forgets everything about the last import, which was about another file.
  useEffect(() => {
    if (!target) return
    setAt(0)
    setPath('')
    setPreview(null)
    setResult(null)
    setError(null)
    setSheets([])
    setMappings([])
    setTable(target.table)
  }, [target])

  useEffect(() => {
    if (target && table) void loadStructure(target.profileId, target.namespace, table)
  }, [target, table, loadStructure])

  const browse = () => {
    void transfer
      .chooseFile(format)
      .then(async (picked) => {
        if (picked === '') return
        setPath(picked)
        setPreview(null)
        setSheets(chosen?.value === 'xlsx' ? await transfer.listWorksheets(picked) : [])
      })
      .catch((thrown) => setError(toAppError(thrown)))
  }

  // The preview is re-read whenever an answer that changes what the file *says* changes, which is
  // every answer on the three steps before the mapping. It is what those steps are checked by: a
  // delimiter that is wrong shows as one column holding the whole line.
  useEffect(() => {
    if (path === '') return

    let live = true
    void transfer
      .preview({ format, path, options })
      .then((read) => live && (setPreview(read), setError(null)))
      .catch((thrown) => live && (setPreview(null), setError(toAppError(thrown))))

    return () => {
      live = false
    }
  }, [format, path, options])

  // A mapping is proposed rather than typed: a file whose fields are named after the columns — which
  // is nearly every file exported from anything — arrives already mapped, and the key is the table's
  // own primary key. The reader then corrects what the guess got wrong instead of filling in twenty
  // rows by hand.
  useEffect(() => {
    if (!structure || !preview) return

    const fields = new Map(preview.columns.map((name) => [name.toLowerCase(), name]))
    setMappings(
      structure.columns.map((column) => ({
        source: fields.get(column.name.toLowerCase()) ?? '',
        target: column.name,
        key: column.primaryKey,
      })),
    )
  }, [structure, preview])

  const request = useMemo(
    () => ({
      namespace: target?.namespace ?? { database: '', schema: '' },
      runId,
      format,
      path,
      options,
      table,
      mode,
      mappings,
      batch,
      continueOnError,
    }),
    [target, runId, format, path, options, table, mode, mappings, batch, continueOnError],
  )

  const start = useCallback(async () => {
    if (sessionId === null || running) return

    const id = crypto.randomUUID()
    const run = async () => {
      setRunId(id)
      setError(null)
      setResult(null)
      try {
        setResult(await transfer.runImport(sessionId, { ...request, runId: id }))
      } catch (thrown) {
        setError(toAppError(thrown))
      } finally {
        setRunId('')
      }
    }

    // Whether this has to be answered for first is the *Go side's* answer — the mode's hazard
    // against the connection's environment — so the wizard asks rather than deciding.
    try {
      const review = await transfer.reviewImport(sessionId, { ...request, runId: id })
      if (!review.confirm) {
        void run()
        return
      }
      ask({
        kind: 'confirm',
        title: 'Run this import',
        message: confirmText(mode, table, review.environment, words.rows),
        submitLabel: 'Import',
        danger: true,
        onSubmit: run,
      })
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }, [sessionId, running, request, ask, mode, table, words])

  const steps = useMemo(
    () =>
      importSteps({
        formats,
        encodings,
        chosen,
        format,
        setFormat,
        path,
        browse,
        setPath,
        options,
        setOptions,
        sheets,
        tables,
        table,
        setTable,
        preview,
        mappings,
        setMappings,
        mode,
        setMode,
        batch,
        setBatch,
        continueOnError,
        setContinueOnError,
        result,
        running,
        words: words.table,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      formats, encodings, chosen, format, path, options, sheets, tables, table,
      preview, mappings, mode, batch, continueOnError, result, running, words,
    ],
  )

  return (
    <Wizard
      open={target !== null}
      onOpenChange={onOpenChange}
      title="Import Wizard"
      description={
        target ? `${profile?.name ?? ''} · ${namespaceLabel(target.namespace)}` : ''
      }
      icon={<Upload size={15} />}
      steps={steps}
      at={at}
      onMove={setAt}
      startLabel="Start"
      running={running}
      finished={result !== null}
      onStart={() => void start()}
      onStop={() => void queries.cancel(runId)}
      summary={summary(result, running)}
      error={error?.message ?? null}
      width={WIDTH}
    />
  )
}

interface StepInput {
  formats: TransferFormat[]
  encodings: TransferEncoding[]
  chosen: TransferFormat | undefined
  format: string
  setFormat: (value: string) => void
  path: string
  browse: () => void
  setPath: (value: string) => void
  options: TransferOptions
  setOptions: (value: TransferOptions) => void
  sheets: string[]
  tables: string[]
  table: string
  setTable: (value: string) => void
  preview: ImportPreview | null
  mappings: FieldMapping[]
  setMappings: (value: FieldMapping[]) => void
  mode: ImportMode
  setMode: (value: ImportMode) => void
  batch: number
  setBatch: (value: number) => void
  continueOnError: boolean
  setContinueOnError: (value: boolean) => void
  result: ImportResult | null
  running: boolean
  words: string
}

/**
 * The steps, in order — and **the delimiter step is left out entirely** for a format that says
 * inside itself where a field ends. A JSON file has no delimiter to be wrong about, and a step of
 * greyed-out boxes says only that the wizard was written for something else.
 */
function importSteps(input: StepInput): WizardStep[] {
  const set = (patch: Partial<TransferOptions>) => input.setOptions({ ...input.options, ...patch })

  const steps: WizardStep[] = [
    {
      prompt: 'Which format is the file you are importing?',
      body: (
        <RadioList
          name="Import format"
          value={input.format}
          onChange={input.setFormat}
          options={input.formats
            .filter((one) => one.reads)
            .map((one) => ({ value: one.value, label: `${one.label} (*.${one.extension})` }))}
        />
      ),
    },
    {
      prompt: 'Which file holds the data?',
      blocked: input.path === '' ? 'Pick a file to go on.' : undefined,
      body: (
        <div className="flex max-w-[620px] flex-col gap-3">
          <Field label="File" inline>
            {(id) => (
              <div className="flex items-center gap-2">
                <TextInput
                  id={id}
                  value={input.path}
                  placeholder="Pick a file"
                  onChange={(e) => input.setPath(e.target.value)}
                />
                <Button variant="default" icon={<FolderOpen size={12} />} onClick={input.browse}>
                  Browse
                </Button>
              </div>
            )}
          </Field>

          {input.chosen?.text ? (
            <Field
              label="Encoding"
              inline
              hint="How the file's bytes are read as text. A file read as the wrong one fails rather than importing mangled values."
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={input.options.encoding}
                  onChange={(e) => set({ encoding: e.target.value })}
                >
                  {input.encodings.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>
          ) : null}

          {input.sheets.length > 0 ? (
            <Field label="Worksheet" inline>
              {(id) => (
                <SelectInput
                  id={id}
                  value={input.options.sheet}
                  onChange={(e) => set({ sheet: e.target.value })}
                >
                  <option value="">{`First sheet (${input.sheets[0]})`}</option>
                  {input.sheets.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>
          ) : null}
        </div>
      ),
    },
  ]

  if (input.chosen?.delimited) {
    steps.push({
      prompt: 'What separates one field from the next?',
      body: <DelimiterStep input={input} set={set} />,
    })
  }

  steps.push(
    {
      prompt: 'Which rows of the file hold the data?',
      body: <RowsStep input={input} set={set} />,
    },
    {
      prompt: `Which field goes into which ${input.words.toLowerCase()} column?`,
      blocked: mappingProblem(input),
      body: <MappingStep input={input} />,
    },
    {
      prompt: 'What should these records do to the ones already there?',
      body: <ModeStep input={input} />,
    },
    {
      prompt: 'Everything is answered. Start when you are ready.',
      body: <RunStep input={input} />,
    },
  )

  return steps
}

function DelimiterStep({
  input,
  set,
}: {
  input: StepInput
  set: (patch: Partial<TransferOptions>) => void
}) {
  const fixed = input.options.widths.length > 0

  return (
    <div className="flex flex-col gap-3">
      <RadioList
        name="How fields are separated"
        value={fixed ? 'fixed' : 'delimited'}
        onChange={(value) => set({ widths: value === 'fixed' ? [10] : [] })}
        options={[
          {
            value: 'delimited',
            label: 'Delimited',
            hint: 'A character such as a comma or a tab separates each field.',
          },
          {
            value: 'fixed',
            label: 'Fixed width',
            hint: 'Every field is a fixed number of characters, padded with spaces.',
          },
        ]}
      />

      <div className="flex max-w-[620px] flex-col gap-3">
        {fixed ? (
          <Field
            label="Field widths"
            inline
            hint="How many characters each field takes, in order, separated by commas."
          >
            {(id) => (
              <TextInput
                id={id}
                value={input.options.widths.join(', ')}
                placeholder="10, 30, 8"
                onChange={(e) => set({ widths: widthsOf(e.target.value) })}
              />
            )}
          </Field>
        ) : (
          <>
            <Field label="Field delimiter" inline>
              {(id) => (
                <SelectInput
                  id={id}
                  value={input.options.delimiter}
                  onChange={(e) => set({ delimiter: e.target.value })}
                >
                  {FIELD_DELIMITERS.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <Field
              label="Text qualifier"
              inline
              hint="What wraps a field holding the delimiter, a quote of its own or a line break."
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={input.options.qualifier}
                  onChange={(e) => set({ qualifier: e.target.value })}
                >
                  {TEXT_QUALIFIERS.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>
          </>
        )}
      </div>

      <PreviewGrid preview={input.preview} />
    </div>
  )
}

function RowsStep({
  input,
  set,
}: {
  input: StepInput
  set: (patch: Partial<TransferOptions>) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex max-w-[620px] flex-col gap-3">
        <Checkbox
          label="The first row holds the field names"
          checked={input.options.headerRow}
          onChange={(e) => set({ headerRow: e.target.checked })}
        />

        <Field
          label="Rows to skip first"
          inline
          hint="Dropped before anything is read, header included — which is how a file with a banner over its table is imported."
        >
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={0}
              value={input.options.skip}
              onChange={(e) => set({ skip: Number(e.target.value) || 0 })}
            />
          )}
        </Field>

        <Field label="Rows to read at most" inline hint="Zero reads the file to its end.">
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={0}
              value={input.options.limit}
              onChange={(e) => set({ limit: Number(e.target.value) || 0 })}
            />
          )}
        </Field>

        <Checkbox
          label="Read an empty field as NULL rather than as the empty string"
          checked={input.options.emptyIsNull}
          onChange={(e) => set({ emptyIsNull: e.target.checked })}
        />
      </div>

      <PreviewGrid preview={input.preview} />
    </div>
  )
}

function MappingStep({ input }: { input: StepInput }) {
  const data = useMemo<GridData>(
    () => ({
      columns: [
        {
          name: 'Source Field',
          type: 'text',
          fullType: 'the file',
          editor: { kind: 'choice', options: ['', ...(input.preview?.columns ?? [])] },
        },
        { name: 'Target Column', type: 'text', fullType: 'the table' },
        { name: 'Key', type: 'boolean', fullType: 'identifies a stored row', editor: { kind: 'check' } },
      ],
      rows: input.mappings.map((mapping) => [
        mapping.source,
        mapping.target,
        mapping.key ? 'YES' : '',
      ]),
    }),
    [input.mappings, input.preview],
  )

  // Only two of the three columns are the reader's: the target is the table's own list of columns
  // and there is nothing to type into it.
  const edit = useCallback(
    (ref: CellRef, value: string | null) => {
      if (ref.column === 1) return
      input.setMappings(
        input.mappings.map((mapping, row) =>
          row !== ref.row
            ? mapping
            : ref.column === 0
              ? { ...mapping, source: value ?? '' }
              : { ...mapping, key: value !== null && value !== '' },
        ),
      )
    },
    [input],
  )

  return (
    <div className="flex flex-col gap-3">
      <div className="flex max-w-[620px] flex-col gap-3">
        <Field label="Import into" inline>
          {(id) => (
            <SelectInput id={id} value={input.table} onChange={(e) => input.setTable(e.target.value)}>
              <option value="">Pick a destination</option>
              {input.tables.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>
      </div>

      {input.mappings.length === 0 ? (
        <Empty title="Nothing to map yet" hint="Pick a destination above." />
      ) : (
        <div className="h-56 overflow-hidden rounded-md border border-line">
          <ResultGrid result={data} onEditCell={edit} />
        </div>
      )}

      <PreviewGrid preview={input.preview} />
    </div>
  )
}

function ModeStep({ input }: { input: StepInput }) {
  const keyed = input.mappings.some((mapping) => mapping.key && mapping.source !== '')

  return (
    <div className="flex flex-col gap-4">
      <RadioList
        name="Import mode"
        value={input.mode}
        onChange={input.setMode}
        options={IMPORT_MODES.map((one) => ({
          value: one.value,
          label: one.label,
          hint: one.needsKey && !keyed ? 'Needs a key column on the mapping step.' : undefined,
          disabled: one.needsKey && !keyed,
        }))}
      />

      <div className="flex max-w-[620px] flex-col gap-3 border-t border-line pt-3">
        <Field
          label="Records per statement"
          inline
          hint="How many rows one statement carries. More is fewer round trips; too many is a statement the server refuses as oversized."
        >
          {(id) => (
            <TextInput
              id={id}
              type="number"
              min={1}
              value={input.batch}
              onChange={(e) => input.setBatch(Math.max(Number(e.target.value) || 1, 1))}
            />
          )}
        </Field>

        <Checkbox
          label="Continue after a record the server rejects"
          checked={input.continueOnError}
          onChange={(e) => input.setContinueOnError(e.target.checked)}
        />
      </div>
    </div>
  )
}

function RunStep({ input }: { input: StepInput }) {
  const mode = importMode(input.mode)

  return (
    <div className="flex flex-col gap-3">
      <dl className="grid grid-cols-[9rem_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm">
        <Fact label="File" value={input.path} />
        <Fact label="Destination" value={input.table} />
        <Fact label="Mode" value={mode.label} />
        <Fact
          label="Fields"
          value={`${input.mappings.filter((one) => one.source !== '').length} mapped`}
        />
      </dl>

      <TransferLog
        messages={input.result?.messages ?? []}
        started={`[…]   ${input.running ? 'Importing…' : 'Ready to import.'}`}
        finished={input.result ? `[…]   ${finishedText(input.result)}` : ''}
      />
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <>
      <dt className="text-muted">{label}</dt>
      <dd className="selectable min-w-0 truncate text-fg">{value || '—'}</dd>
    </>
  )
}

/**
 * The head of the file as it will be read, which is what every step before the mapping is checked
 * by: a delimiter that is wrong shows here as one column holding the whole line, and a header row
 * that is not one shows as columns named after the first record.
 */
function PreviewGrid({ preview }: { preview: ImportPreview | null }) {
  const data = useMemo<GridData>(
    () => ({
      columns: (preview?.columns ?? []).map((name) => ({
        name,
        type: 'text',
        fullType: 'text',
      })),
      rows: preview?.rows ?? [],
    }),
    [preview],
  )

  if (!preview) return <Empty title="No preview yet" hint="Pick a file to see what it holds." />

  return (
    <div className="h-44 overflow-hidden rounded-md border border-line">
      <ResultGrid result={data} />
    </div>
  )
}

function mappingProblem(input: StepInput): string | undefined {
  if (input.table === '') return 'Pick a destination to go on.'
  if (!input.mappings.some((mapping) => mapping.source !== '')) {
    return 'No field is pointed at a column yet, so there would be nothing to write.'
  }
  return undefined
}

/** `10, 30, 8` as the three widths it names. Anything that is not a number is not a width. */
function widthsOf(text: string): number[] {
  return text
    .split(',')
    .map((piece) => Number(piece.trim()))
    .filter((width) => Number.isFinite(width) && width > 0)
}

function confirmText(mode: ImportMode, table: string, environment: string, rows: string): string {
  const where = environment === '' ? 'this connection' : `a ${environment} connection`

  if (mode === 'copy') {
    return `Every ${rows.replace(/s$/, '')} in ${table} on ${where} will be deleted before the file is written into it. This cannot be taken back.`
  }
  if (mode === 'delete') {
    return `This will delete ${rows} from ${table} on ${where}, one for every record the file matches.`
  }
  return `This will write to ${table} on ${where}.`
}

function summary(result: ImportResult | null, running: boolean): string {
  if (running) return 'Importing…'
  if (result === null) return ''

  const failed = result.failed === 0 ? 'none failed' : `${result.failed} failed`
  return `${result.read} read · ${result.written} written · ${failed} · ${result.elapsedMs} ms`
}

function finishedText(result: ImportResult): string {
  const time = `${result.elapsedMs} ms`
  if (result.failed === 0) {
    return `Finished · ${result.read} read · ${result.written} written · ${time}`
  }

  const errors = `${result.failed} ${result.failed === 1 ? 'error' : 'errors'}`
  return `Finished with ${errors} · ${result.read} read · ${result.written} written · ${time}`
}
