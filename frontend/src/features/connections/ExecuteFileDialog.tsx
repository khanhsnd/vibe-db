import { useEffect, useMemo, useState } from 'react'
import { FileCode2, FolderOpen, Loader2, Play, Square } from 'lucide-react'

import {
  engineInfo,
  engineWords,
  namespaceOf,
  type Namespace,
  type Profile,
} from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import {
  DEFAULT_ENCODING,
  queries,
  SCRIPT_ENCODINGS,
  type ScriptFileResult,
} from '@/bridge/query'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { Cell, GridData } from '@/features/results/resultSet'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { databaseKey, useConnectionsStore } from '@/stores/connections'

const NO_NAMES: string[] = []

/**
 * Runs a file of statements against one connection, and says what each of them did.
 *
 * It is the tree's Execute SQL File, and it is deliberately not the editor: a dump of a hundred
 * thousand statements is not something to open in a buffer, review and format — it is something to
 * point at a database and watch. So **no rows come back**, only what each statement did, and the
 * two questions a file raises that a buffer does not are asked here: which encoding its bytes are,
 * and whether a statement the server refuses ends the run.
 */
export function ExecuteFileDialog({
  profile,
  onOpenChange,
}: {
  profile: Profile | null
  onOpenChange: (open: boolean) => void
}) {
  const sessionId = useConnectionsStore((s) =>
    profile ? (s.sessions[profile.id]?.id ?? null) : null,
  )
  const databases = useConnectionsStore((s) =>
    profile ? (s.databases[profile.id] ?? NO_NAMES) : NO_NAMES,
  )
  const activeNamespace = useConnectionsStore((s) =>
    profile ? s.activeNamespace[profile.id] : undefined,
  )

  const [namespace, setNamespace] = useState<Namespace>(namespaceOf(''))
  const [path, setPath] = useState('')
  const [encoding, setEncoding] = useState(DEFAULT_ENCODING)
  const [continueOnError, setContinueOnError] = useState(true)
  const [runId, setRunId] = useState('')
  const [result, setResult] = useState<ScriptFileResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const schemas = useConnectionsStore(
    (s) => (profile ? s.schemas[databaseKey(profile.id, namespace.database)] : undefined) ?? NO_NAMES,
  )
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)

  const { hasSchemas } = engineInfo(profile?.engine ?? '')
  const running = runId !== ''

  // Opening the dialog starts where the reader already is — the namespace the tree has selected on
  // this connection — and forgets the last run, which was about another file.
  useEffect(() => {
    if (!profile) return
    setNamespace(activeNamespace ?? namespaceOf(profile.database))
    setResult(null)
    setError(null)
  }, [profile, activeNamespace])

  useEffect(() => {
    if (profile && hasSchemas && namespace.database !== '') {
      void loadSchemas(profile.id, namespace.database)
    }
  }, [profile, hasSchemas, namespace.database, loadSchemas])

  const browse = () => {
    void queries
      .chooseFile()
      .then((chosen) => chosen !== '' && setPath(chosen))
      .catch((thrown) => setError(toAppError(thrown)))
  }

  const start = async () => {
    if (sessionId === null || path.trim() === '' || running) return

    const id = crypto.randomUUID()
    setRunId(id)
    setError(null)
    setResult(null)

    try {
      setResult(
        await queries.runFile(sessionId, {
          runId: id,
          database: namespace.database,
          schema: namespace.schema,
          path,
          encoding,
          continueOnError,
        }),
      )
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setRunId('')
    }
  }

  return (
    <Dialog
      open={profile !== null}
      onOpenChange={onOpenChange}
      title={`Execute ${engineWords(profile?.engine ?? '').scriptFile}`}
      description={profile?.name ?? ''}
      icon={<FileCode2 size={15} />}
      width={620}
      footer={
        <>
          <span className="mr-auto text-xs text-muted">{summary(result, running)}</span>

          {running ? (
            <Button
              variant="danger"
              icon={<Square size={11} fill="currentColor" />}
              onClick={() => void queries.cancel(runId)}
            >
              Stop
            </Button>
          ) : null}

          <Button variant="subtle" onClick={() => onOpenChange(false)}>
            Close
          </Button>
          <Button
            variant="primary"
            icon={running ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} fill="currentColor" />}
            disabled={running || sessionId === null || path.trim() === ''}
            onClick={() => void start()}
          >
            Start
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label="Database" inline>
          {(id) => (
            <SelectInput
              id={id}
              value={namespace.database}
              onChange={(e) => setNamespace(namespaceOf(e.target.value))}
            >
              <option value="">No database</option>
              {databases.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>

        {hasSchemas ? (
          <Field label="Schema" inline>
            {(id) => (
              <SelectInput
                id={id}
                value={namespace.schema}
                onChange={(e) => setNamespace(namespaceOf(namespace.database, e.target.value))}
              >
                <option value="">No schema</option>
                {schemas.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </SelectInput>
            )}
          </Field>
        ) : null}

        <Field label="File" inline>
          {(id) => (
            <div className="flex items-center gap-2">
              <TextInput
                id={id}
                value={path}
                placeholder="Pick a file"
                onChange={(e) => setPath(e.target.value)}
              />
              <Button variant="default" icon={<FolderOpen size={12} />} onClick={browse}>
                Browse
              </Button>
            </div>
          )}
        </Field>

        <Field
          label="Encoding"
          inline
          hint="How the file's bytes are read as text. A file read as the wrong one fails rather than running mangled statements."
        >
          {(id) => (
            <SelectInput id={id} value={encoding} onChange={(e) => setEncoding(e.target.value)}>
              {SCRIPT_ENCODINGS.map((one) => (
                <option key={one.value} value={one.value}>
                  {one.label}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>

        <Checkbox
          label="Continue on error"
          checked={continueOnError}
          onChange={(e) => setContinueOnError(e.target.checked)}
        />

        {error ? <Banner tone="danger" text={error.message} /> : null}

        {result ? <MessageLog result={result} /> : null}
      </div>
    </Dialog>
  )
}

/** What the run came to, in the one line a footer has room for. */
function summary(result: ScriptFileResult | null, running: boolean): string {
  if (running) return 'Running…'
  if (result === null) return ''

  const count = result.statements.length
  const failed = result.failed === 0 ? 'none failed' : `${result.failed} failed`
  return `${count} ${count === 1 ? 'statement' : 'statements'} · ${failed} · ${result.elapsedMs} ms`
}

/**
 * What each statement did, in the order they ran — the file's own message log. It is a
 * `ResultGrid` because it is rows and columns, and that is the one component for those.
 *
 * There is no Rows column: a file run keeps no rows, so a number there would be a number about the
 * cap rather than about the statement — see `scriptFileRows` on the Go side. Nor is there a `#`
 * column: the grid numbers its own rows, and the statements are in the order they ran.
 */
function MessageLog({ result }: { result: ScriptFileResult }) {
  const rows = useMemo((): GridData => {
    const column = (name: string, type = 'text') => ({ name, type, fullType: type })

    return {
      columns: [
        column('Statement'),
        column('Affected', 'int'),
        column('Time (ms)', 'int'),
        column('Message'),
      ],
      rows: result.statements.map((statement): Cell[] => [
        statement.sql.replace(/\s+/g, ' ').trim(),
        String(statement.rowsAffected),
        String(statement.elapsedMs),
        statement.error === '' ? 'OK' : `${statement.errorCode}: ${statement.error}`,
      ]),
    }
  }, [result])

  return (
    <div className="h-64 overflow-hidden rounded-md border border-line">
      <ResultGrid result={rows} />
    </div>
  )
}
