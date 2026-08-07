import { useCallback, useEffect, useState } from 'react'
import { HardDriveDownload, Loader2, Play } from 'lucide-react'

import { engineWords, namespaceLabel, type EngineWords } from '@/bridge/connections'
import {
  defaultDumpRequest,
  defaultRestoreRequest,
  onToolLine,
  tools as api,
  type Toolkit,
  type ToolProgram,
  type ToolResult,
  type ToolWhich,
} from '@/bridge/dbtool'
import { toAppError, type AppError } from '@/bridge/errors'
import type { TransferMessage } from '@/bridge/transfer'
import { ObjectPicker, useObjectPicker } from '@/features/backup/ObjectPicker'
import { TransferLog } from '@/features/transfer/TransferLog'
import { cn } from '@/lib/cn'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Checkbox, TextInput } from '@/shared/ui/Field'
import { Dialog } from '@/shared/ui/Dialog'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { TabStrip } from '@/shared/ui/TabStrip'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'
import { useToolsStore, type ToolTarget } from '@/stores/dbtool'

const TABS = ['General', 'Object Selection', 'Advanced', 'Message Log'] as const
type TabName = (typeof TABS)[number]

const NO_TABLES: string[] = []

/**
 * Running the engine's **own** dump or restore program.
 *
 * One dialog for both directions, because they ask the same four questions — which server, which
 * file, which tables, and what to do about what is already there — and a second component would be
 * those four boxes with a different verb on the button.
 *
 * **Tabs and not a wizard**, the shape New Backup already uses and for the same reason: every tab
 * here is answered before it is opened. What differs from that dialog is the first tab, which has
 * something no backup ever does — a *program*, which may not be on this machine, and a file the
 * reader names rather than a shelf the app owns.
 */
export function ToolDialog() {
  const target = useToolsStore((s) => s.target)
  const close = useToolsStore((s) => s.close)

  return (
    <Dialog
      open={target !== null}
      onOpenChange={(open) => {
        if (!open) close()
      }}
      title={target?.mode === 'restore' ? 'Restore With Engine Tool' : 'Dump With Engine Tool'}
      icon={<HardDriveDownload size={14} />}
      width={720}
      bodyClassName=""
      footer={null}
    >
      {/* Keyed on the namespace *and* the direction, so a dialog raised on a second one starts
          over rather than opening on the first one's file and the first one's tables. */}
      {target ? <ToolForm key={keyOf(target)} target={target} onClose={close} /> : null}
    </Dialog>
  )
}

const keyOf = (target: ToolTarget) =>
  `${target.mode}:${namespaceKey(target.profileId, target.namespace)}`

function ToolForm({ target, onClose }: { target: ToolTarget; onClose: () => void }) {
  const { profileId, namespace, mode } = target
  const dumping = mode === 'dump'

  const profile = useConnectionsStore((s) => s.profiles.find((one) => one.id === profileId) ?? null)
  const sessionId = useConnectionsStore((s) => s.sessions[profileId]?.id ?? null)
  const objects = useConnectionsStore((s) => s.objects[namespaceKey(profileId, namespace)] ?? null)
  const loadObjects = useConnectionsStore((s) => s.loadObjects)

  const [at, setAt] = useState<TabName>('General')
  const [kit, setKit] = useState<Toolkit | null>(null)
  const [path, setPath] = useState('')

  const [withStructure, setWithStructure] = useState(true)
  const [withData, setWithData] = useState(true)
  const [compress, setCompress] = useState(true)
  const [drop, setDrop] = useState(false)
  const [dryRun, setDryRun] = useState(false)
  const [stopOnError, setStopOnError] = useState(true)

  const [runId, setRunId] = useState('')
  const [lines, setLines] = useState<TransferMessage[]>([])
  const [result, setResult] = useState<ToolResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const tables = objects?.tables ?? NO_TABLES
  const words = engineWords(profile?.engine ?? '')
  const picked = useObjectPicker(tables)
  const running = runId !== ''

  const readKit = useCallback(async () => {
    if (sessionId === null) return
    try {
      setKit(await api.toolkit(sessionId))
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }, [sessionId])

  // Both reads happen when the dialog opens: what this machine has, and what the namespace holds.
  // The kit is read *every* time rather than remembered — a reader who installs the tools while
  // this is open presses Locate and carries on rather than restarting the app.
  useEffect(() => {
    void readKit()
  }, [readKit])

  useEffect(() => {
    void loadObjects(profileId, namespace)
  }, [loadObjects, profileId, namespace])

  const program = dumping ? kit?.dump : kit?.restore

  // What stops the button, said in one line rather than left for the program to refuse.
  const blocked =
    sessionId === null
      ? 'This connection is closed. Reconnect it to run this.'
      : program === undefined
        ? 'Looking for the program…'
        : !program.found
          ? `${program.name} is not on this machine.`
          : path === ''
            ? 'No file chosen yet.'
            : dumping && kit?.splitsStructure && !withStructure && !withData
              ? 'A dump has to hold the structure, the records, or both.'
              : dumping && picked.chosen.length === 0
                ? `Nothing is selected. Pick at least one ${words.table.toLowerCase()}.`
                : ''

  const choose = async () => {
    if (sessionId === null) return
    try {
      const chosen = await api.chooseFile(sessionId, dumping, suggestedName(namespace))
      if (chosen !== '') setPath(chosen)
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }

  // Pointing at a program, and forgetting where one is. Both answer with the whole kit, so the
  // panel redraws from one call — and both stay available however the pair currently stands: a
  // path that was right last month is exactly the thing that needs changing today.
  const locate = async (which: ToolWhich, forget = false) => {
    if (sessionId === null) return
    try {
      setKit(await (forget ? api.forgetProgram : api.chooseProgram)(sessionId, which))
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }

  const start = async () => {
    if (sessionId === null || blocked !== '') return

    const id = crypto.randomUUID()
    setAt('Message Log')
    setRunId(id)
    setError(null)
    setResult(null)
    setLines([])

    // The program's lines are pushed as it prints them, so a dump that takes an hour is watchable.
    // The call answers with the whole log as well, which is what replaces these when it lands.
    const off = onToolLine(id, (text) => setLines((held) => [...held, { text, error: false }]))

    try {
      const ran = dumping
        ? await api.dump(sessionId, {
            ...defaultDumpRequest(namespace, id),
            path,
            tables: picked.wire,
            withStructure,
            withData,
            compress,
          })
        : await api.restore(sessionId, {
            ...defaultRestoreRequest(namespace, id),
            path,
            tables: picked.wire,
            drop,
            dryRun,
            stopOnError,
          })
      setResult(ran)
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      off()
      setRunId('')
    }
  }

  const finished = result !== null

  return (
    <div className="flex flex-col">
      <TabStrip tabs={TABS} at={at} onMove={setAt} />

      <div className="scroll-thin max-h-[60vh] min-h-[19rem] overflow-y-auto px-4 py-3.5">
        {at === 'General' ? (
          <General
            server={profile?.name ?? ''}
            namespace={namespaceLabel(namespace)}
            kit={kit}
            dumping={dumping}
            path={path}
            onPath={setPath}
            onBrowse={() => void choose()}
            onLocate={(which, forget) => void locate(which, forget)}
          />
        ) : at === 'Object Selection' ? (
          <ObjectSelection
            tables={tables}
            excluded={picked.excluded}
            loading={objects === null}
            words={words}
            dumping={dumping}
            onToggle={picked.toggle}
            onAll={picked.all}
            onNone={picked.none}
          />
        ) : at === 'Advanced' ? (
          <Advanced
            kit={kit}
            dumping={dumping}
            values={{ withStructure, withData, compress, drop, dryRun, stopOnError }}
            on={{
              withStructure: setWithStructure,
              withData: setWithData,
              compress: setCompress,
              drop: setDrop,
              dryRun: setDryRun,
              stopOnError: setStopOnError,
            }}
          />
        ) : (
          <MessageLog
            program={program?.name ?? ''}
            messages={result?.messages ?? lines}
            result={result}
            running={running}
            error={error}
          />
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
        <span className="mr-auto truncate text-xs text-muted">
          {blocked !== '' && at !== 'Message Log'
            ? blocked
            : finished
              ? result.path
              : path || `${picked.chosen.length} of ${tables.length} ${words.tables.toLowerCase()}`}
        </span>

        {running ? (
          <Button onClick={() => void api.stop(runId)}>Stop</Button>
        ) : (
          <Button onClick={onClose}>{finished ? 'Close' : 'Cancel'}</Button>
        )}
        <Button
          variant="primary"
          icon={
            running ? (
              <Loader2 size={12} className="animate-spin" />
            ) : (
              <Play size={12} fill="currentColor" />
            )
          }
          disabled={running || blocked !== ''}
          onClick={() => void start()}
        >
          {program?.name ?? (dumping ? 'Dump' : 'Restore')}
        </Button>
      </div>
    </div>
  )
}

/** What the save dialog opens on: the namespace and the moment, which is what a backup is called. */
function suggestedName(namespace: { database: string; schema: string }): string {
  const stamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '')
  return `${namespace.schema || namespace.database || 'dump'}-${stamp}`
}

function General({
  server,
  namespace,
  kit,
  dumping,
  path,
  onPath,
  onBrowse,
  onLocate,
}: {
  server: string
  namespace: string
  kit: Toolkit | null
  dumping: boolean
  path: string
  onPath: (value: string) => void
  onBrowse: () => void
  onLocate: (which: ToolWhich, forget: boolean) => void
}) {
  if (kit === null) return <Loading label="Looking for the program…" />

  const program = dumping ? kit.dump : kit.restore

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Fact label="Server" value={server} />
        <Fact label="Database" value={namespace} />
      </div>

      {/* **The one program this direction runs, and no other.** A dump is not the place to point
          at mongorestore: the other program is pointed at where the other program is asked for,
          and nothing here fills one in on the reader's behalf. */}
      <ProgramBox
        program={program}
        onLocate={(forget) => onLocate(dumping ? 'dump' : 'restore', forget)}
      />

      {program.found ? null : (
        <Banner
          tone="warn"
          text={
            program.chosen === ''
              ? `${program.name} is not on the PATH. Install this engine's command-line tools, or ` +
                `press Change and point at the file.`
              : `${program.name} is no longer at the file this app was pointed at. Point at it ` +
                `again, or Forget it to fall back to the PATH.`
          }
        />
      )}

      <fieldset className="flex min-w-0 flex-col gap-1.5 rounded-md border border-line p-3">
        <legend className="px-1 text-xs font-medium text-muted">
          {dumping ? 'File to write' : 'File to put back'}
        </legend>

        <div className="flex min-w-0 items-center gap-2">
          <TextInput
            value={path}
            onChange={(event) => onPath(event.target.value)}
            placeholder={`A ${kit.format.toLowerCase()} · .${kit.extension}`}
            className="flex-1"
          />
          <Button onClick={onBrowse}>Browse…</Button>
        </div>
      </fieldset>

      <p className="text-xs text-faint">
        {dumping
          ? `${kit.dump.name} writes this file itself — nothing about it passes through this app, ` +
            `and nothing here reads it. It is the engine's own format, not a script, so it goes ` +
            `back with ${kit.restore.name} and with nothing else.`
          : `${kit.restore.name} reads this file itself. What it puts back is whatever ` +
            `${kit.dump.name} wrote, at the namespace the file names — which is not always the one ` +
            `open in the tree.`}
      </p>
    </div>
  )
}

/**
 * The program this direction runs: what it is called, where it turned out to be, and how to point
 * somewhere else. **`Change…` is always there**, because a path that was right last month is
 * exactly the thing that needs changing today.
 *
 * The name and the buttons share the first line and the path has the second to itself. A tool path
 * is a hundred characters of nested directory, and on one line with the buttons it either squeezes
 * them off the edge or truncates to nothing useful — this way the buttons sit at a fixed right edge
 * whatever the path is, and the path gets the whole width before it is cut.
 *
 * `min-w-0` on the fieldset is not decoration: a fieldset's `min-inline-size` is `min-content` by
 * default, so without it the long path widens the box past the dialog instead of truncating.
 *
 * Three states and each reads differently. Found on the PATH is the path with nothing to undo;
 * found because the reader picked it carries `Forget`, which is the way back; and **picked but no
 * longer there** is neither — it names the file that has gone rather than quietly running a
 * different copy off the PATH.
 */
function ProgramBox({
  program,
  onLocate,
}: {
  program: ToolProgram
  onLocate: (forget: boolean) => void
}) {
  const where = program.found ? program.path : program.chosen || 'not on the PATH'

  return (
    <fieldset className="flex min-w-0 flex-col gap-1.5 rounded-md border border-line p-3">
      <legend className="px-1 text-xs font-medium text-muted">Program</legend>

      <div className="flex min-w-0 items-center gap-2">
        <span className="truncate text-sm text-fg">{program.name}</span>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          {program.chosen === '' ? null : (
            <Button variant="subtle" onClick={() => onLocate(true)}>
              Forget
            </Button>
          )}
          <Button onClick={() => onLocate(false)}>Change…</Button>
        </div>
      </div>

      <span
        className={cn(
          'selectable min-w-0 truncate text-xs',
          program.found ? 'text-accent' : 'text-danger',
        )}
        title={where}
      >
        {program.found ? where : `${where} — not there`}
      </span>
    </fieldset>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[7rem_minmax(0,1fr)] gap-2 text-sm">
      <span className="text-muted">{label}:</span>
      <span className="selectable truncate text-accent">{value}</span>
    </div>
  )
}

function ObjectSelection({
  tables,
  excluded,
  loading,
  words,
  dumping,
  onToggle,
  onAll,
  onNone,
}: {
  tables: string[]
  excluded: ReadonlySet<string>
  loading: boolean
  words: EngineWords
  dumping: boolean
  onToggle: (table: string) => void
  onAll: () => void
  onNone: () => void
}) {
  if (loading) return <Loading label="Reading the catalogue…" />
  if (tables.length === 0) {
    return (
      <Empty
        title={`No ${words.tables.toLowerCase()} here`}
        hint={dumping ? 'There is nothing to dump.' : 'Everything in the file comes back.'}
      />
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <ObjectPicker
        tables={tables}
        excluded={excluded}
        onToggle={onToggle}
        onAll={onAll}
        onNone={onNone}
      />

      {/* The honest caveat about the restore side: this list is what the *server* holds now, and
          the file may hold a name that is not on it. All ticked is the only setting that cannot
          leave something behind, which is why it is the one it opens on. */}
      <p className="text-xs text-faint">
        {dumping
          ? 'Leaving them all ticked covers whatever the namespace holds at the moment it runs.'
          : `These are the ${words.tables.toLowerCase()} the server has now. Leave them all ticked ` +
            `to put back everything the file holds — unticking narrows the restore to the names ` +
            `left ticked, and a name only the file has cannot be picked here.`}
      </p>
    </div>
  )
}

interface AdvancedValues {
  withStructure: boolean
  withData: boolean
  compress: boolean
  drop: boolean
  dryRun: boolean
  stopOnError: boolean
}

/**
 * The options this pair of programs actually has, and nothing else.
 *
 * **A box that is not here is a thing the program has not got** — the mysql client cannot drop what
 * is there before it runs a script, and a MongoDB collection has no structure to leave out of a
 * dump. Drawing either as a box that does nothing would be a promise this app cannot keep, so the
 * flags on `domain.Toolkit` decide what appears.
 */
function Advanced({
  kit,
  dumping,
  values,
  on,
}: {
  kit: Toolkit | null
  dumping: boolean
  values: AdvancedValues
  on: { [K in keyof AdvancedValues]: (value: boolean) => void }
}) {
  if (kit === null) return <Loading label="Looking for the program…" />

  if (dumping) {
    return (
      <div className="flex flex-col gap-3">
        {kit.splitsStructure ? (
          <>
            <Checkbox
              label="Include structure — what would declare each table again"
              checked={values.withStructure}
              onChange={(event) => on.withStructure(event.target.checked)}
            />
            <Checkbox
              label="Include records — the rows each table holds"
              checked={values.withData}
              onChange={(event) => on.withData(event.target.checked)}
            />
          </>
        ) : (
          <p className="text-xs text-faint">
            A collection declares nothing, so there is no structure to leave out: what
            {` ${kit.dump.name} `}
            writes is the documents, the indexes and the collection options together.
          </p>
        )}

        {kit.compresses ? (
          <Checkbox
            label="Compress as it is written"
            checked={values.compress}
            onChange={(event) => on.compress(event.target.checked)}
          />
        ) : (
          <p className="text-xs text-faint">
            {kit.format} is compressed already, so there is nothing to turn on.
          </p>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      {kit.drops ? (
        <Checkbox
          label="Drop what is there first"
          checked={values.drop}
          onChange={(event) => on.drop(event.target.checked)}
        />
      ) : null}
      {kit.dryRuns ? (
        <Checkbox
          label="Dry run — say what would happen and change nothing"
          checked={values.dryRun}
          onChange={(event) => on.dryRun(event.target.checked)}
        />
      ) : null}
      <Checkbox
        label="Stop at the first thing the server refuses"
        checked={values.stopOnError}
        onChange={(event) => on.stopOnError(event.target.checked)}
      />

      <Banner
        tone="warn"
        text={
          values.drop
            ? 'Dropping is not undone by anything. What is there now goes, and what the file holds ' +
              'replaces it.'
            : 'A restore adds to what is there rather than replacing it, and nothing here is ' +
              'wrapped in a transaction: what has already been written stays written.'
        }
      />
    </div>
  )
}

function MessageLog({
  program,
  messages,
  result,
  running,
  error,
}: {
  program: string
  messages: TransferMessage[]
  result: ToolResult | null
  running: boolean
  error: AppError | null
}) {
  return (
    <div className="flex flex-col gap-3">
      {error ? <Banner tone="danger" text={error.message} /> : null}

      <TransferLog
        messages={messages}
        started={running ? `Running ${program}…` : result ? '' : 'Nothing has run yet.'}
        finished={
          result === null || running
            ? ''
            : result.failed
              ? `${program} failed`
              : `Finished in ${result.elapsedMs} ms`
        }
      />
    </div>
  )
}
