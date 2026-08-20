import { useEffect, useState } from 'react'
import { Check, FolderOpen } from 'lucide-react'

import {
  ACCESS_MODES,
  CONNECTION_COLOURS,
  ENVIRONMENTS,
  PROXY_KINDS,
  proxyPort,
  TLS_MODES,
} from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import {
  newStoreInput,
  objects,
  PROVIDERS,
  providerInfo,
  STORE_AUTH,
  type StoreInput,
} from '@/bridge/objects'
import { colourSwatch } from '@/features/connections/ConnectionColour'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Checkbox, Field, PasswordInput, SelectInput, TextInput } from '@/shared/ui/Field'
import { TabStrip } from '@/shared/ui/TabStrip'
import { useObjectStoresStore } from '@/stores/objectStores'

import { StoreIcon } from './StoreIcon'

const TABS = ['General', 'Options', 'Tunnel'] as const
type Tab = (typeof TABS)[number]

/**
 * A connection to an object store. It is `ConnectionDialog`'s counterpart and deliberately shorter:
 * there is no engine to pick, no port, no database and no user — a service is an address, a key pair
 * and a bucket.
 *
 * **Picking a service fills three boxes in**, which is the whole reason that list exists: an endpoint
 * that cannot be worked out from a region, whether the bucket goes in the host or in the path, and
 * what a region means to a service that has none. Every one of them is a fact a reader would
 * otherwise have to know by heart, and every one of them is a 404 with nothing in it when wrong.
 */
export function StoreDialog({
  open,
  initial,
  onOpenChange,
}: {
  open: boolean
  initial: StoreInput | null
  onOpenChange: (open: boolean) => void
}) {
  const save = useObjectStoresStore((s) => s.save)

  const [draft, setDraft] = useState<StoreInput>(newStoreInput)
  const [at, setAt] = useState<Tab>('General')
  const [failure, setFailure] = useState<AppError | null>(null)
  const [tested, setTested] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setDraft(initial ?? newStoreInput())
    setAt('General')
    setFailure(null)
    setTested(null)
  }, [open, initial])

  const provider = providerInfo(draft.provider)

  const change = (edit: Partial<StoreInput>) => {
    setDraft((current) => ({ ...current, ...edit }))
    setTested(null)
  }

  /**
   * Picking a service takes its own three answers with it, and **only where the reader has not typed
   * over them**: a profile being edited keeps the endpoint it has, and a fresh one is filled in. That
   * is what makes the list a shortcut rather than something that undoes work.
   */
  const pickProvider = (value: string) => {
    const picked = providerInfo(value)
    change({
      provider: value,
      pathStyle: picked.pathStyle,
      ...(draft.endpoint === '' || !picked.needsEndpoint ? { endpoint: '' } : {}),
      ...(draft.region === '' && picked.region !== '' ? { region: picked.region } : {}),
    })
  }

  const browse = (title: string, take: (path: string) => void) => {
    void objects
      .chooseFiles(title)
      .then((paths) => {
        const first = paths[0]
        if (first !== undefined) take(first)
      })
      .catch(() => {})
  }

  const test = () => {
    setBusy(true)
    setFailure(null)
    void objects
      .testStore(draft)
      .then((result) => setTested(`Reached it in ${result.latencyMs} ms`))
      .catch((thrown: unknown) => setFailure(toAppError(thrown)))
      .finally(() => setBusy(false))
  }

  const submit = () => {
    setBusy(true)
    setFailure(null)
    void save(draft)
      .then(() => onOpenChange(false))
      .catch((thrown: unknown) => setFailure(toAppError(thrown)))
      .finally(() => setBusy(false))
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={draft.id === '' ? 'New Object Store' : draft.name || 'Object Store'}
      description={provider.label}
      icon={<StoreIcon provider={draft.provider} />}
      tone="plain"
      width={560}
      bodyClassName=""
      footer={
        <div className="flex items-center gap-2">
          <Button disabled={busy} onClick={test}>
            Test Connection
          </Button>
          {tested !== null ? (
            <span className="flex items-center gap-1 text-xs text-success">
              <Check size={13} />
              {tested}
            </span>
          ) : null}
          <span className="flex-1" />
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" disabled={busy} onClick={submit}>
            Save
          </Button>
        </div>
      }
    >
      <TabStrip tabs={TABS} at={at} onMove={setAt} />

      <div className="flex flex-col gap-3 px-4 py-3.5">
        {at === 'General' ? (
          <>
            <Field label="Name">
              {(id) => (
                <TextInput
                  id={id}
                  value={draft.name}
                  autoFocus
                  onChange={(e) => change({ name: e.target.value })}
                />
              )}
            </Field>

            <Field label="Service" hint={provider.hint}>
              {(id) => (
                <SelectInput
                  id={id}
                  value={draft.provider}
                  onChange={(e) => pickProvider(e.target.value)}
                >
                  {PROVIDERS.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <div className="grid grid-cols-2 gap-3">
              <Field
                label={provider.needsEndpoint ? 'Endpoint' : 'Endpoint (optional)'}
                hint={
                  provider.needsEndpoint
                    ? 'A missing scheme is https. Say http:// out loud for a service without TLS.'
                    : 'Left empty it is worked out from the region.'
                }
              >
                {(id) => (
                  <TextInput
                    id={id}
                    value={draft.endpoint}
                    placeholder={provider.endpointHint}
                    spellCheck={false}
                    onChange={(e) => change({ endpoint: e.target.value })}
                  />
                )}
              </Field>

              <Field
                label="Region"
                hint={provider.region === '' ? '' : `Defaults to ${provider.region}.`}
              >
                {(id) => (
                  <TextInput
                    id={id}
                    value={draft.region}
                    placeholder={provider.region || 'eu-west-1'}
                    spellCheck={false}
                    onChange={(e) => change({ region: e.target.value })}
                  />
                )}
              </Field>
            </div>

            <Field
              label="Bucket (optional)"
              hint="Named, the tree opens on it alone — which is what a key that may read one bucket and not list them needs. Left empty, every bucket it can list is drawn."
            >
              {(id) => (
                <TextInput
                  id={id}
                  value={draft.bucket}
                  spellCheck={false}
                  onChange={(e) => change({ bucket: e.target.value })}
                />
              )}
            </Field>

            <Field
              label="Signing in with"
              hint={STORE_AUTH.find((one) => one.value === draft.auth)?.hint ?? ''}
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={draft.auth}
                  onChange={(e) => change({ auth: e.target.value })}
                >
                  {STORE_AUTH.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            {draft.auth === 'key' ? (
              <>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Access key ID">
                    {(id) => (
                      <TextInput
                        id={id}
                        value={draft.accessKeyId}
                        spellCheck={false}
                        onChange={(e) => change({ accessKeyId: e.target.value })}
                      />
                    )}
                  </Field>

                  <Field
                    label="Secret access key"
                    hint={
                      initial !== null && draft.secretKey === ''
                        ? 'One is already stored. Leave this empty to keep it.'
                        : ''
                    }
                  >
                    {(id) => (
                      <PasswordInput
                        id={id}
                        value={draft.secretKey}
                        onChange={(e) => change({ secretKey: e.target.value })}
                      />
                    )}
                  </Field>
                </div>

                <Checkbox
                  label="Keep the secret in this machine's credential store"
                  checked={draft.saveSecret}
                  onChange={(e) => change({ saveSecret: e.target.checked })}
                />
              </>
            ) : null}

            <Checkbox
              label="Name the bucket in the path rather than in the host"
              checked={draft.pathStyle}
              onChange={(e) => change({ pathStyle: e.target.checked })}
            />
          </>
        ) : null}

        {at === 'Options' ? (
          <>
            <Field
              label="Environment"
              hint="What this connection is for. A production one asks before anything that cannot be taken back."
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={draft.environment}
                  onChange={(e) => change({ environment: e.target.value })}
                >
                  {ENVIRONMENTS.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <Field
              label="Colour"
              hint="How the sidebar tells this connection from the one beside it."
            >
              {() => (
                <div className="flex flex-wrap items-center gap-1.5">
                  {CONNECTION_COLOURS.map((one) => (
                    <button
                      key={one.value}
                      type="button"
                      aria-label={one.label}
                      title={one.label}
                      onClick={() => change({ colour: one.value })}
                      className={cn(
                        'grid h-6 w-6 place-items-center rounded-md border border-line hover:bg-hover',
                        draft.colour === one.value && 'border-accent',
                      )}
                    >
                      {colourSwatch(one.value, draft.colour === one.value)}
                    </button>
                  ))}
                </div>
              )}
            </Field>

            <Field
              label="Access"
              hint={ACCESS_MODES.find((one) => one.value === draft.access)?.hint ?? ''}
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={draft.access}
                  onChange={(e) => change({ access: e.target.value })}
                >
                  {ACCESS_MODES.map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <Field
              label="Certificate"
              hint="What the service's certificate is held to. It is verified by default, which is the one place this app picks a mode rather than leaving it at the reader's own."
            >
              {(id) => (
                <SelectInput
                  id={id}
                  value={draft.tls.mode}
                  onChange={(e) => change({ tls: { ...draft.tls, mode: e.target.value } })}
                >
                  {TLS_MODES.filter(
                    (one) => one.value !== 'disabled' && one.value !== 'preferred',
                  ).map((one) => (
                    <option key={one.value} value={one.value}>
                      {one.label}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <Field
              label="Root certificate (optional)"
              hint="For a service behind a private authority."
            >
              {(id) => (
                <div className="flex items-center gap-1.5">
                  <TextInput
                    id={id}
                    value={draft.tls.rootCert}
                    className="min-w-0 flex-1"
                    onChange={(e) => change({ tls: { ...draft.tls, rootCert: e.target.value } })}
                  />
                  <Button
                    icon={<FolderOpen size={13} />}
                    onClick={() =>
                      browse('Root certificate', (path) =>
                        change({ tls: { ...draft.tls, rootCert: path } }),
                      )
                    }
                  >
                    Browse
                  </Button>
                </div>
              )}
            </Field>
          </>
        ) : null}

        {at === 'Tunnel' ? (
          <>
            {/* Both hops are the two a database connection has, and they are carried the same way
                round: the proxy is the outer one, so the bastion is dialled *through* it. */}
            <Checkbox
              label="Reach the service through a proxy"
              checked={draft.proxy.enabled}
              onChange={(e) => change({ proxy: { ...draft.proxy, enabled: e.target.checked } })}
            />

            {draft.proxy.enabled ? (
              <div className="grid grid-cols-3 gap-3">
                <Field label="Kind">
                  {(id) => (
                    <SelectInput
                      id={id}
                      value={draft.proxy.kind}
                      onChange={(e) =>
                        change({
                          proxy: {
                            ...draft.proxy,
                            kind: e.target.value,
                            port: proxyPort(e.target.value),
                          },
                        })
                      }
                    >
                      {PROXY_KINDS.map((one) => (
                        <option key={one.value} value={one.value}>
                          {one.label}
                        </option>
                      ))}
                    </SelectInput>
                  )}
                </Field>

                <Field label="Host">
                  {(id) => (
                    <TextInput
                      id={id}
                      value={draft.proxy.host}
                      onChange={(e) => change({ proxy: { ...draft.proxy, host: e.target.value } })}
                    />
                  )}
                </Field>

                <Field label="Port">
                  {(id) => (
                    <TextInput
                      id={id}
                      value={draft.proxy.port === 0 ? '' : String(draft.proxy.port)}
                      inputMode="numeric"
                      onChange={(e) =>
                        change({ proxy: { ...draft.proxy, port: Number(e.target.value) || 0 } })
                      }
                    />
                  )}
                </Field>
              </div>
            ) : null}

            <Checkbox
              label="Reach the service through an SSH bastion"
              checked={draft.ssh.enabled}
              onChange={(e) => change({ ssh: { ...draft.ssh, enabled: e.target.checked } })}
            />

            {draft.ssh.enabled ? (
              <>
                <div className="grid grid-cols-3 gap-3">
                  <Field label="Host">
                    {(id) => (
                      <TextInput
                        id={id}
                        value={draft.ssh.host}
                        onChange={(e) => change({ ssh: { ...draft.ssh, host: e.target.value } })}
                      />
                    )}
                  </Field>

                  <Field label="Port">
                    {(id) => (
                      <TextInput
                        id={id}
                        value={draft.ssh.port === 0 ? '' : String(draft.ssh.port)}
                        inputMode="numeric"
                        onChange={(e) =>
                          change({ ssh: { ...draft.ssh, port: Number(e.target.value) || 0 } })
                        }
                      />
                    )}
                  </Field>

                  <Field label="User">
                    {(id) => (
                      <TextInput
                        id={id}
                        value={draft.ssh.user}
                        onChange={(e) => change({ ssh: { ...draft.ssh, user: e.target.value } })}
                      />
                    )}
                  </Field>
                </div>

                <Field label="Private key">
                  {(id) => (
                    <div className="flex items-center gap-1.5">
                      <TextInput
                        id={id}
                        value={draft.ssh.keyPath}
                        className="min-w-0 flex-1"
                        onChange={(e) => change({ ssh: { ...draft.ssh, keyPath: e.target.value } })}
                      />
                      <Button
                        icon={<FolderOpen size={13} />}
                        onClick={() =>
                          browse('Private key', (path) =>
                            change({ ssh: { ...draft.ssh, keyPath: path } }),
                          )
                        }
                      >
                        Browse
                      </Button>
                    </div>
                  )}
                </Field>

                <Field
                  label="Pass phrase"
                  hint={
                    initial !== null && draft.sshSecret === ''
                      ? 'One is already stored. Leave this empty to keep it.'
                      : ''
                  }
                >
                  {(id) => (
                    <PasswordInput
                      id={id}
                      value={draft.sshSecret}
                      onChange={(e) => change({ sshSecret: e.target.value })}
                    />
                  )}
                </Field>
              </>
            ) : null}
          </>
        ) : null}

        {failure !== null ? <ErrorStrip error={failure} /> : null}
      </div>
    </Dialog>
  )
}
