import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, Save } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { NEW_SOURCE, schema, type SourceRef } from '@/bridge/schema'
import { NO_DIAGNOSTICS } from '@/features/editor/diagnostics'
import { EditorPane } from '@/features/editor/EditorPane'
import { ResultGrid } from '@/features/results/ResultGrid'
import { ResultTabs } from '@/features/results/ResultTabs'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { namespaceKey, useConnectionsStore } from '@/stores/connections'
import { SOURCE_NOUN, useTabsStore } from '@/stores/tabs'

import { changeRows } from './designerRows'

interface SourceEditorProps {
  /** The tab this editor fills, which its own Save turns from a new object into that object. */
  tabId: string
  sessionId: string
  profileId: string
  namespace: Namespace
  /** Which view or routine is being edited. */
  source: SourceRef
  /** Whether it is being declared rather than altered — see `SourceTab.creating`. */
  creating: boolean
}

const NO_SCHEMA: never[] = []

/**
 * A view's or a routine's definition, edited as the statement it is. It is `TableDesigner`'s
 * counterpart for the two objects a grid of fields could never describe: a view is a query and a
 * routine is a program.
 *
 * **The buffer is the whole `CREATE` statement**, as the server answered with it, and it travels
 * back verbatim. That is what makes a routine body work with no `DELIMITER` anywhere: a definition
 * is *one* statement however many semicolons are inside it, and nothing on this side ever reads
 * that text — where a statement ends is the driver's, and this never asks.
 *
 * **Save replaces by dropping first.** There is no transaction around DDL, so what protects the
 * reader is seeing it: the Changes pane is `PreviewSource`, which is the very list `SaveSource`
 * runs, and it says `DROP …` in as many words before anything happens.
 */
export function SourceEditor({
  tabId,
  sessionId,
  profileId,
  namespace,
  source,
  creating,
}: SourceEditorProps) {
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === profileId)?.engine ?? '',
  )
  // The same table names the query editor completes against, read the same way and cached until
  // disconnect: a view's body is a query, and a query is what completion is for.
  const tableSchemas = useConnectionsStore(
    (s) => s.tableSchemas[namespaceKey(profileId, namespace)] ?? NO_SCHEMA,
  )
  const loadTableSchemas = useConnectionsStore((s) => s.loadTableSchemas)
  const changeSchema = useConnectionsStore((s) => s.changeSchema)
  const declared = useTabsStore((s) => s.declared)

  const [definition, setDefinition] = useState<string | null>(null)
  const [statements, setStatements] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    void loadTableSchemas(profileId, namespace)
  }, [loadTableSchemas, profileId, namespace])

  // A new object opens on the engine's own skeleton and an existing one on the server's own words.
  // Both are a *read*, and neither is spelled here — see `driver.Schema.NewSource` and `Source`.
  //
  // A Save turns `creating` off, so this runs again and the buffer becomes the server's own
  // spelling of what was just declared. That is the same thing the table designer does after its
  // Save, and for the same reason: what the reader is looking at is what is there now.
  const load = useCallback(async () => {
    setError(null)
    try {
      const text = creating
        ? await schema.newSource(sessionId, namespace, source.kind, source.name)
        : (await schema.loadSource(sessionId, namespace, source)).definition
      setDefinition(text)
    } catch (thrown) {
      setError(toAppError(thrown))
      setDefinition('')
    }
  }, [creating, sessionId, namespace, source])

  useEffect(() => {
    void load()
  }, [load])

  // The preview follows the buffer, so what the Changes pane says is always what a Save would run
  // — the drop in front of it included, which is the whole reason that pane is there.
  useEffect(() => {
    if (definition === null) return

    let current = true
    schema
      .previewSource(sessionId, namespace, creating ? NEW_SOURCE : source, definition)
      .then((planned) => {
        if (current) setStatements(planned)
      })
      .catch(() => {
        if (current) setStatements([])
      })

    return () => {
      current = false
    }
  }, [sessionId, namespace, source, creating, definition])

  if (definition === null) {
    return <Empty title="Reading the definition…" hint={qualified(namespace, source.name)} />
  }

  const noun = SOURCE_NOUN[source.kind]
  const dirty = definition.trim() !== ''

  const save = async () => {
    setSaving(true)
    setError(null)
    try {
      await changeSchema(profileId, { namespace }, (id) =>
        schema.saveSource(id, namespace, creating ? NEW_SOURCE : source, definition),
      )
      // The object exists now, so the next Save replaces it rather than declaring it a second
      // time and being refused for a name that is taken.
      if (creating) declared(tabId)
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2">
        {/* Save is the weight of the toolbar the moment there is anything to save, which here is
            anything at all: an empty buffer would leave the drop standing on its own. */}
        <Button
          variant={dirty ? 'primary' : 'subtle'}
          icon={<Save size={13} />}
          disabled={saving || !dirty}
          onClick={() => void save()}
        >
          {saving ? 'Saving…' : creating ? `Create ${noun}` : 'Save'}
        </Button>
        <Button
          variant="subtle"
          icon={<RefreshCw size={13} />}
          disabled={saving}
          onClick={() => void load()}
        >
          {creating ? 'Reset' : 'Refresh'}
        </Button>

        <div className="flex-1" />
        <span className="truncate rounded-md border border-line bg-canvas px-1.5 py-0.5 font-mono text-2xs text-faint">
          {qualified(namespace, source.name)}
        </span>
      </div>

      {error ? (
        <div className="shrink-0 px-2 pt-2">
          <Banner tone="danger" text={error.message} />
        </div>
      ) : null}

      <div className="min-h-0 flex-1">
        <ResultTabs
          panes={[
            {
              id: 'definition',
              label: 'Definition',
              content: (
                <EditorPane
                  value={definition}
                  schema={tableSchemas}
                  engine={engine}
                  review={NO_DIAGNOSTICS}
                  onChange={setDefinition}
                  onSelectionChange={noSelection}
                />
              ),
            },
            {
              id: 'changes',
              label: `Changes (${statements.length})`,
              content: <ResultGrid result={changeRows(statements)} />,
            },
          ]}
        />
      </div>
    </div>
  )
}

// Nothing here acts on a selection — a definition runs whole or not at all, so there is no
// "run what is selected" for this editor to have.
const noSelection = () => {}

const qualified = (ns: Namespace, name: string) =>
  `${ns.schema === '' ? ns.database : `${ns.database}.${ns.schema}`}.${name}`
