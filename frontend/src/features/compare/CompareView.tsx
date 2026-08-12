import { useCallback, useMemo, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  ChevronsDownUp,
  ChevronsUpDown,
  Columns2,
  Rows2,
  TriangleAlert,
} from 'lucide-react'

import {
  COMPARE_ROW_PRESETS,
  DEFAULT_COMPARE_ROWS,
  type DataDiff,
  type StructureDiff,
} from '@/bridge/compare'
import { engineInfo } from '@/bridge/connections'
import { nodeGlyph } from '@/features/connections/ObjectTree'
import { ResultTabs } from '@/features/results/ResultTabs'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { Segmented } from '@/shared/ui/Segmented'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { targetLabel, useTabsStore, type CompareMode, type CompareTab } from '@/stores/tabs'
import { useUIStore } from '@/stores/ui'

import { ComparePanes, type CompareLayout } from './ComparePanes'
import { CompareScript } from './CompareScript'
import { CompareSummary, type CompareWords } from './CompareSummary'
import { DdlDiff } from './DdlDiff'
import { FieldFilterButton } from './FieldFilterButton'
import { filterCount, shownAt, type FieldFilters } from './fieldFilter'
import { endsOf, everyPick, rowPicks, tableInto, type Direction } from './migrate'
import { allSame, countStatuses, dataPanes, structurePanes, type DiffCounts } from './panes'
import { BRANCHES, structureRows, type SideNames, type StructureRow } from './structureTree'
import { useCompare } from './useCompare'
import { useMigration } from './useMigration'

const numbers = new Intl.NumberFormat('en-US')

const MODES: ReadonlyArray<{ id: CompareMode; label: string }> = [
  { id: 'structure', label: 'Structure' },
  { id: 'data', label: 'Data' },
]

const LAYOUTS: ReadonlyArray<{ id: CompareLayout; label: string; icon: typeof Columns2 }> = [
  { id: 'columns', label: 'Side by side', icon: Columns2 },
  { id: 'rows', label: 'Stacked', icon: Rows2 },
]

const SHAPE_WORDS: CompareWords = { onlyLeft: 'only left', onlyRight: 'only right' }

/** How tall the script sits under the panes, and how far the divider between them may be dragged. */
const SCRIPT_HEIGHT = 180
const SCRIPT_MIN_HEIGHT = 80

/**
 * Two tables read side by side, and made to agree — what Compare opens.
 *
 * Three rules hold the whole surface up:
 *
 * 1. **Row *n* of one pane is row *n* of the other.** The pairing is the Go side's
 *    (`internal/compare`), a blank stands where a side has not got the row, and the two panes
 *    scroll as one. The shape half needs no second pane at all: one row carries both declarations.
 * 2. **A tick is what is taken.** Every row that differs can be picked, in either direction, and
 *    what is picked is the whole of what Apply touches.
 * 3. **The script is asked for, never spelled here.** What the reader reads under the panes is the
 *    same list Apply is handed, spelled by the driver — so the two cannot be different things.
 */
export function CompareView({ tab }: { tab: CompareTab }) {
  const setMode = useTabsStore((s) => s.setCompareMode)
  const layout = useUIStore((s) => s.compareLayout)
  const setLayout = useUIStore((s) => s.setCompareLayout)
  const profiles = useConnectionsStore((s) => s.profiles)

  const [differencesOnly, setDifferencesOnly] = useState(false)
  /** What is looked for in each compared field, which narrows the rows the panes draw. */
  const [filters, setFilters] = useState<FieldFilters>({})
  const [rows, setRows] = useState(DEFAULT_COMPARE_ROWS)
  const [direction, setDirection] = useState<Direction>('toTarget')
  const [taken, setTaken] = useState<ReadonlySet<string>>(new Set())
  /** The branches the reader has folded shut. Every one is open to begin with. */
  const [shut, setShut] = useState<ReadonlySet<string>>(new Set())

  const left = useMemo(
    () => ({ profileId: tab.profileId, namespace: tab.namespace, table: tab.table }),
    [tab.profileId, tab.namespace, tab.table],
  )
  const right = tab.right

  const comparison = useCompare(left, right, tab.mode, rows)
  const { structure, data, loading, error, connected } = comparison

  // A fresh read is a fresh set of things to pick, so nothing a tick named survives one: the pick
  // is a position in *this* comparison, and the tables may have moved under it.
  const refresh = useCallback(() => {
    setTaken(new Set())
    comparison.refresh()
  }, [comparison])

  // One tick, however many things it stands for: a folder's takes its whole section, and untakes
  // it again once every one of them is already taken.
  const take = useCallback((picks: string[]) => {
    if (picks.length === 0) return

    setTaken((current) => {
      const next = new Set(current)
      const held = picks.every((pick) => next.has(pick))
      for (const pick of picks) {
        if (held) next.delete(pick)
        else next.add(pick)
      }
      return next
    })
  }, [])

  const setFilter = useCallback((column: string, text: string) => {
    setFilters((current) => ({ ...current, [column]: text }))
  }, [])

  const fold = useCallback((branch: string) => {
    setShut((current) => {
      const next = new Set(current)
      if (!next.delete(branch)) next.add(branch)
      return next
    })
  }, [])

  const ends = endsOf(direction)
  const into = tableInto(direction, left, right)
  const migration = useMigration(comparison, ends, taken, into, direction)

  const profileOf = (profileId: string) => profiles.find((one) => one.id === profileId)
  const names: SideNames = { left: targetLabel(left), right: targetLabel(right) }
  // Which of the two a pane is, drawn over it. The tab is read left to right — the origin is what
  // the reader has, the target is what they are holding it against — and neither pane can say so
  // for itself.
  const labels: SideNames = { left: `Origin: ${names.left}`, right: `Target: ${names.right}` }

  // A constraint neither engine has is a section that could only ever be empty — the same rule
  // the designer follows when it draws two panes instead of three.
  const referential =
    engineInfo(profileOf(left.profileId)?.engine ?? '').hasForeignKeys &&
    engineInfo(profileOf(right.profileId)?.engine ?? '').hasForeignKeys

  // Which rows are drawn is worked out here rather than in the half below, because `Take all` on the
  // toolbar takes what is drawn: a tick has to be a row the reader can see.
  const narrowed = filterCount(filters)
  const at = data === null ? [] : shownAt(data.rows, data.columns, differencesOnly, filters)
  const picks = structure ? everyPick(structure) : data ? rowPicks(data.rows, at) : []
  const setAll = (all: boolean) => setTaken(all ? new Set(picks) : new Set())

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-2 border-b border-line bg-bar px-1.5"
      >
        <Segmented
          label="What is compared"
          options={MODES}
          value={tab.mode}
          onChange={(mode) => {
            setTaken(new Set())
            // A field filter names the columns of *these* rows, and the other half has none.
            setFilters({})
            setMode(tab.id, mode)
          }}
        />
        {/* Only rows can be stacked. Two trees one above the other cannot be read across, and the
            ticks between them would have nowhere to be. */}
        {tab.mode === 'data' ? (
          <Segmented
            label="How the panes sit"
            options={LAYOUTS}
            value={layout}
            onChange={setLayout}
            iconOnly
          />
        ) : null}

        {/* Only a tree has anything to open. Both panes draw the same rows, so one press opens or
            shuts the pair — there is no per-pane fold to get out of step. */}
        {tab.mode === 'structure' ? (
          <>
            <Button
              variant="subtle"
              icon={<ChevronsUpDown size={13} />}
              title="Expand all"
              aria-label="Expand all"
              onClick={() => setShut(new Set())}
            />
            <Button
              variant="subtle"
              icon={<ChevronsDownUp size={13} />}
              title="Collapse all"
              aria-label="Collapse all"
              onClick={() => setShut(new Set(BRANCHES))}
            />
          </>
        ) : null}

        <div className="h-4 w-px bg-line" />

        {/* Which table changes. It is the one control on this bar that decides what Apply *does*,
            so it names both tables rather than saying only which way the arrow points. */}
        <Segmented
          label="Which way the migration runs"
          options={[
            { id: 'toTarget' as Direction, label: names.right, icon: ArrowRight },
            { id: 'toOrigin' as Direction, label: names.left, icon: ArrowLeft },
          ]}
          value={direction}
          onChange={(next) => setDirection(next)}
          prefix="Apply to"
        />

        <div className="h-4 w-px bg-line" />

        <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted">
          <input
            type="checkbox"
            checked={differencesOnly}
            onChange={(e) => setDifferencesOnly(e.target.checked)}
            className="h-3.5 w-3.5 accent-[var(--accent)]"
          />
          Differences only
        </label>

        {/* A box per compared field, which is the other half of the same question: `Differences
            only` narrows by *what a row is*, and this by *what it holds*. The two compose, and
            neither asks the server anything — see `fieldFilter`. */}
        {tab.mode === 'data' && data !== null ? (
          <FieldFilterButton
            columns={data.columns}
            filters={filters}
            count={narrowed}
            shown={at.length}
            total={data.rows.length}
            onChange={setFilter}
            onClear={() => setFilters({})}
          />
        ) : null}

        {/* Only the data half reads rows. It is a list of counts and **not** the grids' own
            `RowLimit`, which offers All: a comparison holds both sides in memory at once and
            crosses the bridge as one answer, so there is no unbounded read to offer here — see
            `app.DefaultCompareRows`. What it did not reach is said under the panes instead. */}
        {tab.mode === 'data' ? (
          <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted">
            Rows
            <select
              value={rows}
              onChange={(e) => setRows(Number(e.target.value))}
              className={`${TOOLBAR_CONTROL} font-mono`}
            >
              {COMPARE_ROW_PRESETS.map((preset) => (
                <option key={preset} value={preset}>
                  {numbers.format(preset)}
                </option>
              ))}
            </select>
          </label>
        ) : null}

        <Button variant="subtle" icon={ICON.refresh} disabled={loading} onClick={refresh}>
          Refresh
        </Button>

        <div className="min-w-2 flex-1" />

        <Button variant="subtle" disabled={picks.length === 0} onClick={() => setAll(true)}>
          Take all
        </Button>
        <Button variant="subtle" disabled={taken.size === 0} onClick={() => setAll(false)}>
          Take none
        </Button>
        <Button
          variant="primary"
          icon={ICON.migrate}
          disabled={migration.statements.length === 0 || migration.applying}
          onClick={() => void migration.apply().then(refresh)}
        >
          {migration.applying ? 'Applying…' : `Apply to ${into.table}`}
        </Button>
      </div>

      {migration.error !== null ? (
        <div className="shrink-0 px-2 pt-2">
          <Banner tone="danger" text={migration.error.message} />
        </div>
      ) : null}

      <div className="min-h-0 flex-1">
        {!connected ? (
          <Empty
            title="Not connected"
            hint="Both sides of a comparison have to be open. Reconnect them and press Refresh."
          />
        ) : loading ? (
          <Loading
            label={tab.mode === 'structure' ? 'Reading both shapes…' : 'Reading both tables…'}
          />
        ) : error !== null ? (
          <Empty title={error.message} hint={error.detail} />
        ) : (
          <Split
            id="vibe-db.compare.script"
            orientation="vertical"
            panelIds={['diff', 'script']}
            className="min-h-0 flex-1"
          >
            <SplitPanel id="diff" minSize="20%" className="min-h-0">
              {structure !== null ? (
                <StructureHalf
                  diff={structure}
                  differencesOnly={differencesOnly}
                  referential={referential}
                  names={names}
                  labels={labels}
                  taken={taken}
                  shut={shut}
                  onTake={take}
                  onFold={fold}
                />
              ) : data !== null ? (
                <DataHalf
                  diff={data}
                  at={at}
                  filtered={narrowed > 0}
                  names={names}
                  labels={labels}
                  taken={taken}
                  onTake={take}
                  layout={layout}
                />
              ) : null}
            </SplitPanel>

            <SplitHandle orientation="vertical" />

            <SplitPanel
              id="script"
              defaultSize={SCRIPT_HEIGHT}
              minSize={SCRIPT_MIN_HEIGHT}
              maxSize="70%"
              className="min-h-0"
            >
              <ResultTabs
                panes={[
                  {
                    id: 'script',
                    label: `Script (${migration.statements.length})`,
                    content: (
                      <CompareScript
                        statements={migration.statements}
                        into={targetLabel(into)}
                        empty={
                          picks.length > 0
                            ? 'Tick what to take above, or press Take all.'
                            : narrowed > 0
                              ? 'Nothing drawn differs — the field filters may be hiding what does.'
                              : 'The two sides agree, so there is nothing to carry across.'
                        }
                      />
                    ),
                  },
                  {
                    id: 'ddl',
                    label: 'DDL Diff',
                    content: (
                      <DdlDiff
                        left={migration.ddl.left}
                        right={migration.ddl.right}
                        leftLabel={names.left}
                        rightLabel={names.right}
                        loading={migration.ddl.loading}
                      />
                    ),
                  },
                ]}
              />
            </SplitPanel>
          </Split>
        )}
      </div>
    </div>
  )
}

/**
 * The shape half: **the same tree drawn twice**, one side per pane, with the ticks down the
 * boundary between them. The table is the root, `Fields`, `Indexes` and `Foreign Keys` are foldable
 * sections of it rather than three tab strips, and each row under one names a thing and carries
 * what that side declares it as.
 *
 * Both panes are built from **one** list of rows, so row *n* of one is row *n* of the other, a
 * blank stands where a side has not got the thing, and a caret folds the pair rather than one of
 * them. A section's own tick takes **everything under it** and the root's takes the lot, which is
 * the same control as a row's and not a fourth kind of button.
 */
function StructureHalf({
  diff,
  differencesOnly,
  referential,
  names,
  labels,
  taken,
  shut,
  onTake,
  onFold,
}: {
  diff: StructureDiff
  differencesOnly: boolean
  referential: boolean
  names: SideNames
  labels: SideNames
  taken: ReadonlySet<string>
  shut: ReadonlySet<string>
  onTake: (picks: string[]) => void
  onFold: (branch: string) => void
}) {
  const rows = structureRows(diff, { names, shut, differencesOnly, referential })
  const counts = countStatuses([
    ...diff.fields.map((one) => one.status),
    ...diff.indexes.map((one) => one.status),
    ...(referential ? diff.foreignKeys.map((one) => one.status) : []),
  ])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ComparePanes
        panes={structurePanes(rows, taken, onFold, glyphOf)}
        layout="columns"
        leftLabel={labels.left}
        rightLabel={labels.right}
        onTake={onTake}
      />

      <div className="flex h-6 shrink-0 items-center border-t border-line bg-panel px-2">
        <CompareSummary counts={counts} words={SHAPE_WORDS} />
      </div>
    </div>
  )
}

/**
 * What a row is drawn as. It is the **sidebar tree's own** glyph and colour, so a field, an index
 * and a foreign key look here exactly as they do there — see `nodeGlyph`.
 */
function glyphOf(row: StructureRow) {
  return nodeGlyph(row.kind, row.open === true, 12)
}

/**
 * The data half: one pane pair of rows, and under it what the comparison was *able* to do — which
 * columns it matched on, which it left out, and which side it did not see the whole of. All three
 * are the difference between a diff and a diff that can be trusted, so none of them is optional.
 *
 * Which pairs are drawn is the toolbar's — `at` — because the same answer is what `Take all` takes.
 */
function DataHalf({
  diff,
  at,
  filtered,
  names,
  labels,
  taken,
  onTake,
  layout,
}: {
  diff: DataDiff
  /** Where each drawn pair sits in the whole comparison, which is what a tick is named by. */
  at: readonly number[]
  /** Whether a field filter is in force — why two empty panes need not mean two equal tables. */
  filtered: boolean
  names: SideNames
  labels: SideNames
  taken: ReadonlySet<string>
  onTake: (picks: string[]) => void
  layout: CompareLayout
}) {
  const shown = at.map((one) => diff.rows[one]).filter((one) => one !== undefined)
  const counts = countStatuses(diff.rows.map((one) => one.status))
  const words: CompareWords = {
    onlyLeft: `only in ${names.left}`,
    onlyRight: `only in ${names.right}`,
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      {diff.columns.length === 0 ? (
        <Empty
          title="These two share no columns"
          hint="There is nothing to compare row by row. Structure is where two shapes this far apart are read."
        />
      ) : shown.length === 0 ? (
        <Empty {...nothingDrawn(counts, filtered)} />
      ) : (
        <ComparePanes
          panes={dataPanes(diff.columns, shown, at, taken, layout === 'rows')}
          layout={layout}
          leftLabel={labels.left}
          rightLabel={labels.right}
          onTake={onTake}
        />
      )}

      <div className="flex h-6 shrink-0 items-center gap-3 overflow-hidden border-t border-line bg-panel px-2">
        <CompareSummary counts={counts} words={words} />
        <span className="min-w-0 truncate font-mono text-2xs text-faint">{matchedBy(diff)}</span>
      </div>

      {/* A comparison of two windows must never read as a comparison of two tables, and a column
          left out must never read as a column that agreed. */}
      {caveats(diff).map((caveat) => (
        <div key={caveat} className="shrink-0 px-2 pb-2">
          <Banner tone="warn" icon={<TriangleAlert size={13} />} text={caveat} />
        </div>
      ))}
    </div>
  )
}

/**
 * Why the panes are empty. Three different facts, and never the same sentence: a filter that matched
 * nothing must never read as two tables that agreed, which is the one reading a diff cannot afford.
 */
function nothingDrawn(counts: DiffCounts, filtered: boolean): { title: string; hint: string } {
  if (filtered) {
    return {
      title: 'No row holds what the fields ask for',
      hint: 'The panes are narrowed rather than empty — clear a field filter to widen them.',
    }
  }
  if (allSame(counts)) {
    return {
      title: 'Every row read is identical',
      hint: `${numbers.format(counts.same)} rows matched and agreed on every compared column.`,
    }
  }

  return {
    title: 'Neither side returned a row',
    hint: 'Both tables are empty, or neither read returned anything.',
  }
}

function matchedBy(diff: DataDiff): string {
  const on = diff.key.length === 0 ? '(nothing)' : diff.key.join(', ')
  return diff.keyDeclared
    ? `Matched on the primary key: ${on}`
    : `No shared primary key — matched on every compared column: ${on}`
}

/** What the reader has to be told before they trust the counts beside it. */
function caveats(diff: DataDiff): string[] {
  const said: string[] = []

  if (diff.leftTruncated || diff.rightTruncated) {
    const read = numbers.format(Math.max(diff.leftRows, diff.rightRows))
    said.push(
      `Only the first ${read} rows of each side were read, ordered by the key. ` +
        'Anything past that was not compared — raise Rows to reach it.',
    )
  }
  if (!diff.keyDeclared) {
    said.push(
      'Neither side declares a primary key the other shares, so a row is matched on the whole ' +
        'of itself: nothing can be reported as changed, only as present on one side.',
    )
  }
  if (diff.leftOnly.length > 0 || diff.rightOnly.length > 0) {
    const only = [
      ...(diff.leftOnly.length > 0 ? [`left only: ${diff.leftOnly.join(', ')}`] : []),
      ...(diff.rightOnly.length > 0 ? [`right only: ${diff.rightOnly.join(', ')}`] : []),
    ]
    said.push(`Columns one side has not got were not compared — ${only.join('; ')}.`)
  }

  return said
}

