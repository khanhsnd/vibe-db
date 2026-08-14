import { useEffect, useState } from 'react'
import { Eraser, Timer } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import {
  applyProfileChanges,
  loadProfileConfig,
  previewProfileChanges,
  profileChange,
  type ProfileAction,
  type ProfileChange,
  type ProfileConfig,
} from '@/bridge/profiler'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Checkbox, Field, TextInput } from '@/shared/ui/Field'
import { Loading } from '@/shared/ui/Loading'
import { useSchemaStore } from '@/stores/schema'

/**
 * What a server is told to record, and how slow is slow enough — what `Profiling…` opens over the
 * [Slow Queries pane](../../../../docs/features/profiler.md).
 *
 * It is **read from the server every time it opens**, never from what this app set last: another
 * client may have moved either setting, and a switch drawn from memory would be a switch that lies
 * about the server in front of the reader.
 *
 * Nothing is applied until Save, and Save shows the statements first wherever the connection's
 * environment says they have to be answered for — `PreviewProfileChanges` spells exactly what
 * `ApplyProfileChanges` runs, which is the rule everywhere in this app that a preview exists at all.
 */
export function ProfileDialog({
  sessionId,
  namespace,
  open,
  onClose,
  onApplied,
}: {
  sessionId: string
  namespace: Namespace
  open: boolean
  onClose: () => void
  onApplied: () => void
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose()
      }}
      title="Profiling"
      description="What this server writes down about the statements that run slowly."
      icon={<Timer size={14} />}
      width={520}
      footer={null}
      bodyClassName=""
    >
      {/* Keyed on the database, so opening it against a second one reads that one's settings rather
          than showing the first one's answers. Two engines keep this per database. */}
      {open ? (
        <ProfileForm
          key={namespace.database}
          sessionId={sessionId}
          namespace={namespace}
          onClose={onClose}
          onApplied={onApplied}
        />
      ) : null}
    </Dialog>
  )
}

function ProfileForm({
  sessionId,
  namespace,
  onClose,
  onApplied,
}: {
  sessionId: string
  namespace: Namespace
  onClose: () => void
  onApplied: () => void
}) {
  const ask = useSchemaStore((s) => s.ask)

  const [config, setConfig] = useState<ProfileConfig | null>(null)
  const [enabled, setEnabled] = useState(false)
  const [threshold, setThreshold] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    let live = true
    void (async () => {
      try {
        const read = await loadProfileConfig(sessionId, namespace)
        if (!live) return
        setConfig(read)
        setEnabled(read.enabled)
        setThreshold(String(read.thresholdMs))
      } catch (thrown) {
        if (live) setError(toAppError(thrown))
      }
    })()
    return () => {
      live = false
    }
  }, [sessionId, namespace])

  /**
   * Runs a list of changes, having first asked the Go side what that will send. The statements are
   * what the confirmation shows — there is no Changes pane here, so the preview *is* the sentence —
   * and whether it has to be answered for at all is a `domain` rule already applied against this
   * connection's environment.
   */
  const apply = async (changes: ProfileChange[], title: string, submitLabel: string) => {
    if (changes.length === 0) {
      onClose()
      return
    }

    const run = async () => {
      setBusy(true)
      try {
        await applyProfileChanges(sessionId, namespace, changes)
        onApplied()
        onClose()
      } catch (thrown) {
        setError(toAppError(thrown))
      } finally {
        setBusy(false)
      }
    }

    try {
      const preview = await previewProfileChanges(sessionId, changes)
      if (!preview.confirm) {
        await run()
        return
      }

      ask({
        kind: 'confirm',
        title,
        message: `${sentence(changes)} This runs: ${preview.statements.join('; ')}.`,
        submitLabel,
        danger: changes.some((change) => change.action === 'reset'),
        onSubmit: run,
      })
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }

  if (config === null) {
    return error === null ? (
      <div className="px-4 py-6">
        <Loading label="Asking the server…" />
      </div>
    ) : (
      <div className="px-4 py-4">
        <ErrorStrip error={error} />
      </div>
    )
  }

  // A threshold that is not a number is refused here rather than reaching the Go side, which refuses
  // it too: a `SET GLOBAL` binds no arguments, so the check is worth having on both sides of the wire.
  const milliseconds = Number(threshold)
  const badThreshold = !Number.isInteger(milliseconds) || milliseconds < 0

  const save = () => {
    const changes: ProfileChange[] = []
    if (!enabled && config.enabled) changes.push(profileChange('disable'))
    if (enabled) changes.push(profileChange('enable', milliseconds))

    void apply(changes, 'Change profiling', 'Change')
  }

  const clear = () => {
    void apply([profileChange('reset')], 'Clear what was recorded', 'Clear')
  }

  return (
    <>
      <div className="flex flex-col gap-3.5 px-4 py-3.5">
        {/* What the server said about itself, where the reader is deciding what to tell it. A
            configuration that is not ready is the one thing worth reading before the controls. */}
        {config.ready ? null : <Banner tone="warn" text={config.reason} />}

        <Checkbox
          label="Record statements that run slowly"
          checked={enabled}
          onChange={(event) => setEnabled(event.target.checked)}
        />

        {/* The threshold is drawn only where the engine has one. PostgreSQL counts every statement
            and the reader picks the slow end out of the list, so a box there would be a number that
            changed nothing. */}
        {config.thresholdMs > 0 || threshold !== '0' || enabled ? (
          <Field
            label="Slower than"
            inline
            hint="In milliseconds. Anything quicker is not written down."
            error={badThreshold ? 'A whole number of milliseconds.' : undefined}
          >
            {(controlId, invalid) => (
              <TextInput
                id={controlId}
                invalid={invalid}
                value={threshold}
                disabled={!enabled}
                onChange={(event) => setThreshold(event.target.value)}
                className="w-32"
              />
            )}
          </Field>
        ) : null}

        {/* What this engine says beyond the two settings above, in its own words — the arrangement
            every list in this app has, and for the same reason: no two engines keep the same facts. */}
        <dl className="flex flex-col gap-1 rounded-md border border-line bg-canvas px-3 py-2">
          {config.detail.map((fact) => (
            <div key={fact.label} className="flex items-baseline justify-between gap-4">
              <dt className="text-xs text-faint">{fact.label}</dt>
              <dd className="selectable truncate text-xs text-fg">{fact.value}</dd>
            </div>
          ))}
        </dl>

        {error === null ? null : <ErrorStrip error={error} />}
      </div>

      <div className="flex items-center gap-2 border-t border-line bg-bar px-4 py-2.5">
        <Button
          variant="subtle"
          icon={<Eraser size={13} />}
          disabled={busy}
          onClick={clear}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Clear Recorded
        </Button>

        <div className="ml-auto flex items-center gap-2">
          <Button variant="subtle" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy || badThreshold} onClick={save}>
            Save
          </Button>
        </div>
      </div>
    </>
  )
}

/**
 * The sentence a change is answered for by. It says what the change *does* rather than naming the
 * setting, because the one worth stopping over is the one that throws away rows the reader may be
 * halfway through reading.
 */
function sentence(changes: ProfileChange[]): string {
  const has = (action: ProfileAction) => changes.some((change) => change.action === action)

  if (has('reset')) {
    return 'Throw away everything this server has recorded so far? Nothing puts it back.'
  }
  if (has('disable')) {
    return 'Stop recording slow statements on this server? What was recorded stays where it is.'
  }
  return 'Change what this server records? It is a server-wide setting, so every client feels it.'
}
