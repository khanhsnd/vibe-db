import { useEffect, useState } from 'react'
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'

import {
  DEFAULT_MYSQL_PORT,
  TLS_MODES,
  emptyProfileInput,
  connections,
  type ProfileInput,
} from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { Button } from '@/shared/ui/Button'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { Dialog } from '@/shared/ui/Dialog'
import { useConnectionsStore } from '@/stores/connections'

type Probe =
  | { state: 'idle' }
  | { state: 'testing' }
  | { state: 'ok'; version: string; latencyMs: number }
  | { state: 'failed'; error: AppError }

interface ConnectionDialogProps {
  open: boolean
  initial: ProfileInput | null
  onOpenChange: (open: boolean) => void
}

export function ConnectionDialog({ open, initial, onOpenChange }: ConnectionDialogProps) {
  const saveProfile = useConnectionsStore((s) => s.saveProfile)

  const [form, setForm] = useState<ProfileInput>(emptyProfileInput)
  const [probe, setProbe] = useState<Probe>({ state: 'idle' })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<AppError | null>(null)

  useEffect(() => {
    if (!open) return
    setForm(initial ?? emptyProfileInput())
    setProbe({ state: 'idle' })
    setSaveError(null)
  }, [open, initial])

  const update = <K extends keyof ProfileInput>(key: K, value: ProfileInput[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }))
    setProbe({ state: 'idle' })
  }

  const fieldError = (name: string) => {
    const error = probe.state === 'failed' ? probe.error : saveError
    return error?.code === 'INVALID_PROFILE' && error.field === name ? error.message : undefined
  }

  const test = async () => {
    setProbe({ state: 'testing' })
    try {
      const result = await connections.testConnection(form)
      setProbe({ state: 'ok', version: result.serverVersion, latencyMs: result.latencyMs })
    } catch (thrown) {
      setProbe({ state: 'failed', error: toAppError(thrown) })
    }
  }

  const save = async () => {
    setSaving(true)
    setSaveError(null)
    try {
      await saveProfile(form)
      onOpenChange(false)
    } catch (thrown) {
      setSaveError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={initial?.id ? 'Edit Connection' : 'New Connection'}
      description="MySQL"
      width={480}
      footer={
        <>
          <Button variant="subtle" onClick={test} disabled={probe.state === 'testing'}>
            {probe.state === 'testing' ? 'Testing…' : 'Test Connection'}
          </Button>
          <div className="flex-1" />
          <Button variant="default" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label="Name" error={fieldError('name')}>
          {(id, invalid) => (
            <TextInput
              id={id}
              invalid={invalid}
              value={form.name}
              placeholder="localhost"
              onChange={(e) => update('name', e.target.value)}
            />
          )}
        </Field>

        <div className="grid grid-cols-[1fr_110px] gap-2">
          <Field label="Host" error={fieldError('host')}>
            {(id, invalid) => (
              <TextInput
                id={id}
                invalid={invalid}
                value={form.host}
                onChange={(e) => update('host', e.target.value)}
              />
            )}
          </Field>

          <Field label="Port" error={fieldError('port')}>
            {(id, invalid) => (
              <TextInput
                id={id}
                invalid={invalid}
                inputMode="numeric"
                value={form.port === 0 ? '' : String(form.port)}
                placeholder={String(DEFAULT_MYSQL_PORT)}
                onChange={(e) => update('port', Number(e.target.value.replace(/\D/g, '')) || 0)}
              />
            )}
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-2">
          <Field label="User" error={fieldError('user')}>
            {(id, invalid) => (
              <TextInput
                id={id}
                invalid={invalid}
                value={form.user}
                onChange={(e) => update('user', e.target.value)}
              />
            )}
          </Field>

          <Field
            label="Password"
            hint={initial?.id && !form.password ? 'Leave blank to keep the saved password' : undefined}
          >
            {(id) => (
              <TextInput
                id={id}
                type="password"
                value={form.password}
                onChange={(e) => update('password', e.target.value)}
              />
            )}
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-2">
          <Field label="Default database" hint="Optional" error={fieldError('database')}>
            {(id, invalid) => (
              <TextInput
                id={id}
                invalid={invalid}
                value={form.database}
                onChange={(e) => update('database', e.target.value)}
              />
            )}
          </Field>

          <Field label="TLS" error={fieldError('tlsMode')}>
            {(id, invalid) => (
              <SelectInput
                id={id}
                invalid={invalid}
                value={form.tlsMode}
                onChange={(e) => update('tlsMode', e.target.value)}
              >
                {TLS_MODES.map((mode) => (
                  <option key={mode.value} value={mode.value}>
                    {mode.label}
                  </option>
                ))}
              </SelectInput>
            )}
          </Field>
        </div>

        <Checkbox
          label="Save password in Windows Credential Manager"
          checked={form.savePassword}
          onChange={(e) => update('savePassword', e.target.checked)}
        />

        <ProbeResult probe={probe} />
        {saveError && saveError.code !== 'INVALID_PROFILE' ? (
          <Banner tone="danger" text={saveError.message} />
        ) : null}
      </div>
    </Dialog>
  )
}

function ProbeResult({ probe }: { probe: Probe }) {
  if (probe.state === 'idle') return null

  if (probe.state === 'testing') {
    return (
      <Banner tone="muted" icon={<Loader2 size={13} className="animate-spin" />} text="Connecting…" />
    )
  }

  if (probe.state === 'ok') {
    return (
      <Banner
        tone="success"
        icon={<CheckCircle2 size={13} />}
        text={`Connected to ${probe.version} in ${probe.latencyMs} ms`}
      />
    )
  }

  return <Banner tone="danger" icon={<XCircle size={13} />} text={probe.error.message} />
}

const TONES = {
  success: 'border-success/40 bg-success/10 text-success',
  danger: 'border-danger/40 bg-danger-subtle text-danger',
  muted: 'border-line bg-panel text-muted',
} as const

function Banner({
  tone,
  icon,
  text,
}: {
  tone: keyof typeof TONES
  icon?: React.ReactNode
  text: string
}) {
  return (
    <div
      role="status"
      className={`flex items-start gap-2 rounded-md border px-2.5 py-2 text-xs ${TONES[tone]}`}
    >
      {icon ? <span className="mt-px shrink-0">{icon}</span> : null}
      <span className="selectable min-w-0">{text}</span>
    </div>
  )
}
