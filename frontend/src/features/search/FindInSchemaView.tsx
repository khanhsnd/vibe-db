import { useMemo, useState } from 'react'
import { TriangleAlert } from 'lucide-react'

import { engineInfo, engineWords, namespaceOf, type Namespace } from '@/bridge/connections'
import {
  isPatternMode,
  SEARCH_MODES,
  SEARCH_SCOPES,
  type SearchHit,
  type SearchMode,
  type SearchResult,
  type SearchScope,
  type SearchSkip,
} from '@/bridge/search'
import type { Cell } from '@/bridge/tables'
import { nodeGlyph } from '@/features/connections/ObjectTree'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { GridOutline, ResultColumn, Selection } from '@/features/results/resultSet'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { Checkbox, Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { Loading } from '@/shared/ui/Loading'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { databaseKey, useConnectionsStore } from '@/stores/connections'
import { IDLE_SEARCH, useSearchesStore, type SearchRun } from '@/stores/searches'
import { useSettingsStore } from '@/stores/settings'
import { useTabsStore, type FindTab } from '@/stores/tabs'

const numbers = new Intl.NumberFormat('en-US')

/** How wide the form opens, and how far the divider beside it may be dragged. */
const FORM_WIDTH = 340
const FORM_MIN_WIDTH = 260

/** The width the two short lists take, which are answers rather than names and need no more. */
const SHORT_CONTROL = 'w-[190px]'

/**
 * Every row is one object, so the name column wears the tree's own glyph and never a caret: the
 * rows are a flat list and nothing under one opens.
 */
const OBJECT_OUTLINE: GridOutline = {
  depth: () => 0,
  open: () => undefined,
  onToggle: () => {},
  icon: () => nodeGlyph('table', false, 12),
}

const NAME_WIDTH = 260

/**
 * Every table of a namespace searched for one piece of text — what `Find in Schema` opens.
 *
 * Three things about it are worth knowing before reading the code:
 *
 * 1. **The form is the tab.** What to look for, where, and how are fields of `FindTab`, so a reader
 *    who looks at another tab and comes back finds their question where they left it — and the hits
 *    with it, which live in `stores/searches` for the same reason a query tab's rows do.
 * 2. **`Look In` is two different questions.** `Data` is a count of matching rows and is the
 *    server's to answer; `Structure` is a count of matching *names* and is answered from the
 *    catalogue reads the app already makes. One rule matches both — see `domain.TextMatch`.
 * 3. **The answer is a count per object, not the matches.** A table with four thousand matching
 *    rows is not a list anybody scrolls, so a double-click opens the object itself.
 */
export function FindInSchemaView({ tab }: { tab: FindTab }) {
  const edit = useTabsStore((s) => s.editFind)
  const repoint = useTabsStore((s) => s.repoint)
  const openTab = useTabsStore((s) => s.open)
  const openDesign = useTabsStore((s) => s.openDesign)

  const profiles = useConnectionsStore((s) => s.profiles)
  const sessions = useConnectionsStore((s) => s.sessions)
  const databases = useConnectionsStore((s) => s.databases)
  const schemas = useConnectionsStore((s) => s.schemas)
  const activeNamespace = useConnectionsStore((s) => s.activeNamespace)
  const loadSchemas = useConnectionsStore((s) => s.loadSchemas)

  const run = useSearchesStore((s) => s.runs[tab.id] ?? IDLE_SEARCH)
  const find = useSearchesStore((s) => s.find)
  const stop = useSearchesStore((s) => s.stop)

  const session = sessions[tab.profileId]
  const profile = profiles.find((one) => one.id === tab.profileId)
  const info = engineInfo(profile?.engine ?? '')
  const words = engineWords(profile?.engine ?? '')

  // Only an open connection can be searched, so the list is the open ones: a shut connection
  // offered here would be a choice that could answer nothing.
  const open = profiles.filter((one) => sessions[one.id] !== undefined)
  const databaseList = databases[tab.profileId] ?? []
  const schemaList = schemas[databaseKey(tab.profileId, tab.namespace.database)] ?? []

  const point = (profileId: string, namespace: Namespace) => {
    repoint(tab.id, profileId, namespace)
    // The level below a database is read one database at a time — PostgreSQL can only see the
    // schemas of a database something is connected to — so pointing at one is what asks for them.
    if (namespace.database !== '') void loadSchemas(profileId, namespace.database)
  }

  const searchable = session !== undefined && tab.text !== '' && tab.namespace.database !== ''

  /**
   * What two clicks on a hit open. A **data** hit opens a query tab holding the read the count was
   * made through — the engine's own statement, arriving with the hit — so the reader lands on the
   * rows themselves rather than on the whole table and then has to narrow it again by hand. It is
   * not run for them: a statement in an editor is the reader's to send, which is the rule everywhere
   * else in the app.
   *
   * A **structure** hit has no such read — matching names is not one — so it opens the designer,
   * which is where a field, an index and a constraint are read.
   */
  const openHit = (hit: SearchHit) => {
    const target = { profileId: tab.profileId, namespace: tab.namespace, table: hit.table }

    if (hit.statement === '') {
      openDesign(target)
      return
    }

    openTab({
      kind: 'query',
      title: `${hit.table} · Find`,
      profileId: tab.profileId,
      namespace: tab.namespace,
      sql: hit.statement,
      rowLimit: useSettingsStore.getState().settings.queryRows,
    })
  }

  const start = () => {
    if (!session || !searchable) return
    void find(tab.id, session.id, {
      database: tab.namespace.database,
      schema: tab.namespace.schema,
      text: tab.text,
      scope: tab.scope,
      mode: tab.mode,
      caseInsensitive: tab.caseInsensitive,
    })
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <Split
        id="vibe-db.find"
        orientation="horizontal"
        panelIds={['form', 'results']}
        className="min-h-0 flex-1"
      >
        <SplitPanel
          id="form"
          defaultSize={FORM_WIDTH}
          minSize={FORM_MIN_WIDTH}
          maxSize="60%"
          className="min-h-0"
        >
          {/* A form, so Enter in the box searches — which is what Enter in a Find box does
              everywhere, and the reason the button is a submit rather than a click handler. */}
          <form
            onSubmit={(event) => {
              event.preventDefault()
              start()
            }}
            className="scroll-thin flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3"
          >
            <Field label="Connection:">
              {(id) => (
                <SelectInput
                  id={id}
                  value={tab.profileId}
                  disabled={open.length === 0}
                  onChange={(event) => {
                    const next = event.target.value
                    point(
                      next,
                      activeNamespace[next] ??
                        namespaceOf(profiles.find((one) => one.id === next)?.database ?? ''),
                    )
                  }}
                >
                  {open.length === 0 ? <option value="">No open connection</option> : null}
                  {open.map((one) => (
                    <option key={one.id} value={one.id}>
                      {one.name}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            <Field label="Database:">
              {(id) => (
                <SelectInput
                  id={id}
                  value={tab.namespace.database}
                  disabled={databaseList.length === 0}
                  onChange={(event) => point(tab.profileId, namespaceOf(event.target.value))}
                  className="font-mono"
                >
                  <option value="">
                    {databaseList.length === 0 ? 'Not connected' : 'No database'}
                  </option>
                  {databaseList.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </SelectInput>
              )}
            </Field>

            {/* An engine whose database *is* its schema has no second level, so there is nothing
                to draw — the same rule the tree and the tab strips follow. */}
            {info.hasSchemas ? (
              <Field label="Schema:">
                {(id) => (
                  <SelectInput
                    id={id}
                    value={tab.namespace.schema}
                    disabled={schemaList.length === 0}
                    onChange={(event) =>
                      point(tab.profileId, namespaceOf(tab.namespace.database, event.target.value))
                    }
                    className="font-mono"
                  >
                    <option value="">
                      {tab.namespace.database === '' ? 'No database' : 'No schema'}
                    </option>
                    {schemaList.map((name) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </SelectInput>
                )}
              </Field>
            ) : null}

            {/* What is searched for sits apart from where it is searched: the three above are the
                place, and everything below is the question. */}
            <div className="mt-2 flex flex-col gap-3">
              <Field label="Find what:">
                {(id) => (
                  <TextInput
                    id={id}
                    autoFocus
                    value={tab.text}
                    onChange={(event) => edit(tab.id, { text: event.target.value })}
                  />
                )}
              </Field>

              <div className="flex items-start justify-between gap-3">
                <Field label="Look In:">
                  {(id) => (
                    <SelectInput
                      id={id}
                      value={tab.scope}
                      onChange={(event) =>
                        edit(tab.id, { scope: event.target.value as SearchScope })
                      }
                      className={SHORT_CONTROL}
                    >
                      {SEARCH_SCOPES.map((one) => (
                        <option key={one.value} value={one.value}>
                          {one.label}
                        </option>
                      ))}
                    </SelectInput>
                  )}
                </Field>

                {/* Said where it is true and nowhere else: `%` and `_` are the reader's own in the
                    two pattern modes, and `\` is what makes one the character itself. */}
                {isPatternMode(tab.mode) ? (
                  <span className="font-mono text-xs text-faint">
                    {"'\\' as escape character"}
                  </span>
                ) : null}
              </div>

              <Field label="Search Mode:">
                {(id) => (
                  <SelectInput
                    id={id}
                    value={tab.mode}
                    onChange={(event) => edit(tab.id, { mode: event.target.value as SearchMode })}
                    className={SHORT_CONTROL}
                  >
                    {SEARCH_MODES.map((one) => (
                      <option key={one.value} value={one.value}>
                        {one.label}
                      </option>
                    ))}
                  </SelectInput>
                )}
              </Field>

              <Checkbox
                label="Case Insensitive"
                checked={tab.caseInsensitive}
                onChange={(event) => edit(tab.id, { caseInsensitive: event.target.checked })}
              />
            </div>

            <div className="flex justify-end pt-1">
              {/* One button, two states. A search over a whole schema is as long as the schema is,
                  so stopping it is the honest bound on it — see `app.searchTimeout`. */}
              {run.running ? (
                <Button type="button" variant="default" onClick={() => stop(tab.id)}>
                  Stop
                </Button>
              ) : (
                <Button type="submit" variant="primary" disabled={!searchable}>
                  Find
                </Button>
              )}
            </div>
          </form>
        </SplitPanel>

        <SplitHandle orientation="horizontal" />

        <SplitPanel id="results" minSize="30%" className="min-h-0">
          <Results
            run={run}
            object={words.table.toLowerCase()}
            objects={words.tables.toLowerCase()}
            onOpen={openHit}
          />
        </SplitPanel>
      </Split>

      <div className="flex h-6 shrink-0 items-center gap-2 border-t border-line bg-panel px-2 text-xs text-muted">
        <span>{summary(run.result, run.running)}</span>
        {run.result !== null && !run.running ? (
          <span className="font-mono text-2xs text-faint">
            {numbers.format(run.result.searched)} objects searched
          </span>
        ) : null}
      </div>
    </div>
  )
}

interface ResultsProps {
  run: SearchRun
  /** What this engine calls one of the things being searched, and several of them. */
  object: string
  objects: string
  onOpen: (hit: SearchHit) => void
}

/**
 * What the search found: one row per object, its name and how many of its rows or names matched.
 * A `ResultGrid` like everything else in the app that shows rows and columns — see docs/grid.md.
 * Its own head strip is off, this pane already carrying one that says what was looked for.
 */
function Results({ run, object, objects, onOpen }: ResultsProps) {
  const [selection, setSelection] = useState<Selection | null>(null)
  const hits = run.result?.hits ?? []

  const data = useMemo(
    () => ({
      columns: RESULT_COLUMNS,
      rows: hits.map((hit): Cell[] => [hit.table, String(hit.matches)]),
    }),
    [hits],
  )

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-6 shrink-0 items-center border-b border-line bg-bar px-2 text-xs text-muted">
        {run.request === null ? 'Find results:' : `Find results for “${run.request.text}”:`}
      </div>

      <div className="min-h-0 flex-1">
        {run.running && hits.length === 0 ? (
          <Loading label={`Searching every ${object}…`} />
        ) : run.error !== null ? (
          <Empty title={run.error.message} hint={run.error.detail} />
        ) : run.result === null ? (
          <Empty
            title="Nothing searched yet"
            hint="Type what to look for and press Find. Data counts matching rows; Structure counts matching names."
          />
        ) : hits.length === 0 ? (
          <Empty
            title={`No ${object} holds that`}
            hint={`${numbers.format(run.result.searched)} ${objects} were read and none of them matched.`}
          />
        ) : (
          <ResultGrid
            result={data}
            headStrip={false}
            selection={selection}
            onSelectionChange={setSelection}
            onActivateRow={(row) => {
              const hit = hits[row]
              if (hit) onOpen(hit)
            }}
          />
        )}
      </div>

      {/* An object the search could not read must never read as one that held nothing. */}
      {(run.result?.skipped ?? []).length > 0 ? (
        <div className="shrink-0 px-2 pb-2">
          <Banner
            tone="warn"
            icon={<TriangleAlert size={13} />}
            text={skippedText(run.result?.skipped ?? [])}
          />
        </div>
      ) : null}
    </div>
  )
}

const RESULT_COLUMNS: ResultColumn[] = [
  {
    name: 'Name',
    type: 'text',
    fullType: 'text',
    readOnly: true,
    width: NAME_WIDTH,
    outline: OBJECT_OUTLINE,
  },
  // A count, so the grid draws it as one: right-aligned, in the numeric column's own colour.
  { name: 'Number of Matched Records', type: 'bigint', fullType: 'bigint', readOnly: true },
]

function summary(result: SearchResult | null, running: boolean): string {
  if (running) return 'Searching…'
  if (result === null) return 'Ready'
  return `Found ${numbers.format(result.hits.length)} objects with matched records`
}

function skippedText(skipped: readonly SearchSkip[]): string {
  const named = skipped
    .slice(0, 3)
    .map((one) => `${one.table} (${one.error})`)
    .join('; ')
  const rest = skipped.length > 3 ? `, and ${skipped.length - 3} more` : ''
  return `${skipped.length} could not be read and were not searched — ${named}${rest}.`
}
