import { useEffect, useState, type ReactNode } from 'react'
import { Archive, Loader2, Play } from 'lucide-react'

import { backups as api, defaultBackupRequest, type BackupResult } from '@/bridge/backup'
import { engineWords, namespaceLabel } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { TransferLog } from '@/features/transfer/TransferLog'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Checkbox, TextArea } from '@/shared/ui/Field'
import { Dialog } from '@/shared/ui/Dialog'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { TabStrip } from '@/shared/ui/TabStrip'
import { useBackupsStore, type BackupTarget } from '@/stores/backups'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'

import { ObjectPicker, useObjectPicker } from './ObjectPicker'

const TABS = ['General', 'Object Selection', 'Advanced', 'Message Log'] as const
type TabName = (typeof TABS)[number]

const NO_TABLES: string[] = []

/**
 * Taking one backup of one namespace.
 *
 * **Tabs and not a wizard.** The two transfer wizards ask one question a screen because their
 * answers depend on each other — what a delimiter is worth asking about depends on the format —
 * and none of these do: every tab here is already answered, and a reader who wants the default
 * presses Backup on the screen it opened on. Being able to reach the third answer without passing
 * the second is the whole difference, and it is why this is the shape Navicat uses for it too.
 *
 * The last tab is the only one that is not a question. It fills as the backup runs and is switched
 * to when Backup is pressed, so what the reader is looking at is what is happening.
 */
export function NewBackupDialog() {
  const target = useBackupsStore((s) => s.target)
  const close = useBackupsStore((s) => s.close)
  const refreshBackups = useConnectionsStore((s) => s.refreshBackups)

  return (
    <Dialog
      open={target !== null}
      onOpenChange={(open) => {
        if (!open) close()
      }}
      title="New Backup"
      icon={<Archive size={14} />}
      width={720}
      bodyClassName=""
      footer={null}
    >
      {/* Keyed on the namespace, so a dialog raised on a second one starts over rather than
          opening on the first one's tables and the first one's comment. */}
      {target ? (
        <NewBackupForm
          key={keyOf(target)}
          target={target}
          onClose={close}
          // The folder has a new file in it, so the tree's branch and the list tab — one list,
          // read once — are told rather than left showing what was there before.
          onDone={() => void refreshBackups(target.profileId, target.namespace)}
        />
      ) : null}
    </Dialog>
  )
}

const keyOf = (target: BackupTarget) => namespaceKey(target.profileId, target.namespace)

interface NewBackupFormProps {
  target: BackupTarget
  onClose: () => void
  onDone: () => void
}

function NewBackupForm({ target, onClose, onDone }: NewBackupFormProps) {
  const { profileId, namespace } = target

  const profile = useConnectionsStore((s) => s.profiles.find((one) => one.id === profileId) ?? null)
  const sessionId = useConnectionsStore((s) => s.sessions[profileId]?.id ?? null)
  const objects = useConnectionsStore((s) => s.objects[namespaceKey(profileId, namespace)] ?? null)
  const loadObjects = useConnectionsStore((s) => s.loadObjects)

  const [at, setAt] = useState<TabName>('General')
  const [comment, setComment] = useState('')
  const [withStructure, setWithStructure] = useState(true)
  const [withData, setWithData] = useState(true)

  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<BackupResult | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const tables = objects?.tables ?? NO_TABLES
  const words = engineWords(profile?.engine ?? '')
  const picked = useObjectPicker(tables)
  const chosen = picked.chosen

  // The list is the one thing here that has to be read from the server, and it is the same read the
  // tree's own folders are filled by — so a namespace already opened in the tree costs nothing.
  useEffect(() => {
    void loadObjects(profileId, namespace)
  }, [loadObjects, profileId, namespace])

  // What stops the button, said in one line rather than left for the server to refuse.
  const blocked =
    sessionId === null
      ? 'This connection is closed. Reconnect it to take a backup.'
      : !withStructure && !withData
        ? 'A backup has to hold the structure, the records, or both.'
        : chosen.length === 0
          ? `Nothing is selected. Pick at least one ${words.table.toLowerCase()}.`
          : ''

  const start = async () => {
    if (sessionId === null || blocked !== '') return

    setAt('Message Log')
    setRunning(true)
    setError(null)
    setResult(null)

    try {
      const ran = await api.run(sessionId, {
        ...defaultBackupRequest(namespace),
        // Every table is sent as `[]`, which the Go side reads as all of them — so a backup of a
        // whole namespace covers whatever it holds at the moment it runs.
        tables: picked.wire,
        withStructure,
        withData,
        comment,
      })
      setResult(ran)
      onDone()
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setRunning(false)
    }
  }

  const finished = result !== null

  return (
    <div className="flex flex-col">
      <TabStrip tabs={TABS} at={at} onMove={setAt} />

      {/* The panel is what scrolls, not the dialog: the footer holds Backup and has to stay
          reachable however long the table list under Object Selection runs. */}
      <div className="scroll-thin max-h-[60vh] min-h-[19rem] overflow-y-auto px-4 py-3.5">
        {at === 'General' ? (
          <General
            server={profile?.name ?? ''}
            namespace={namespaceLabel(namespace)}
            comment={comment}
            onComment={setComment}
          />
        ) : at === 'Object Selection' ? (
          <ObjectSelection
            tables={tables}
            excluded={picked.excluded}
            loading={objects === null}
            words={words.tables}
            onToggle={picked.toggle}
            onAll={picked.all}
            onNone={picked.none}
          />
        ) : at === 'Advanced' ? (
          <Advanced
            withStructure={withStructure}
            withData={withData}
            onStructure={setWithStructure}
            onData={setWithData}
          />
        ) : (
          <MessageLog result={result} running={running} error={error} />
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
        <span className="mr-auto text-xs text-muted">
          {blocked !== '' && at !== 'Message Log'
            ? blocked
            : finished
              ? `${result.name} · ${result.path}`
              : `${chosen.length} of ${tables.length} ${words.tables.toLowerCase()}`}
        </span>

        <Button onClick={onClose}>{finished && !running ? 'Close' : 'Cancel'}</Button>
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
          Backup
        </Button>
      </div>
    </div>
  )
}

function General({
  server,
  namespace,
  comment,
  onComment,
}: {
  server: string
  namespace: string
  comment: string
  onComment: (value: string) => void
}) {
  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Fact label="Server" value={server} />
        <Fact label="Database" value={namespace} />
      </div>

      <fieldset className="flex min-h-0 flex-1 flex-col gap-1.5 rounded-md border border-line p-3">
        <legend className="px-1 text-xs font-medium text-muted">Backup File Option</legend>
        <label className="text-xs font-medium text-muted" htmlFor="backup-comment">
          Comment
        </label>
        <TextArea
          id="backup-comment"
          value={comment}
          onChange={(event) => onComment(event.target.value)}
          className="min-h-[8rem] flex-1"
          placeholder="What this backup is for — kept in the file itself."
        />
      </fieldset>

      {/* The one thing about the file worth saying before it is written: what it does not hold. */}
      <p className="text-xs text-faint">
        Note: views, functions and the query history are not included. A backup holds the tables of
        this namespace and nothing else.
      </p>
    </div>
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
  onToggle,
  onAll,
  onNone,
}: {
  tables: string[]
  excluded: ReadonlySet<string>
  loading: boolean
  words: string
  onToggle: (table: string) => void
  onAll: () => void
  onNone: () => void
}): ReactNode {
  if (loading) return <Loading label="Reading the catalogue…" />
  if (tables.length === 0) {
    return <Empty title={`No ${words.toLowerCase()} here`} hint="There is nothing to back up." />
  }

  return (
    <ObjectPicker
      tables={tables}
      excluded={excluded}
      onToggle={onToggle}
      onAll={onAll}
      onNone={onNone}
    />
  )
}

function Advanced({
  withStructure,
  withData,
  onStructure,
  onData,
}: {
  withStructure: boolean
  withData: boolean
  onStructure: (value: boolean) => void
  onData: (value: boolean) => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <Checkbox
        label="Include structure — what would declare each table again"
        checked={withStructure}
        onChange={(event) => onStructure(event.target.checked)}
      />
      <Checkbox
        label="Include records — the rows each table holds"
        checked={withData}
        onChange={(event) => onData(event.target.checked)}
      />

      <p className="text-xs text-faint">
        A backup is a file of the statements that would put this namespace back, so both together
        is a whole one. Leaving the records out keeps a shape worth restoring onto an empty server;
        leaving the structure out keeps rows for a server that already has the tables.
      </p>
    </div>
  )
}

function MessageLog({
  result,
  running,
  error,
}: {
  result: BackupResult | null
  running: boolean
  error: AppError | null
}) {
  return (
    <div className="flex flex-col gap-3">
      {error ? <Banner tone="danger" text={error.message} /> : null}

      <TransferLog
        messages={result?.messages ?? []}
        started={running ? 'Backing up…' : result ? '' : 'Nothing has run yet.'}
        finished={
          result === null || running
            ? ''
            : result.failed > 0
              ? 'Failed'
              : `Finished in ${result.elapsedMs} ms`
        }
      />
    </div>
  )
}
