import { useEffect, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  FolderOpen,
  Loader2,
  PlugZap,
  Save,
  XCircle,
} from 'lucide-react'

import {
  ACCESS_MODES,
  AUTH_METHODS,
  accessInfo,
  authInfo,
  ENVIRONMENTS,
  PROXY_KINDS,
  SSH_AUTHS,
  TLS_MODES,
  emptyProfileInput,
  engineInfo,
  connections,
  proxyPort,
  tlsUsesFiles,
  withEngine,
  type Engine,
  type ProfileInput,
} from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { files } from '@/bridge/files'
import { ResultTabs } from '@/features/results/ResultTabs'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Checkbox, Field, PasswordInput, SelectInput, TextInput } from '@/shared/ui/Field'
import { Dialog } from '@/shared/ui/Dialog'
import { useConnectionsStore } from '@/stores/connections'

import { EngineIcon } from './EngineIcon'
import { EnginePicker } from './EnginePicker'
import { EnvironmentDot } from './EnvironmentDot'
import { McpPane } from './McpPane'

/** Tall enough that every tab is the same height, so switching one never moves the footer. */
const PANE_HEIGHT = 316

/**
 * What the file picker offers first for a certificate and for a key. Neither is a restriction — All
 * files sits behind it — and the boxes with no pattern at all are the two whose file is usually
 * named without an extension: an SSH private key, and `known_hosts`.
 */
const CERTIFICATES = '*.pem;*.crt;*.cer;*.der'
const KEYS = '*.pem;*.key'

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
  // A new connection names its type first; an existing one already has one, so it opens on the form.
  const [picking, setPicking] = useState(true)
  const [probe, setProbe] = useState<Probe>({ state: 'idle' })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<AppError | null>(null)

  useEffect(() => {
    if (!open) return
    setForm(initial ?? emptyProfileInput())
    setPicking(!initial?.id)
    setProbe({ state: 'idle' })
    setSaveError(null)
  }, [open, initial])

  const engine = engineInfo(form.engine)

  const update = <K extends keyof ProfileInput>(key: K, value: ProfileInput[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }))
    setProbe({ state: 'idle' })
  }

  const updateTLS = <K extends keyof ProfileInput['tls']>(
    key: K,
    value: ProfileInput['tls'][K],
  ) => update('tls', { ...form.tls, [key]: value })

  const updateSSH = <K extends keyof ProfileInput['ssh']>(
    key: K,
    value: ProfileInput['ssh'][K],
  ) => update('ssh', { ...form.ssh, [key]: value })

  const updateProxy = <K extends keyof ProfileInput['proxy']>(
    key: K,
    value: ProfileInput['proxy'][K],
  ) => update('proxy', { ...form.proxy, [key]: value })

  // Changing the kind brings its own default port with it, but only where the box still holds the
  // other kind's default — the rule `withEngine` follows, for the same reason: a typed port is the
  // reader's.
  const changeProxyKind = (kind: string) =>
    update('proxy', {
      ...form.proxy,
      kind,
      port: form.proxy.port === proxyPort(form.proxy.kind) ? proxyPort(kind) : form.proxy.port,
    })

  const changeEngine = (value: Engine) => {
    setForm((prev) => withEngine(prev, value))
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

  const shell = {
    open,
    onOpenChange,
    title: initial?.id ? 'Edit Connection' : 'New Connection',
    icon: <EngineIcon engine={engine.value} className="h-6 w-6 rounded-md p-0.5" />,
    tone: 'plain',
    width: 520,
  } as const

  if (picking) {
    return (
      <Dialog
        {...shell}
        description="Select a connection type"
        footer={
          <>
            <div className="flex-1" />
            <Button variant="default" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              icon={<ArrowRight size={13} />}
              onClick={() => setPicking(false)}
            >
              Next
            </Button>
          </>
        }
      >
        <EnginePicker
          value={engine.value}
          onChange={changeEngine}
          onConfirm={() => setPicking(false)}
        />
      </Dialog>
    )
  }

  const auth = authInfo(form.auth)
  const access = accessInfo(form.access)

  return (
    <Dialog
      {...shell}
      description={engine.label}
      bodyClassName=""
      footer={
        <>
          <Button variant="subtle" icon={<ArrowLeft size={13} />} onClick={() => setPicking(true)}>
            Back
          </Button>
          <Button
            variant="subtle"
            icon={<PlugZap size={13} />}
            onClick={test}
            disabled={probe.state === 'testing'}
          >
            {probe.state === 'testing' ? 'Testing…' : 'Test Connection'}
          </Button>
          <div className="flex-1" />
          <Button variant="default" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" icon={<Save size={13} />} onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </>
      }
    >
      <div style={{ height: PANE_HEIGHT }}>
        <ResultTabs
          panes={[
            {
              id: 'general',
              label: 'General',
              content: (
                <Pane>
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

                  {/* A file engine has no server, so it is asked for a *path* and nothing else.
                      The three boxes that would otherwise be here are not disabled but absent:
                      there is no host to fill in later, and a greyed-out Password would suggest
                      the file might have one. */}
                  {engine.isFile ? (
                    <FileField
                      label={engine.databaseLabel}
                      hint={engine.databaseHint}
                      value={form.database}
                      disabled={false}
                      error={fieldError('database')}
                      pattern="*.db;*.sqlite;*.sqlite3"
                      onChange={(value) => update('database', value)}
                    />
                  ) : (
                    <>
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
                              placeholder={String(engine.port)}
                              onChange={(e) =>
                                update('port', Number(e.target.value.replace(/\D/g, '')) || 0)
                              }
                            />
                          )}
                        </Field>
                      </div>

                      <div className="grid grid-cols-2 gap-2">
                        {/* A MongoDB server started without access control and a Redis server with
                            no ACL take no credentials at all, so the box says so rather than being
                            refused when it is left empty. */}
                        <Field
                          label="User"
                          hint={engine.needsUser ? undefined : 'Optional'}
                          error={fieldError('user')}
                        >
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
                          hint={passwordHint(form, auth.minted, Boolean(initial?.id))}
                        >
                          {(id) => (
                            <PasswordInput
                              id={id}
                              disabled={auth.minted}
                              value={form.password}
                              onChange={(e) => update('password', e.target.value)}
                            />
                          )}
                        </Field>
                      </div>

                      {/* A Redis database is numbered rather than named, so the box takes an index
                          and says which — leaving it blank is database 0. An Oracle one is not a
                          database at all but the *service* the listener is asked for. */}
                      <Field
                        label={engine.databaseLabel}
                        hint={engine.databaseHint}
                        error={fieldError('database')}
                      >
                        {(id, invalid) => (
                          <TextInput
                            id={id}
                            invalid={invalid}
                            value={form.database}
                            placeholder={engine.keys ? '0' : undefined}
                            onChange={(e) => update('database', e.target.value)}
                          />
                        )}
                      </Field>
                    </>
                  )}

                  <Field
                    label="Environment"
                    hint="Colours this connection everywhere, and decides what is confirmed before it runs."
                    error={fieldError('environment')}
                  >
                    {(id, invalid) => (
                      <div className="flex items-center gap-2">
                        <EnvironmentDot environment={form.environment} />
                        <SelectInput
                          id={id}
                          invalid={invalid}
                          value={form.environment}
                          onChange={(e) => update('environment', e.target.value)}
                        >
                          {ENVIRONMENTS.map((one) => (
                            <option key={one.value} value={one.value}>
                              {one.label}
                            </option>
                          ))}
                        </SelectInput>
                      </div>
                    )}
                  </Field>

                  {/* What is *refused* sits beside what is confirmed, because they are the same
                      kind of fact about a connection and are read together: an environment turns a
                      hazard into a question, and this turns one into a refusal. */}
                  <Field label="Access" hint={access.hint} error={fieldError('access')}>
                    {(id, invalid) => (
                      <SelectInput
                        id={id}
                        invalid={invalid}
                        value={form.access}
                        onChange={(e) => update('access', e.target.value)}
                      >
                        {ACCESS_MODES.map((one) => (
                          <option key={one.value} value={one.value}>
                            {one.label}
                          </option>
                        ))}
                      </SelectInput>
                    )}
                  </Field>

                  <Checkbox
                    label="Save password in Windows Credential Manager"
                    checked={form.savePassword}
                    disabled={auth.minted}
                    onChange={(e) => update('savePassword', e.target.checked)}
                  />
                </Pane>
              ),
            },
            {
              id: 'security',
              label: 'Security',
              content: (
                <Pane>
                  <Field label="Authentication" hint={auth.hint} error={fieldError('auth')}>
                    {(id, invalid) => (
                      <SelectInput
                        id={id}
                        invalid={invalid}
                        value={form.auth}
                        onChange={(e) => update('auth', e.target.value)}
                      >
                        {AUTH_METHODS.map((one) => (
                          <option key={one.value} value={one.value}>
                            {one.label}
                          </option>
                        ))}
                      </SelectInput>
                    )}
                  </Field>

                  {form.auth === 'aws-iam' ? (
                    <Field
                      label="AWS region"
                      hint="Leave blank to read it out of an RDS endpoint, or from AWS_REGION."
                    >
                      {(id) => (
                        <TextInput
                          id={id}
                          value={form.region}
                          placeholder="ap-southeast-1"
                          onChange={(e) => update('region', e.target.value)}
                        />
                      )}
                    </Field>
                  ) : null}

                  <Field label="TLS" hint={tlsHint(form.tls.mode)} error={fieldError('tlsMode')}>
                    {(id, invalid) => (
                      <SelectInput
                        id={id}
                        invalid={invalid}
                        value={form.tls.mode}
                        onChange={(e) => updateTLS('mode', e.target.value)}
                      >
                        {TLS_MODES.map((mode) => (
                          <option key={mode.value} value={mode.value}>
                            {mode.label}
                          </option>
                        ))}
                      </SelectInput>
                    )}
                  </Field>

                  <FileField
                    label="Root certificate"
                    hint="Blank uses this machine’s own trust store."
                    disabled={!tlsUsesFiles(form.tls.mode)}
                    value={form.tls.rootCert}
                    pattern={CERTIFICATES}
                    onChange={(value) => updateTLS('rootCert', value)}
                  />

                  <div className="grid grid-cols-2 gap-2">
                    <FileField
                      label="Client certificate"
                      disabled={!tlsUsesFiles(form.tls.mode)}
                      value={form.tls.clientCert}
                      error={fieldError('clientCert')}
                      pattern={CERTIFICATES}
                      onChange={(value) => updateTLS('clientCert', value)}
                    />
                    <FileField
                      label="Client key"
                      disabled={!tlsUsesFiles(form.tls.mode)}
                      value={form.tls.clientKey}
                      pattern={KEYS}
                      onChange={(value) => updateTLS('clientKey', value)}
                    />
                  </div>

                  <Field
                    label="Certificate name"
                    hint="The name the certificate is checked against, when it is not the host dialled."
                  >
                    {(id) => (
                      <TextInput
                        id={id}
                        disabled={!tlsUsesFiles(form.tls.mode)}
                        value={form.tls.serverName}
                        placeholder={form.host}
                        onChange={(e) => updateTLS('serverName', e.target.value)}
                      />
                    )}
                  </Field>
                </Pane>
              ),
            },
            {
              id: 'proxy',
              label: 'Proxy',
              content: (
                <Pane>
                  <Checkbox
                    label="Connect through a proxy"
                    checked={form.proxy.enabled}
                    onChange={(e) => updateProxy('enabled', e.target.checked)}
                  />

                  <p className="text-xs text-faint">
                    The <em>outer</em> hop: a proxy is how this machine reaches the network, so a
                    connection that also uses SSH dials the bastion through the proxy. The address is
                    resolved on the far side, exactly as a tunnel’s is.
                  </p>

                  <div className="grid grid-cols-2 gap-2">
                    <Field label="Proxy type" error={fieldError('proxyKind')}>
                      {(id, invalid) => (
                        <SelectInput
                          id={id}
                          invalid={invalid}
                          disabled={!form.proxy.enabled}
                          value={form.proxy.kind}
                          onChange={(e) => changeProxyKind(e.target.value)}
                        >
                          {PROXY_KINDS.map((one) => (
                            <option key={one.value} value={one.value}>
                              {one.label}
                            </option>
                          ))}
                        </SelectInput>
                      )}
                    </Field>

                    <Field label="Proxy port" error={fieldError('proxyPort')}>
                      {(id, invalid) => (
                        <TextInput
                          id={id}
                          invalid={invalid}
                          inputMode="numeric"
                          disabled={!form.proxy.enabled}
                          value={form.proxy.port === 0 ? '' : String(form.proxy.port)}
                          placeholder={String(proxyPort(form.proxy.kind))}
                          onChange={(e) =>
                            updateProxy('port', Number(e.target.value.replace(/\D/g, '')) || 0)
                          }
                        />
                      )}
                    </Field>
                  </div>

                  <Field label="Proxy host" error={fieldError('proxyHost')}>
                    {(id, invalid) => (
                      <TextInput
                        id={id}
                        invalid={invalid}
                        disabled={!form.proxy.enabled}
                        value={form.proxy.host}
                        placeholder="proxy.example.com"
                        onChange={(e) => updateProxy('host', e.target.value)}
                      />
                    )}
                  </Field>

                  {/* Most proxies on a private network ask for nobody at all, so the box says so
                      rather than being refused when it is left empty. */}
                  <div className="grid grid-cols-2 gap-2">
                    <Field label="Proxy user" hint="Optional">
                      {(id) => (
                        <TextInput
                          id={id}
                          disabled={!form.proxy.enabled}
                          value={form.proxy.user}
                          onChange={(e) => updateProxy('user', e.target.value)}
                        />
                      )}
                    </Field>

                    <Field label="Proxy password" hint={proxySecretHint(form)}>
                      {(id) => (
                        <PasswordInput
                          id={id}
                          disabled={!form.proxy.enabled || form.proxy.user === ''}
                          value={form.proxySecret}
                          onChange={(e) => update('proxySecret', e.target.value)}
                        />
                      )}
                    </Field>
                  </div>
                </Pane>
              ),
            },
            {
              id: 'ssh',
              label: 'SSH',
              content: (
                <Pane>
                  <Checkbox
                    label="Connect through an SSH tunnel"
                    checked={form.ssh.enabled}
                    onChange={(e) => updateSSH('enabled', e.target.checked)}
                  />

                  <p className="text-xs text-faint">
                    The host and port above are resolved <em>on the far side</em> of the tunnel, so
                    a server bound to loopback there is reached by naming 127.0.0.1.
                  </p>

                  <div className="grid grid-cols-[1fr_110px] gap-2">
                    <Field label="SSH host" error={fieldError('sshHost')}>
                      {(id, invalid) => (
                        <TextInput
                          id={id}
                          invalid={invalid}
                          disabled={!form.ssh.enabled}
                          value={form.ssh.host}
                          placeholder="bastion.example.com"
                          onChange={(e) => updateSSH('host', e.target.value)}
                        />
                      )}
                    </Field>

                    <Field label="SSH port" error={fieldError('sshPort')}>
                      {(id, invalid) => (
                        <TextInput
                          id={id}
                          invalid={invalid}
                          inputMode="numeric"
                          disabled={!form.ssh.enabled}
                          value={form.ssh.port === 0 ? '' : String(form.ssh.port)}
                          placeholder="22"
                          onChange={(e) =>
                            updateSSH('port', Number(e.target.value.replace(/\D/g, '')) || 0)
                          }
                        />
                      )}
                    </Field>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <Field label="SSH user" error={fieldError('sshUser')}>
                      {(id, invalid) => (
                        <TextInput
                          id={id}
                          invalid={invalid}
                          disabled={!form.ssh.enabled}
                          value={form.ssh.user}
                          onChange={(e) => updateSSH('user', e.target.value)}
                        />
                      )}
                    </Field>

                    <Field label="SSH authentication" error={fieldError('sshAuth')}>
                      {(id, invalid) => (
                        <SelectInput
                          id={id}
                          invalid={invalid}
                          disabled={!form.ssh.enabled}
                          value={form.ssh.auth}
                          onChange={(e) => updateSSH('auth', e.target.value)}
                        >
                          {SSH_AUTHS.map((one) => (
                            <option key={one.value} value={one.value}>
                              {one.label}
                            </option>
                          ))}
                        </SelectInput>
                      )}
                    </Field>
                  </div>

                  {form.ssh.auth === 'key' ? (
                    <FileField
                      label="Private key"
                      disabled={!form.ssh.enabled}
                      value={form.ssh.keyPath}
                      error={fieldError('sshKeyPath')}
                      onChange={(value) => updateSSH('keyPath', value)}
                    />
                  ) : null}

                  {form.ssh.auth === 'agent' ? null : (
                    <Field
                      label={form.ssh.auth === 'key' ? 'Key pass phrase' : 'SSH password'}
                      hint={sshSecretHint(form)}
                    >
                      {(id) => (
                        <PasswordInput
                          id={id}
                          disabled={!form.ssh.enabled}
                          value={form.sshSecret}
                          onChange={(e) => update('sshSecret', e.target.value)}
                        />
                      )}
                    </Field>
                  )}

                  <Checkbox
                    label="Check the SSH host key against known_hosts"
                    checked={form.ssh.strictHostKey}
                    disabled={!form.ssh.enabled}
                    onChange={(e) => updateSSH('strictHostKey', e.target.checked)}
                  />

                  <FileField
                    label="known_hosts"
                    hint="Blank uses ~/.ssh/known_hosts."
                    disabled={!form.ssh.enabled || !form.ssh.strictHostKey}
                    value={form.ssh.knownHosts}
                    onChange={(value) => updateSSH('knownHosts', value)}
                  />
                </Pane>
              ),
            },
            {
              id: 'mcp',
              label: 'MCP',
              content: (
                <Pane>
                  <McpPane enabled={form.mcp} onChange={(on) => update('mcp', on)} />
                </Pane>
              ),
            },
          ]}
        />
      </div>

      <div className="flex flex-col gap-2 px-4 pb-3">
        <ProbeResult probe={probe} engine={engine.label} />
        {saveError && saveError.code !== 'INVALID_PROFILE' ? (
          <Banner tone="danger" text={saveError.message} />
        ) : null}
      </div>
    </Dialog>
  )
}

function Pane({ children }: { children: React.ReactNode }) {
  return <div className="scroll-thin flex h-full flex-col gap-3 overflow-y-auto px-4 py-3.5">{children}</div>
}

/**
 * A path on this machine, picked or typed. **Both**, deliberately: `Browse` is what a reader who has
 * the file in front of them wants, and the box stays writable because the path is as often already
 * on the clipboard — out of `gcloud`, out of a download, out of a colleague's message.
 *
 * `pattern` is what the picker offers first, and is left empty for a file that usually carries no
 * extension at all — a private key, a `known_hosts`. Nothing is read here either way: the path is
 * saved on the profile and the file is opened at connect time.
 */
function FileField({
  label,
  hint,
  value,
  disabled,
  error,
  pattern,
  onChange,
}: {
  label: string
  hint?: string
  value: string
  disabled: boolean
  error?: string | undefined
  pattern?: string
  onChange: (value: string) => void
}) {
  // A picker that will not open is not a failure the reader can act on — there is nothing wrong with
  // the connection they are filling in — so it leaves the box as it was, and they type the path.
  const browse = () => {
    void files
      .choose(label, pattern)
      .then((path) => path !== '' && onChange(path))
      .catch(() => {})
  }

  return (
    <Field label={label} hint={hint} error={error}>
      {(id, invalid) => (
        <div className="flex items-center gap-2">
          <TextInput
            id={id}
            invalid={invalid}
            disabled={disabled}
            value={value}
            placeholder={disabled ? '' : 'C:\\certs\\ca.pem'}
            className="font-mono text-xs"
            onChange={(e) => onChange(e.target.value)}
          />
          <Button
            variant="default"
            icon={<FolderOpen size={12} />}
            disabled={disabled}
            onClick={browse}
          >
            Browse
          </Button>
        </div>
      )}
    </Field>
  )
}

function tlsHint(mode: string): string {
  return TLS_MODES.find((one) => one.value === mode)?.hint ?? ''
}

function passwordHint(form: ProfileInput, minted: boolean, editing: boolean): string | undefined {
  if (minted) return 'Signed for at connect time — there is nothing to type or to save.'
  if (editing && form.password === '') return 'Leave blank to keep the saved password'
  return undefined
}

function sshSecretHint(form: ProfileInput): string | undefined {
  return form.ssh.enabled ? 'Leave blank to keep what is saved' : undefined
}

function proxySecretHint(form: ProfileInput): string | undefined {
  return form.proxy.enabled && form.proxy.user !== ''
    ? 'Leave blank to keep what is saved'
    : undefined
}

function ProbeResult({ probe, engine }: { probe: Probe; engine: string }) {
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
        text={`Connected to ${engine} ${probe.version} in ${probe.latencyMs} ms`}
      />
    )
  }

  // A tunnel or a cloud identity says more than one line can hold, and what it says is exactly
  // what has to be changed — a variable to set, a host key to enrol.
  return (
    <Banner
      tone="danger"
      icon={<XCircle size={13} />}
      text={probe.error.detail ? `${probe.error.message} — ${probe.error.detail}` : probe.error.message}
    />
  )
}
