import { useEffect, useState } from 'react'
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
  type StatementOutcome,
} from '@/bridge/query'
import { cn } from '@/lib/cn'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { databaseKey, useConnectionsStore } from '@/stores/connections'

const NO_NAMES: string[] = []

/**
 * Two widths, and the dialog is whichever it currently has something to fill.
 *
 * The form is five fields and wants a form's width: a Database picker stretched to 900 px is harder
 * to read, not easier. The log wants far more — around 140 characters to a line, so a `CREATE TABLE`
 * reads as its author laid it out and only the longest single-line `ALTER TABLE` folds.
 *
 * **So it opens narrow and widens when the log arrives.** Holding the log's width from the start
 * would leave most of the dialog standing empty for the whole time a reader spends filling it in,
 * which is a cost paid every time; the widening is paid once, seconds or minutes after `Start` was
 * pressed, when the reader is looking at the log that has just appeared rather than at the button.
 */
const FORM_WIDTH = 660
const LOG_WIDTH = 1120

/**
 * Runs a file of statements against one connection, and says what each of them did.
 *
 * It is the tree's Execute SQL File, and it is deliberately not the editor: a dump of a hundred
 * thousand statements is not something to open in a buffer, review and format — it is something to
 * point at a database and watch. So **no rows come back**, only what each statement did, and the
 * two questions a file raises that a buffer does not are asked here: which encoding its bytes are,
 * and whether a statement the server refuses ends the run.
 *
 * What comes back is a **transcript** rather than a table — see `ScriptLog`.
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
      width={result ? LOG_WIDTH : FORM_WIDTH}
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
        {/*
          The form keeps its own width once the log has widened the dialog past it: a picker is read
          from its left edge, so a Database box stretched to 900 px is harder to read, not easier. At
          FORM_WIDTH the cap does nothing, which is the point — the dialog is that width *because*
          this is.
        */}
        <div className="flex max-w-[620px] flex-col gap-3">
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
        </div>

        {error ? <Banner tone="danger" text={error.message} /> : null}

        {result ? <ScriptLog result={result} fileName={baseName(path)} /> : null}
      </div>
    </Dialog>
  )
}

/** The file's own name, which is what the log's first line says rather than the whole path. */
function baseName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path
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
 * What each statement did, in the order they ran — the file's own log.
 *
 * **It is a transcript and not a grid**, and for the reason the [console's](../console/ConsoleLog.tsx)
 * is one: what a run produces is not rows and columns. A statement is a statement — laid out over as
 * many lines as its author wrote it over — and an error is a paragraph the server wrote. Put in cells
 * those are the two widest columns of four, both clipped to a couple of hundred pixels, and the one
 * thing a reader opened this to read is the one thing they cannot see. `ResultGrid` is still the only
 * thing in this app that draws rows and columns; this simply is not rows and columns.
 *
 * A statement keeps **its own line breaks**, so a `CREATE TABLE` reads as it was written. Every line
 * hangs from a six-character tag — measured in `ch` so it holds at whatever size the reader set the
 * data font to — which is what keeps a wrapped line, and a statement's own second line, under the
 * statement rather than under the tag.
 *
 * There is nothing about rows: a file run keeps none, so a count would be a fact about the cap rather
 * than about the statement — see `scriptFileRows` on the Go side.
 */
function ScriptLog({ result, fileName }: { result: ScriptFileResult; fileName: string }) {
  return (
    // Selectable and copied as it reads: the tag is part of each line's own text rather than a span
    // beside it, so a log lifted out of here into a message arrives laid out the way it was shown.
    // Tall enough that the form above it and the log together clear an ordinary window without the
    // dialog's own body having to scroll as well: the log is then the one scroller on this surface,
    // and two nested ones is exactly what made the old table unreachable.
    <div className="scroll-thin selectable data-font h-80 overflow-auto rounded-md border border-line bg-canvas px-3 py-2">
      <Line tone="accent">{`[SQL] Started ${fileName}`}</Line>

      {/*
        A statement and what it did are one group with a little air around it, so a log of a thousand
        of them is scannable. The air is padding and not a blank line, because a blank line would be
        in the text a reader copies out and doubles the length of what they paste.
      */}
      {result.statements.map((statement, at) => (
        <div key={at} className="py-0.5">
          <Line tone="fg">{`[SQL] ${statement.sql.trim()}`}</Line>
          {statement.error === '' ? (
            <Line tone="muted">{`[OK]  ${outcomeText(statement)}`}</Line>
          ) : (
            // The message alone, without the wire code beside it. The code is what the *app*
            // switches on — see docs/errors.md — and next to the wording it is only ever noise or,
            // where the server's own answer already opens `ERROR:`, noise that reads as a stutter.
            <Line tone="danger">{`[ERR] ${statement.error}`}</Line>
          )}
        </div>
      ))}

      <Line tone="accent">{`[SQL] ${finishedText(result)}`}</Line>
    </div>
  )
}

const TONES = {
  fg: 'text-fg',
  muted: 'text-muted',
  danger: 'text-danger',
  accent: 'text-accent',
} as const

/**
 * One line of the log, hanging from its tag: `pl-[6ch]` puts the body under the tag and the negative
 * indent of the same width pulls the *first* line back out to where the tag belongs. `whitespace-pre`
 * wrapping is what keeps a statement's own newlines and still folds a line too long for the width.
 */
function Line({ tone, children }: { tone: keyof typeof TONES; children: string }) {
  return (
    <div className={cn('-indent-[6ch] pl-[6ch] whitespace-pre-wrap', TONES[tone])}>{children}</div>
  )
}

/**
 * What a statement the server accepted did. A statement that changed no rows reports its time alone:
 * the tag already said it succeeded, and `OK · 1 ms` says that twice. Never a row *count* — see
 * `ScriptLog`.
 */
function outcomeText(statement: StatementOutcome): string {
  if (statement.rowsAffected === 0) return `${statement.elapsedMs} ms`

  const rows = `${statement.rowsAffected} ${statement.rowsAffected === 1 ? 'row' : 'rows'} affected`
  return `${rows} · ${statement.elapsedMs} ms`
}

/**
 * How the run ended, which is the line a reader scrolls to the bottom for. A run stopped by
 * `Continue on error` being off says so, because the statements after the failure did not run and a
 * log that only said "1 error" would leave the reader counting lines to find that out.
 */
function finishedText(result: ScriptFileResult): string {
  const time = `${result.elapsedMs} ms`
  if (result.failed === 0) return `Finished · ${result.statements.length} statements · ${time}`

  const errors = `${result.failed} ${result.failed === 1 ? 'error' : 'errors'}`
  return `Finished with ${errors} · ${result.statements.length} statements run · ${time}`
}
