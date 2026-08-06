import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download, FolderOpen } from 'lucide-react'

import { engineWords, namespaceLabel, type Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { queries } from '@/bridge/query'
import {
  defaultTransferOptions,
  FIELD_DELIMITERS,
  transfer,
  TEXT_QUALIFIERS,
  type ExportJobResult,
  type TransferEncoding,
  type TransferFormat,
  type TransferOptions,
} from '@/bridge/transfer'
import { Button } from '@/shared/ui/Button'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { Empty } from '@/shared/ui/Empty'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'

import { RadioList } from './RadioList'
import { TransferLog } from './TransferLog'
import { Wizard, type WizardStep } from './Wizard'

/** Where the wizard is pointed, and non-null exactly while it is up. */
export interface ExportTarget {
  profileId: string
  namespace: Namespace
  /** The one table to start with ticked, and empty for a wizard opened on the `Tables` folder. */
  table: string
}

const WIDTH = 820
const NO_TABLES: string[] = []

/**
 * Writes tables out to files, one question to a screen.
 *
 * **It is not `Dump`, and the two do not overlap.** A dump writes the statements that would make
 * these objects again — it is for putting a table back on a server. This writes the *rows*, in the
 * formats something other than a database reads: a spreadsheet, a JSON feed, a page to send someone.
 * That is why SQL is not in this list and no format here is in that menu.
 *
 * **One file per table**, named after the table, in the folder the reader picks. A wizard that also
 * asked for a file name would be asking a question with no answer for the forty-table case.
 */
export function ExportWizard({
  target,
  onOpenChange,
}: {
  target: ExportTarget | null
  onOpenChange: (open: boolean) => void
}) {
  const profile = useConnectionsStore((s) => s.profiles.find((p) => p.id === target?.profileId))
  const sessionId = useConnectionsStore((s) =>
    target ? (s.sessions[target.profileId]?.id ?? null) : null,
  )
  const tables = useConnectionsStore((s) =>
    target
      ? (s.objects[namespaceKey(target.profileId, target.namespace)]?.tables ?? NO_TABLES)
      : NO_TABLES,
  )

  const [formats, setFormats] = useState<TransferFormat[]>([])
  const [encodings, setEncodings] = useState<TransferEncoding[]>([])

  const [format, setFormat] = useState('csv')
  const [picked, setPicked] = useState<string[]>([])
  const [folder, setFolder] = useState('')
  const [options, setOptions] = useState<TransferOptions>(defaultTransferOptions)
  const [maxRows, setMaxRows] = useState(0)

  const [at, setAt] = useState(0)
  const [runId, setRunId] = useState('')
  const [result, setResult] = useState<ExportJobResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const words = engineWords(profile?.engine ?? '')
  const chosen = formats.find((one) => one.value === format)
  const running = runId !== ''

  useEffect(() => {
    void transfer.listFormats().then(setFormats).catch(() => {})
    void transfer.listEncodings().then(setEncodings).catch(() => {})
  }, [])

  // Opening starts from what the reader right-clicked: one table if they picked one, and every
  // table of the namespace if they picked the folder that holds them.
  useEffect(() => {
    if (!target) return
    setAt(0)
    setResult(null)
    setError(null)
    setPicked(target.table === '' ? tables : [target.table])
    // The table list is what the tree already read; it is deliberately not re-read here.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target])

  const browse = () => {
    void transfer
      .chooseFolder()
      .then((chosenFolder) => chosenFolder !== '' && setFolder(chosenFolder))
      .catch((thrown) => setError(toAppError(thrown)))
  }

  const start = useCallback(async () => {
    if (sessionId === null || running || target === null) return

    const id = crypto.randomUUID()
    setRunId(id)
    setError(null)
    setResult(null)

    try {
      setResult(
        await transfer.runExport(sessionId, {
          namespace: target.namespace,
          runId: id,
          format,
          tables: picked,
          folder,
          options,
          maxRows,
        }),
      )
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setRunId('')
    }
  }, [sessionId, running, target, format, picked, folder, options, maxRows])

  const steps = useMemo<WizardStep[]>(() => {
    const set = (patch: Partial<TransferOptions>) => setOptions({ ...options, ...patch })

    const built: WizardStep[] = [
      {
        prompt: 'Which format should the files be written in?',
        body: (
          <RadioList
            name="Export format"
            value={format}
            onChange={setFormat}
            options={formats
              .filter((one) => one.writes)
              .map((one) => ({ value: one.value, label: `${one.label} (*.${one.extension})` }))}
          />
        ),
      },
      {
        prompt: `Which ${words.tables.toLowerCase()} are going out?`,
        blocked: picked.length === 0 ? 'Tick at least one to go on.' : undefined,
        body: (
          <TablePicker
            tables={tables}
            picked={picked}
            onChange={setPicked}
            noun={words.tables.toLowerCase()}
          />
        ),
      },
      {
        prompt: 'Where should the files go?',
        blocked: folder === '' ? 'Pick a folder to go on.' : undefined,
        body: (
          <div className="flex max-w-[620px] flex-col gap-3">
            <Field
              label="Folder"
              inline
              hint={`One file per ${words.table.toLowerCase()}, named after it.`}
            >
              {(id) => (
                <div className="flex items-center gap-2">
                  <TextInput
                    id={id}
                    value={folder}
                    placeholder="Pick a folder"
                    onChange={(e) => setFolder(e.target.value)}
                  />
                  <Button variant="default" icon={<FolderOpen size={12} />} onClick={browse}>
                    Browse
                  </Button>
                </div>
              )}
            </Field>

            {chosen?.text ? (
              <Field label="Encoding" inline>
                {(id) => (
                  <SelectInput
                    id={id}
                    value={options.encoding}
                    onChange={(e) => set({ encoding: e.target.value })}
                  >
                    {encodings.map((one) => (
                      <option key={one.value} value={one.value}>
                        {one.label}
                      </option>
                    ))}
                  </SelectInput>
                )}
              </Field>
            ) : null}

            <Field
              label={`${words.rows[0]?.toUpperCase()}${words.rows.slice(1)} per ${words.table.toLowerCase()}`}
              inline
              hint="Zero writes every one of them, which is what an export usually means."
            >
              {(id) => (
                <TextInput
                  id={id}
                  type="number"
                  min={0}
                  value={maxRows}
                  onChange={(e) => setMaxRows(Math.max(Number(e.target.value) || 0, 0))}
                />
              )}
            </Field>
          </div>
        ),
      },
    ]

    built.push({
      prompt: 'How should the values be written?',
      body: (
        <div className="flex max-w-[620px] flex-col gap-3">
          <Checkbox
            label="Write the field names as the first row"
            checked={options.headerRow}
            onChange={(e) => set({ headerRow: e.target.checked })}
          />

          {chosen?.delimited ? (
            <>
              <Field label="Field delimiter" inline>
                {(id) => (
                  <SelectInput
                    id={id}
                    value={options.delimiter}
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
                hint="Wrapped around a value holding the delimiter, a quote of its own or a line break — and around nothing else, which is what keeps the file readable by eye."
              >
                {(id) => (
                  <SelectInput
                    id={id}
                    value={options.qualifier}
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
          ) : null}

          {nullIsWritten(chosen) ? (
            <Field
              label="Write NULL as"
              inline
              hint="This format has no null of its own, so a NULL and an empty value would otherwise read alike."
            >
              {(id) => (
                <TextInput
                  id={id}
                  value={options.null}
                  placeholder="(empty)"
                  onChange={(e) => set({ null: e.target.value })}
                />
              )}
            </Field>
          ) : null}
        </div>
      ),
    })

    built.push({
      prompt: 'Everything is answered. Start when you are ready.',
      body: (
        <div className="flex flex-col gap-3">
          <dl className="grid grid-cols-[9rem_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm">
            <dt className="text-muted">Format</dt>
            <dd className="text-fg">{chosen?.label ?? format}</dd>
            <dt className="text-muted">{words.tables}</dt>
            <dd className="text-fg">{`${picked.length} selected`}</dd>
            <dt className="text-muted">Folder</dt>
            <dd className="selectable min-w-0 truncate text-fg">{folder || '—'}</dd>
          </dl>

          <TransferLog
            messages={result?.messages ?? []}
            started={`[…]   ${running ? 'Exporting…' : 'Ready to export.'}`}
            finished={result ? `[…]   ${finishedText(result)}` : ''}
          />
        </div>
      ),
    })

    return built
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formats, encodings, chosen, format, tables, picked, folder, options, maxRows, result, running, words])

  return (
    <Wizard
      open={target !== null}
      onOpenChange={onOpenChange}
      title="Export Wizard"
      description={target ? `${profile?.name ?? ''} · ${namespaceLabel(target.namespace)}` : ''}
      icon={<Download size={15} />}
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

/**
 * Which tables are going out. A list of checkboxes rather than a grid: the only fact about each row
 * is its name, and `ResultGrid` is for rows that have columns.
 */
function TablePicker({
  tables,
  picked,
  onChange,
  noun,
}: {
  tables: string[]
  picked: string[]
  onChange: (picked: string[]) => void
  noun: string
}) {
  if (tables.length === 0) {
    return <Empty title={`No ${noun} here`} hint="There is nothing in this namespace to export." />
  }

  const toggle = (name: string) =>
    onChange(picked.includes(name) ? picked.filter((one) => one !== name) : [...picked, name])

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Button onClick={() => onChange(tables)}>Select all</Button>
        <Button onClick={() => onChange([])}>Select none</Button>
        <span className="ml-auto text-xs text-muted">{`${picked.length} of ${tables.length}`}</span>
      </div>

      <div className="scroll-thin h-64 overflow-auto rounded-md border border-line px-2 py-1.5">
        {tables.map((name) => (
          <Checkbox
            key={name}
            label={name}
            checked={picked.includes(name)}
            onChange={() => toggle(name)}
          />
        ))}
      </div>
    </div>
  )
}

/**
 * Whether this format needs to be told what a NULL looks like. JSON has `null`, XML says so in an
 * attribute and a spreadsheet's empty cell is not a cell holding no characters — the rest have
 * nothing to say it with, so the reader picks.
 */
function nullIsWritten(format: TransferFormat | undefined): boolean {
  return format !== undefined && ['csv', 'txt', 'html', 'md'].includes(format.value)
}

function summary(result: ExportJobResult | null, running: boolean): string {
  if (running) return 'Exporting…'
  if (result === null) return ''

  const failed = result.failed === 0 ? 'none failed' : `${result.failed} failed`
  return `${result.files.length} files · ${result.rows} rows · ${failed} · ${result.elapsedMs} ms`
}

function finishedText(result: ExportJobResult): string {
  const written = result.files.length - result.failed
  const time = `${result.elapsedMs} ms`

  if (result.failed === 0) return `Finished · ${written} files · ${result.rows} rows · ${time}`

  const errors = `${result.failed} ${result.failed === 1 ? 'error' : 'errors'}`
  return `Finished with ${errors} · ${written} files written · ${time}`
}
