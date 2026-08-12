import { useEffect, useMemo, useState } from 'react'

import {
  accountLabel,
  loadAccount,
  loadAccountOptions,
  previewAccountChanges,
  applyAccountChanges,
  type AccountAttribute,
  type AccountDetail,
  type AccountOptions,
  type AccountRef,
  type Grant,
  type GrantLevel,
} from '@/bridge/accounts'
import { engineInfo } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { Cell, GridData } from '@/features/results/resultSet'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Field, PasswordInput, SelectInput, TextInput } from '@/shared/ui/Field'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { TabStrip } from '@/shared/ui/TabStrip'
import { useConnectionsStore } from '@/stores/connections'
import { useSchemaStore } from '@/stores/schema'

import {
  changesOf,
  draftOf,
  grantAt,
  held,
  newDraft,
  toggleGrant,
  toggleGrantable,
  toggleRole,
  type AccountDraft,
} from './accountDraft'

type Pane = 'general' | 'privileges' | 'roles' | 'changes'

const PANE_LABEL: Record<Pane, string> = {
  general: 'General',
  privileges: 'Privileges',
  roles: 'Roles',
  changes: 'Changes',
}

/** What each level is called where a reader picks one. The words are the app's; the *names* under
 *  them are the engine's, and never translated. */
const LEVEL_LABEL: Record<GrantLevel, string> = {
  server: 'Server',
  database: 'Database',
  schema: 'Schema',
  table: 'Table',
  column: 'Column',
  routine: 'Routine',
}

interface AccountEditorProps {
  sessionId: string
  profileId: string
  /** `null` opens the editor on a **new** account, which is the whole difference between the two. */
  account: AccountRef | null
  onSaved: (ref: AccountRef) => void
}

/**
 * One account, edited as a draft: what the server holds is read once, the reader draws over it, and
 * the **Changes** pane is exactly what Apply will run. It is `SourceEditor`'s arrangement and it is
 * here for the stronger reason — a grant is not a thing anybody should press blind.
 *
 * Nothing in this file spells a statement, and nothing decides what is dangerous. Both answers come
 * back from the Go side: the statements from the driver, the confirmation from `domain.GrantHazard`
 * paired with this connection's environment.
 */
export function AccountEditor({ sessionId, profileId, account, onSaved }: AccountEditorProps) {
  const [options, setOptions] = useState<AccountOptions | null>(null)
  const [draft, setDraft] = useState<AccountDraft | null>(null)
  const [pane, setPane] = useState<Pane>('general')
  const [error, setError] = useState<AppError | null>(null)
  const [statements, setStatements] = useState<string[]>([])
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)

  const ask = useSchemaStore((s) => s.ask)
  const refreshAccounts = useConnectionsStore((s) => s.refreshAccounts)
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === profileId)?.engine ?? '',
  )
  const info = engineInfo(engine)

  useEffect(() => {
    let live = true
    setDraft(null)
    setError(null)

    void (async () => {
      try {
        const read = await loadAccountOptions(sessionId)
        if (!live) return
        setOptions(read)

        const next = account ? draftOf(await loadAccount(sessionId, account)) : newDraft(read.attributes)
        if (live) setDraft(next)
      } catch (thrown) {
        if (live) setError(toAppError(thrown))
      }
    })()

    return () => {
      live = false
    }
  }, [sessionId, account])

  const changes = useMemo(() => (draft ? changesOf(draft) : []), [draft])

  // The preview is re-asked whenever the draft moves, because it *is* the draft: a Changes pane
  // showing the statements of a moment ago would be the one thing this pane exists to prevent.
  useEffect(() => {
    let live = true
    if (changes.length === 0) {
      setStatements([])
      setConfirm(false)
      return
    }

    void (async () => {
      try {
        const preview = await previewAccountChanges(sessionId, changes)
        if (!live) return
        setStatements(preview.statements)
        setConfirm(preview.confirm)
      } catch (thrown) {
        if (live) setError(toAppError(thrown))
      }
    })()

    return () => {
      live = false
    }
  }, [sessionId, changes])

  if (error && draft === null) return <Empty title={error.message} hint={error.detail} />
  if (draft === null || options === null) return <Loading label="Reading the account…" />

  const apply = async () => {
    setBusy(true)
    try {
      await applyAccountChanges(sessionId, changes)
      await refreshAccounts(profileId)
      setError(null)
      onSaved(draft.ref)
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setBusy(false)
    }
  }

  const save = () => {
    if (!confirm) {
      void apply()
      return
    }
    // The answer to whether this has to be confirmed came from the Go side. Nothing here re-derives
    // it from the hazard, which is the rule the query tab follows for the same reason.
    ask({
      kind: 'confirm',
      title: 'Apply these changes?',
      message:
        `${statements.length} statement${statements.length === 1 ? '' : 's'} will run against ` +
        `this connection, and one of them grants more than it takes away.`,
      submitLabel: 'Apply',
      danger: true,
      onSubmit: apply,
    })
  }

  // A tab for a thing the engine has not got is a tab of empty boxes — MongoDB grants roles and no
  // privileges, Redis has no roles at all — so each is drawn only where there is something in it.
  const panes: Pane[] = [
    'general',
    ...(options.privileges.length > 0 ? (['privileges'] as Pane[]) : []),
    ...(info.hasRoles && options.roles.length > 0 ? (['roles'] as Pane[]) : []),
    'changes',
  ]
  const paneLabel = (one: Pane) =>
    one === 'changes' && changes.length > 0 ? `Changes (${statements.length})` : PANE_LABEL[one]

  const movePane = (label: string) => {
    const found = panes.find((one) => paneLabel(one) === label)
    if (found) setPane(found)
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5">
        <TabStrip tabs={panes.map(paneLabel)} at={paneLabel(pane)} onMove={movePane} />
        <span className="flex-1" />
        <Button
          variant="primary"
          icon={ICON.migrate}
          disabled={busy || changes.length === 0 || draft.ref.name.trim() === ''}
          onClick={save}
        >
          Apply
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {pane === 'general' ? (
          <GeneralPane
            draft={draft}
            options={options}
            label={info.accountLabel}
            onChange={setDraft}
          />
        ) : pane === 'privileges' ? (
          <PrivilegePane draft={draft} options={options} onChange={setDraft} />
        ) : pane === 'roles' ? (
          <RolePane draft={draft} options={options} onChange={setDraft} />
        ) : (
          <ChangesPane statements={statements} source={draft.original?.source ?? []} />
        )}
      </div>

      {error ? <ErrorStrip error={error} /> : null}
    </div>
  )
}

function GeneralPane({
  draft,
  options,
  label,
  onChange,
}: {
  draft: AccountDraft
  options: AccountOptions
  label: string
  onChange: (draft: AccountDraft) => void
}) {
  const set = (edit: Partial<AccountDraft>) => onChange({ ...draft, ...edit })
  const setAttribute = (name: string, value: string) =>
    onChange({ ...draft, attributes: { ...draft.attributes, [name]: value } })

  return (
    <div className="max-w-2xl space-y-2 p-3">
      <Field inline label={`${label} Name`}>
        {(id) => (
          <TextInput
            id={id}
            value={draft.ref.name}
            autoFocus={draft.original === null}
            onChange={(e) => set({ ref: { ...draft.ref, name: e.target.value } })}
          />
        )}
      </Field>

      {/* Only the engine whose account **is** a pair draws a host, and it offers the hosts the
          server already knows rather than a box to mistype one into. */}
      {options.hosts.length > 0 ? (
        <Field inline label="Host" hint="an account here is the name and the host together">
          {(id) => (
            <TextInput
              id={id}
              list="account-hosts"
              value={draft.ref.host}
              placeholder="%"
              onChange={(e) => set({ ref: { ...draft.ref, host: e.target.value } })}
            />
          )}
        </Field>
      ) : null}
      <datalist id="account-hosts">
        {options.hosts.map((host) => (
          <option key={host} value={host} />
        ))}
      </datalist>

      <Field
        inline
        label="Password"
        hint={
          draft.original === null
            ? 'sent to the server and kept nowhere else'
            : 'blank leaves the password alone'
        }
      >
        {(id) => (
          <PasswordInput
            id={id}
            value={draft.password}
            onChange={(e) => set({ password: e.target.value })}
          />
        )}
      </Field>

      {options.attributes.map((attribute) => (
        <AttributeField
          key={attribute.name}
          attribute={attribute}
          value={draft.attributes[attribute.name] ?? ''}
          onChange={(value) => setAttribute(attribute.name, value)}
        />
      ))}
    </div>
  )
}

/** One attribute, drawn as the kind of box the driver said it was. Nothing here knows the clause. */
function AttributeField({
  attribute,
  value,
  onChange,
}: {
  attribute: AccountAttribute
  value: string
  onChange: (value: string) => void
}) {
  return (
    <Field inline label={attribute.label} hint={attribute.help}>
      {(id) =>
        attribute.kind === 'check' ? (
          <input
            id={id}
            type="checkbox"
            className="accent-accent"
            checked={value === 'true'}
            onChange={(e) => onChange(String(e.target.checked))}
          />
        ) : attribute.kind === 'choice' && attribute.choices.length > 0 ? (
          <SelectInput id={id} value={value} onChange={(e) => onChange(e.target.value)}>
            <option value="">(unchanged)</option>
            {attribute.choices.map((choice) => (
              <option key={choice} value={choice}>
                {choice}
              </option>
            ))}
          </SelectInput>
        ) : (
          <TextInput
            id={id}
            inputMode={attribute.kind === 'number' ? 'numeric' : undefined}
            value={value}
            onChange={(e) => onChange(e.target.value)}
          />
        )
      }
    </Field>
  )
}

/**
 * The privilege matrix: one row per privilege of the chosen level, and two ticks — whether it is
 * granted, and whether it may be passed on. It is a `ResultGrid` with `check` columns, like every
 * other grid of ticks in this app.
 *
 * The scope above it is what the grant is *on*, and it is only drawn as deep as the level goes: a
 * server privilege has no scope at all, and a table one has a database, a schema and a name.
 */
function PrivilegePane({
  draft,
  options,
  onChange,
}: {
  draft: AccountDraft
  options: AccountOptions
  onChange: (draft: AccountDraft) => void
}) {
  const levels = options.privileges.map((set) => set.level)
  const [level, setLevel] = useState<GrantLevel>(levels[0] ?? 'server')
  const [database, setDatabase] = useState(options.databases[0] ?? '')
  const [schema, setSchema] = useState('')
  const [object, setObject] = useState('')

  const names = options.privileges.find((set) => set.level === level)?.names ?? []
  const scope = { database, schema, object }

  const rows: Cell[][] = names.map((privilege) => {
    const grant = grantAt(level, scope, privilege)
    const drawn = held(draft, grant)
    return [privilege, drawn ? 'on' : null, drawn?.grantable ? 'on' : null]
  })

  const data: GridData = {
    columns: [
      { name: 'Privilege', type: 'text', fullType: 'text', readOnly: true, width: 260 },
      { name: 'Granted', type: 'boolean', fullType: 'boolean', editor: { kind: 'check' }, width: 90 },
      {
        name: 'With Grant Option',
        type: 'boolean',
        fullType: 'boolean',
        editor: { kind: 'check' },
        width: 150,
      },
    ],
    rows,
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-line px-2 py-1.5">
        <SelectInput
          className="h-6 w-auto text-xs"
          value={level}
          onChange={(e) => setLevel(e.target.value as GrantLevel)}
        >
          {levels.map((one) => (
            <option key={one} value={one}>
              {LEVEL_LABEL[one]}
            </option>
          ))}
        </SelectInput>

        {level !== 'server' && options.databases.length > 0 ? (
          <SelectInput
            className="h-6 w-auto text-xs"
            value={database}
            onChange={(e) => setDatabase(e.target.value)}
          >
            {options.databases.map((one) => (
              <option key={one} value={one}>
                {one}
              </option>
            ))}
          </SelectInput>
        ) : null}

        {level === 'schema' || level === 'table' || level === 'column' ? (
          <TextInput
            className="h-6 w-40 text-xs"
            placeholder="schema"
            value={schema}
            onChange={(e) => setSchema(e.target.value)}
          />
        ) : null}

        {level === 'table' || level === 'column' || level === 'routine' ? (
          <TextInput
            className="h-6 w-52 text-xs"
            placeholder="table"
            value={object}
            onChange={(e) => setObject(e.target.value)}
          />
        ) : null}
      </div>

      <div className="min-h-0 flex-1">
        <ResultGrid
          result={data}
          onEditCell={(ref, value) => {
            const privilege = names[ref.row]
            if (privilege === undefined) return
            const grant: Grant = grantAt(level, scope, privilege)
            const on = value !== null && value !== ''
            onChange(
              ref.column === 1
                ? toggleGrant(draft, grant, on)
                : toggleGrantable(draft, grant, on),
            )
          }}
        />
      </div>
    </div>
  )
}

/** One row per role this server offers, and one tick: whether the account is a member. */
function RolePane({
  draft,
  options,
  onChange,
}: {
  draft: AccountDraft
  options: AccountOptions
  onChange: (draft: AccountDraft) => void
}) {
  const data: GridData = {
    columns: [
      { name: 'Role', type: 'text', fullType: 'text', readOnly: true, width: 320 },
      { name: 'Member', type: 'boolean', fullType: 'boolean', editor: { kind: 'check' }, width: 90 },
    ],
    rows: options.roles.map((role) => [role, draft.roles.includes(role) ? 'on' : null]),
  }

  return (
    <ResultGrid
      result={data}
      onEditCell={(ref, value) => {
        const role = options.roles[ref.row]
        if (role === undefined) return
        onChange(toggleRole(draft, role, value !== null && value !== ''))
      }}
    />
  )
}

/**
 * Exactly what Apply will run, and what the server itself says about the account below it.
 *
 * **A password is never here.** The statements carry `<password>` where the literal will go: this
 * is what goes on the screen, into the History Log and into a saved history, and the driver is what
 * spells it that way — see `docs/features/accounts.md`.
 */
function ChangesPane({ statements, source }: { statements: string[]; source: string[] }) {
  return (
    <div className="space-y-4 p-3">
      <section>
        <h3 className="mb-1.5 text-2xs font-medium tracking-wide text-muted uppercase">
          Statements
        </h3>
        {statements.length === 0 ? (
          <p className="text-xs text-faint">Nothing has changed yet.</p>
        ) : (
          <pre className="selectable rounded-md border border-line bg-bar p-2 font-mono text-xs whitespace-pre-wrap text-fg">
            {statements.join('\n')}
          </pre>
        )}
      </section>

      {source.length > 0 ? (
        <section>
          <h3 className="mb-1.5 text-2xs font-medium tracking-wide text-muted uppercase">
            What the server says
          </h3>
          <pre className="selectable rounded-md border border-line bg-bar p-2 font-mono text-xs whitespace-pre-wrap text-muted">
            {source.join('\n')}
          </pre>
        </section>
      ) : null}
    </div>
  )
}

export { accountLabel, type AccountDetail }
