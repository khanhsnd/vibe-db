import { useEffect, useMemo, useState } from 'react'
import { HardDriveDownload, Loader2, Play } from 'lucide-react'

import { objectDump, type DumpSelection } from '@/bridge/export'
import { engineWords, namespaceLabel, type Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Checkbox, SelectInput } from '@/shared/ui/Field'
import { Dialog } from '@/shared/ui/Dialog'
import { Empty } from '@/shared/ui/Empty'
import { Loading } from '@/shared/ui/Loading'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'

export interface DumpTarget {
  profileId: string
  namespace: Namespace
}

export function DumpDialog({
  target,
  withData,
  onClose,
}: {
  target: DumpTarget[] | null
  withData: boolean
  onClose: () => void
}) {
  return (
    <Dialog
      open={target !== null}
      onOpenChange={(open) => { if (!open) onClose() }}
      title="Dump SQL"
      icon={<HardDriveDownload size={14} />}
      width={720}
      bodyClassName=""
      footer={null}
    >
      {target ? <DumpForm key={target.map((one) => namespaceKey(one.profileId, one.namespace)).join('|')} target={target} withData={withData} onClose={onClose} /> : null}
    </Dialog>
  )
}

function DumpForm({ target, withData, onClose }: { target: DumpTarget[]; withData: boolean; onClose: () => void }) {
  const profile = useConnectionsStore((s) => s.profiles.find((one) => one.id === target[0]?.profileId) ?? null)
  const sessionId = useConnectionsStore((s) => s.sessions[target[0]?.profileId ?? '']?.id ?? null)
  const objects = useConnectionsStore((s) => s.objects)
  const loadObjects = useConnectionsStore((s) => s.loadObjects)
  const [excluded, setExcluded] = useState<Record<string, Set<string>>>({})
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState('')
  const [error, setError] = useState<AppError | null>(null)
  const [activeKey, setActiveKey] = useState(() =>
    target[0] ? namespaceKey(target[0].profileId, target[0].namespace) : '',
  )

  useEffect(() => {
    for (const one of target) void loadObjects(one.profileId, one.namespace)
  }, [target, loadObjects])

  const entries = useMemo(() => target.map((one) => {
    const key = namespaceKey(one.profileId, one.namespace)
    const tables = objects[key]?.tables ?? null
    const hidden = excluded[key] ?? new Set<string>()
    return { ...one, key, tables, hidden, chosen: tables?.filter((table) => !hidden.has(table)) ?? [] }
  }), [target, objects, excluded])
  const selectedCount = entries.reduce((count, one) => count + one.chosen.length, 0)
  const loading = entries.some((one) => one.tables === null)
  const active = entries.find((one) => one.key === activeKey) ?? entries[0]
  const words = engineWords(profile?.engine ?? '')
  const blocked = sessionId === null ? 'This connection is closed.' : loading ? 'Reading the catalogue…' : selectedCount === 0 ? `Pick at least one ${words.table.toLowerCase()}.` : ''

  const toggle = (key: string, table: string) => setExcluded((current) => {
    const next = new Set(current[key] ?? [])
    if (next.has(table)) next.delete(table); else next.add(table)
    return { ...current, [key]: next }
  })
  const selectAll = (key: string) => setExcluded((current) => ({ ...current, [key]: new Set() }))
  const selectNone = (key: string, tables: string[]) => setExcluded((current) => ({ ...current, [key]: new Set(tables) }))

  const start = async () => {
    if (sessionId === null || blocked !== '') return
    setRunning(true); setError(null); setResult('')
    try {
      const selections: DumpSelection[] = entries.map((one) => ({
        namespace: one.namespace,
        tables: one.chosen,
        all: one.hidden.size === 0,
      }))
      const first = target[0]
      if (!first) return
      setResult(await objectDump.run(sessionId, { namespace: first.namespace, table: '', withData, selections }))
    } catch (thrown) { setError(toAppError(thrown)) } finally { setRunning(false) }
  }

  return <div className="flex flex-col">
    <div className="scroll-thin max-h-[60vh] min-h-[19rem] overflow-y-auto px-4 py-3.5">
      <div className="mb-3 flex flex-col gap-3 text-sm">
        <div><span className="text-muted">Server:</span> <span className="text-accent">{profile?.name ?? ''}</span></div>
        <label className="flex items-center gap-3">
          <span className="shrink-0 text-muted">Database:</span>
          <SelectInput
            value={active?.key ?? ''}
            onChange={(event) => setActiveKey(event.target.value)}
            className="min-w-0 flex-1"
          >
            {entries.map((one) => (
              <option key={one.key} value={one.key}>
                {namespaceLabel(one.namespace)}
              </option>
            ))}
          </SelectInput>
        </label>
      </div>
      {error ? <Banner tone="danger" text={error.message} /> : null}
      {active ? <fieldset className="rounded-md border border-line p-3">
        <legend className="px-1 text-xs font-medium text-muted">Tables in {namespaceLabel(active.namespace)}</legend>
        {active.tables === null ? <Loading label="Reading the catalogue…" /> : active.tables.length === 0 ? <Empty title={`No ${words.tables.toLowerCase()} here`} hint="There is nothing to dump." /> : <>
          <div className="mb-2 flex gap-2"><Button variant="subtle" onClick={() => selectAll(active.key)}>Select All</Button><Button variant="subtle" onClick={() => selectNone(active.key, active.tables!)}>Select None</Button></div>
          {active.tables.map((table) => <Checkbox key={table} label={table} checked={!active.hidden.has(table)} onChange={() => toggle(active.key, table)} />)}
        </>}
      </fieldset> : null}
    </div>
    <div className="flex shrink-0 items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
      <span className="mr-auto truncate text-xs text-muted">{result || blocked || `${selectedCount} ${words.tables.toLowerCase()} selected`}</span>
      <Button onClick={onClose}>{result ? 'Close' : 'Cancel'}</Button>
      <Button variant="primary" icon={running ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} fill="currentColor" />} disabled={running || blocked !== ''} onClick={() => void start()}>Dump</Button>
    </div>
  </div>
}
