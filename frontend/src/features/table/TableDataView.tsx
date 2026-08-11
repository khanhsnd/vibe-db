import { useMemo, useState } from 'react'

import { engineInfo, type Namespace } from '@/bridge/connections'
import { rowExport, type ExportFormat } from '@/bridge/export'
import { queries } from '@/bridge/query'
import { tables, type Cell, type TablePage } from '@/bridge/tables'
import {
  CellEditor,
  EDITOR_HEIGHT,
  EDITOR_MIN_HEIGHT,
} from '@/features/results/CellEditor'
import { GridFooter } from '@/features/results/GridFooter'
import { GridPager } from '@/features/results/GridPager'
import { GridStatus } from '@/features/results/GridStatus'
import {
  arrange,
  columnOrderKey,
  loadColumnOrder,
  moveName,
  saveColumnOrder,
} from '@/features/results/columnOrder'
import { countConditions, type Group } from '@/features/results/filter'
import { pageCount } from '@/features/results/paging'
import { ResultGrid } from '@/features/results/ResultGrid'
import { CurlDrawer } from '@/features/results/CurlDrawer'
import {
  focusOf,
  oneRange,
  rowRange,
  selectedRows,
  type CellRef,
  type GridOutline,
  type ResultColumn,
  type Selection,
  type SortKey,
} from '@/features/results/resultSet'
import { useGridEdits, type EditSource } from '@/features/results/useGridEdits'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { FetchProgress } from '@/shared/ui/FetchProgress'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { useConnectionsStore } from '@/stores/connections'
import { IDLE_VIEW, useTableViewsStore } from '@/stores/tableViews'

import { ColumnPicker } from './ColumnPicker'
import { KeyEditor, KEY_EDITOR_HEIGHT, KEY_EDITOR_MIN_HEIGHT, type KeyEdit } from './KeyEditor'
import { keyBadge, keyRows, KEY_SEPARATOR, type KeyRow } from './keyTree'
import { keyMenuItems } from './keyMenu'
import {
  cellAt,
  columnAt,
  KEY_COLUMN,
  readSearch,
  TTL_COLUMN,
  TYPE_COLUMN,
  VALUE_COLUMN,
  writeSearch,
  type KeySearch,
} from './keySpace'
import { sampledTotal } from './keyValue'
import { useWholeValue } from './useWholeValue'
import { TableToolbar, type PanelKind } from './TableToolbar'
import { useTableData } from './useTableData'

// What the column list opens at, and how far it may be dragged. Only the size it opens at is a look —
// the bounds are what stop a drag leaving a panel too small to read or the grid with nothing left.
// The cell editor's own three are `CellEditor`'s, being the same panel on two surfaces.
const PICKER_WIDTH = 168
const PICKER_MIN_WIDTH = 120
const PICKER_MAX_WIDTH = 420

/**
 * What the key column opens at where it is drawn as a tree. A tree's name column is wider than a
 * plain one — it holds an indent, a caret and a name — and it is only the width it *opens* at.
 */
const TREE_KEY_WIDTH = 360

interface TableDataViewProps {
  sessionId: string
  /** Which connection this table is on — half of what names the column order it remembers. */
  profileId: string
  namespace: Namespace
  table: string
  /** Which tab this is, and so which view is the reader's — see `stores/tableViews.ts`. */
  tabId: string
}

export function TableDataView({
  sessionId,
  profileId,
  namespace,
  table,
  tabId,
}: TableDataViewProps) {
  const {
    data,
    loadedAt,
    loading,
    fetch,
    error,
    page,
    limit,
    sort,
    criteria,
    goToPage,
    changeLimit,
    changeSort,
    changeCriteria,
    refresh,
    stopFetch,
  } = useTableData(sessionId, namespace, table, tabId)

  /**
   * Whether this namespace is a **key space**, which changes four things about the surface: the rows
   * group into a tree, the builder is replaced by a search, the panel below is always open and holds
   * the key rather than one cell, and the cell menu is the short one. It is `EngineInfo.keys` and
   * never a comparison against an engine's name.
   */
  const engine = useConnectionsStore(
    (s) => s.profiles.find((profile) => profile.id === profileId)?.engine ?? '',
  )
  const keySpace = engineInfo(engine).keys

  // A table outlives the app, so the order its columns were dragged into is stored under it. The
  // tab is keyed on its own target, so the key is read once and never changes under the component.
  const orderKey = columnOrderKey(profileId, namespace, table)
  const [order, setOrder] = useState(() => loadColumnOrder(orderKey))

  // How the tab was set up is the tab's and survives it being looked away from; the selection is
  // not — it names rows of a read this mount is about to make again.
  const { panels, hidden, filterShown, flatten, shut, keyMatch } = useTableViewsStore(
    (s) => s.views[tabId] ?? IDLE_VIEW,
  )
  const change = useTableViewsStore((s) => s.change)
  const [selection, setSelection] = useState<Selection | null>(null)
  const [curlOpen, setCurlOpen] = useState(false)

  const setFilterShown = (shown: boolean) => change(tabId, { filterShown: shown })

  const columns = data?.columns ?? []
  const editable = columns.some((column) => column.primaryKey)
  const edits = useGridEdits(tabId, sourceOf(data, namespace, table), sessionId, refresh)

  /** The columns in the reader's own order, each keeping the place it holds in a row. */
  const arranged = useMemo(() => arrange(columns, order), [columns, order])

  const visible = useMemo(
    () => arranged.filter(({ column }) => !hidden.has(column.name)),
    [arranged, hidden],
  )

  /** Which column identifies a row, and so which one a key space groups its tree by. */
  const keyColumn = keySpace ? columnAt(columns, KEY_COLUMN) : -1

  /**
   * The keys of the page as the tree they group into, or `null` where the rows are drawn flat — every
   * engine but this one, and this one once the reader has pressed Flatten. It is built over
   * `edits.rows` rather than over the read, so a staged rename regroups and a draft key appears.
   */
  const tree = useMemo(
    () =>
      keyColumn < 0 || flatten
        ? null
        : keyRows(
            edits.rows.map((row) => row[keyColumn] ?? null),
            shut,
          ),
    [keyColumn, flatten, edits.rows, shut],
  )

  const outline = useMemo<GridOutline | undefined>(
    () =>
      tree === null
        ? undefined
        : {
            depth: (row) => tree[row]?.depth ?? 0,
            open: (row) => tree[row]?.open,
            onToggle: (row) => {
              const branch = tree[row]?.branch
              if (branch !== undefined) change(tabId, { shut: toggled(shut, branch) })
            },
            badge: (row) => {
              const held = tree[row]
              return held === undefined ? undefined : keyBadge(held)
            },
          },
    [tree, shut, change, tabId],
  )

  /** The page as it stands, in the reader's own column order. What an export writes is this. */
  const flatRows = useMemo(
    () => edits.rows.map((row) => visible.map((entry) => row[entry.index] ?? null)),
    [edits.rows, visible],
  )

  const grid = useMemo(() => {
    const drawn = visible.map((entry) => entry.column)
    if (tree === null) return { columns: drawn, rows: flatRows }

    return {
      // The hierarchy is one *column's* business and not a mode of the grid — see `GridOutline`.
      columns: drawn.map((column, at): ResultColumn =>
        visible[at]?.index === keyColumn ? { ...column, outline, width: TREE_KEY_WIDTH } : column,
      ),
      rows: tree.map((row) => {
        const held = row.source < 0 ? undefined : edits.rows[row.source]
        return visible.map((entry) => {
          // The bare segment, the prefix being on the row above. How many keys are under it is the
          // grid's own pill beside the name and never part of this text — see `GridOutline.badge`.
          if (entry.index === keyColumn) return row.label
          // A branch stands for a prefix and not for a key, so it has no type, value or clock — and
          // is drawn **blank** rather than as `(Null)`, which is the word for a value the server said
          // was absent. There is no such value here: there is no key.
          if (held === undefined) return ''
          return held[entry.index] ?? null
        })
      }),
    }
  }, [tree, flatRows, visible, keyColumn, outline, edits.rows])

  /**
   * Which row of the page the grid's row *n* is. A tree reorders and adds rows, so the two differ
   * exactly as the visible columns differ from the real ones — and a **branch** that is no key of its
   * own answers `-1`, there being nothing there to write to.
   */
  const realRow = (row: number) => (tree === null ? row : (tree[row]?.source ?? -1))

  /** The grid reports positions among the visible columns, which map back to real indexes. */
  const realColumn = (column: number) => visible[column]?.index ?? -1

  const target = useMemo(() => {
    const focus = focusOf(selection)
    if (!focus) return null

    const entry = visible[focus.column]
    const row = edits.rows[realRow(focus.row)]
    if (!entry || !row) return null

    return {
      ref: { row: realRow(focus.row), column: entry.index },
      column: entry.column,
      value: row[entry.index] ?? null,
    }
  }, [selection, visible, edits.rows, tree])

  /**
   * The key the panel below is showing: a whole row rather than one cell, because that is what a
   * Redis row is.
   *
   * **A branch names none, and a grid nobody has clicked in names none.** A prefix is a folder the
   * reader is walking through rather than a key they picked, so clicking one opens nothing below — and
   * nothing opens before the first click either, there being no key the reader has asked about yet. A
   * key that is *also* a prefix is reached with **Flatten**, where every row is a key.
   */
  const keyTarget = useMemo(() => {
    if (!keySpace) return null

    const picked = focusOf(selection)?.row
    if (picked === undefined || isBranch(tree, picked)) return null

    const source = realRow(picked)
    const row = source < 0 ? undefined : edits.rows[source]
    if (!row) return null

    return {
      source,
      /** A key not on the server yet, which is the one state whose type is the reader's to pick. */
      isNew: source >= (data?.rows.length ?? 0),
      name: cellAt(row, columnAt(columns, KEY_COLUMN)) ?? '',
      type: cellAt(row, columnAt(columns, TYPE_COLUMN)) ?? '',
      value: cellAt(row, columnAt(columns, VALUE_COLUMN)),
      ttl: cellAt(row, columnAt(columns, TTL_COLUMN)),
    }
  }, [keySpace, selection, edits.rows, columns, tree, data])

  /**
   * The whole of that key's value, where the page could only carry a sample of it. The grid's bound
   * is what makes a page of a hundred keys answerable; the panel is looking at one, so there is
   * nothing there for the bound to protect — see `useWholeValue`.
   */
  const whole = useWholeValue({
    sessionId,
    namespace,
    table,
    key: keyTarget?.name ?? '',
    sampled: sampledTotal(keyTarget?.value ?? '') !== null,
    at: loadedAt,
  })

  /**
   * Where the reader is in the whole table. The rows are paged on the server, so the grid's first
   * row is this page's first row and the read's own offset is what puts it in the table — and with
   * nothing focused, the page's first row is where the reader is.
   */
  const record =
    data === null ? 0 : data.offset + Math.max(0, realRow(focusOf(selection)?.row ?? 0)) + 1

  const togglePanel = (kind: PanelKind) => change(tabId, { panels: toggled(panels, kind) })

  const toggleColumn = (name: string) => {
    setSelection(null)
    change(tabId, { hidden: toggled(hidden, name) })
  }

  /**
   * The draft lands after every fetched row, and is selected so a paste that follows means it.
   *
   * On a key space it opens with **the prefix the reader is looking inside**, ending in the separator:
   * a key is added beside another one, so the whole of what is left to type is the tail. It is the
   * same answer whichever row is picked — a branch is that folder, and a leaf is a key *in* it — which
   * is what makes the button mean one thing wherever the reader happens to be standing.
   */
  const addRow = () => {
    setSelection(oneRange(rowRange(grid.rows.length, visible.length)))
    edits.addRow(keyColumn >= 0 ? { [keyColumn]: addedUnder() } : undefined)
  }

  /**
   * The prefix a fresh key opens with. A **branch** is a folder, so it is that folder; a **leaf** is a
   * key inside one, so it is everything up to its last separator. A key with no separator in it is at
   * the top and there is no prefix to offer, which is the empty box.
   */
  const addedUnder = (): string => {
    const picked = focusOf(selection)?.row
    if (picked === undefined) return ''

    const branch = tree?.[picked]?.branch
    if (branch !== undefined) return `${branch}${KEY_SEPARATOR}`

    const name = keyTarget?.name ?? ''
    const at = name.lastIndexOf(KEY_SEPARATOR)
    return at < 0 ? '' : name.slice(0, at + 1)
  }

  const showAllColumns = () => {
    setSelection(null)
    change(tabId, { hidden: new Set() })
  }

  /**
   * The grid reports where a column was dropped among the columns it *drew*, so the name it landed
   * before is what places it among them all — a hidden column keeps the place it had. The selection
   * goes, its cell coordinates being positions among those drawn columns.
   */
  const moveColumn = (from: number, to: number) => {
    const name = visible[from]?.column.name
    if (name === undefined) return

    const next = moveName(
      arranged.map((one) => one.column.name),
      name,
      visible[to]?.column.name,
    )

    setSelection(null)
    setOrder(next)
    saveColumnOrder(orderKey, next)
  }

  const changePage = (next: number) => {
    setSelection(null)
    goToPage(next)
  }

  const changePageSize = (next: number) => {
    setSelection(null)
    changeLimit(next)
  }

  const changeOrder = (next: SortKey[]) => {
    setSelection(null)
    changeSort(next)
  }

  /**
   * The conditions travel into the read's own `WHERE`, so a filter narrows the whole table
   * rather than the page in front — and the panel that spells them opens with the first one.
   */
  const changeConditions = (next: Group) => {
    setSelection(null)
    setFilterShown(next.children.length > 0 || filterShown)
    changeCriteria(next)
  }

  /**
   * The same conditions from the other spelling, and no panel to open — see `writeSearch`. How the
   * text is matched travels with the tab rather than with the conditions, an empty box writing none.
   */
  const changeSearch = (next: KeySearch) => {
    setSelection(null)
    change(tabId, { keyMatch: next.match })
    changeCriteria(writeSearch(next))
  }

  /** Stages a whole key: a rename, a type, a value and a clock are one Apply. */
  const applyKey = (edit: KeyEdit) => {
    if (keyTarget === null) return

    const stage = (name: string, value: Cell, valueType?: string) => {
      const column = columnAt(columns, name)
      if (column >= 0) edits.setCell({ row: keyTarget.source, column }, value, valueType)
    }

    if (keyTarget.isNew) {
      // A key that is not on the server yet is *inserted*, and an insert carries its columns as they
      // stand — so there the type is a cell like any other, and it is what decides how the value is
      // written.
      stage(TYPE_COLUMN, edit.type)
      stage(VALUE_COLUMN, edit.value)
    } else if (edit.type !== keyTarget.type || edit.value !== keyTarget.value) {
      // A key that exists is *written*, and there the type cannot be a cell of its own: a key's type
      // is what its value is, so the two are one fact and one write. Either of them changing is that
      // write, and the type travels on it — see `driver.CellEdit.ValueType`.
      stage(VALUE_COLUMN, edit.value, edit.type)
    }

    if (edit.ttl !== keyTarget.ttl) stage(TTL_COLUMN, edit.ttl)
    // Last, because every command above names the key by the name it had.
    if (edit.name !== keyTarget.name) stage(KEY_COLUMN, edit.name)
  }

  /**
   * The values a column holds, for the filter builder's picker. Held steady across renders,
   * because the picker reads it once per column and one rebuilt every render would never stop.
   */
  const suggest = useMemo(
    () => (column: string) =>
      tables.columnValues(sessionId, {
        database: namespace.database,
        schema: namespace.schema,
        table,
        column,
      }),
    [sessionId, namespace.database, namespace.schema, table],
  )

  /**
   * Lays a value out for the value viewer — the engine's own `Format`, since a document store's
   * documents are its own notation and not JSON. Held steady across renders for the reason
   * `suggest` is: the viewer calls it from an effect, and one rebuilt every render would never
   * settle. A SQL engine never types a column `object`, so on one it is simply never called.
   */
  const formatValue = useMemo(
    () => (value: string) => queries.format(sessionId, value),
    [sessionId],
  )

  /** The panel edits the focused cell, so the menu's own cell becomes the selection first. */
  const editInPanel = (ref: CellRef) => {
    setSelection(oneRange({ anchor: ref, focus: ref }))
    if (!keySpace) change(tabId, { panels: new Set(panels).add('cell') })
  }

  /**
   * The page in front, with the columns that are showing — a table's rows are paged by the
   * server, so what the grid holds *is* the page, and a hidden column is one nobody asked for.
   * It writes the rows **flat**: a tree draws a key by its tail, the prefix being on the row above,
   * and a file of tails would be a file of keys no server has.
   */
  const exportRows = (format: ExportFormat) =>
    void rowExport.run(sessionId, format, {
      namespace,
      table,
      columns: visible.map((entry) => entry.column.name),
      rows: flatRows,
    })

  const failure = edits.error ?? whole.error ?? (data ? error : null)

  /**
   * Which panels are open, named for the splitter. A layout is remembered per *combination*, which
   * is what `panelIds` is for: the width the reader gave the column list is theirs the next time
   * they open it, and closing it does not teach the grid a new size. A key space's panel is always
   * one of them, there being no toggle for it.
   */
  const editorShown = keySpace || panels.has('cell')
  const rowPanels = useMemo(
    () => (editorShown ? ['rows', keySpace ? 'key' : 'cell'] : ['rows']),
    [editorShown, keySpace],
  )
  const gridPanels = useMemo(() => (panels.has('column') ? ['picker', 'grid'] : ['grid']), [panels])

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TableToolbar
        active={panels}
        onToggle={togglePanel}
        filterShown={filterShown}
        filterCount={countConditions(criteria) + sort.length}
        onFilterToggle={() => setFilterShown(!filterShown)}
        onExport={data !== null ? exportRows : undefined}
        onGenerateCurl={data !== null ? () => setCurlOpen(true) : undefined}
        keySpace={
          keySpace
            ? {
                search: readSearch(criteria, keyMatch),
                onSearchChange: changeSearch,
                flatten,
                onFlattenToggle: () => change(tabId, { flatten: !flatten }),
              }
            : undefined
        }
      />

      {/* Both side panels are dragged rather than fixed, and the handle between them is the only
          divider — a border of their own beside it would read as two lines. */}
      <Split
        id="vibe-db.table.rows"
        orientation="vertical"
        panelIds={rowPanels}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="rows" minSize="25%">
          <Split
            id="vibe-db.table.columns"
            orientation="horizontal"
            panelIds={gridPanels}
            className="min-h-0"
          >
            {panels.has('column') ? (
              <>
                <SplitPanel
                  id="picker"
                  defaultSize={PICKER_WIDTH}
                  minSize={PICKER_MIN_WIDTH}
                  maxSize={PICKER_MAX_WIDTH}
                >
                  <ColumnPicker
                    columns={arranged.map((one) => one.column)}
                    hidden={hidden}
                    onToggle={toggleColumn}
                  />
                </SplitPanel>

                <SplitHandle orientation="horizontal" />
              </>
            ) : null}

            <SplitPanel id="grid" className="relative min-w-0">
              {/* A read of every row arrives in batches and is watched rather than waited on, so
                  its count and its Stop sit over whatever the grid already holds. */}
              {fetch !== null ? <FetchProgress fetched={fetch.fetched} onStop={stopFetch} /> : null}

              {/* Nothing has arrived yet: a grid with no columns is a blank pane, and the first page
                  of a large table is exactly where the reader waits longest. Once rows are on screen
                  a re-read leaves them there and the footer's pager says it is working. */}
              {data === null ? (
                error ? (
                  <Empty title={error.message} hint={error.detail} />
                ) : fetch === null ? (
                  // A streamed read reports itself, and two spinners over one another say no more.
                  <Loading label={`Loading ${table}…`} />
                ) : null
              ) : (
                <ResultGrid
                  result={grid}
                  selection={selection}
                  // On a key space what a press picks is the **key**, not the cell under the pointer:
                  // a row there is one key and its five facts, so every range is widened to the whole
                  // row exactly as a press in the row gutter already does anywhere. The band then runs
                  // the width of the row rather than colouring one cell of it.
                  onSelectionChange={(next) =>
                    setSelection(keySpace ? wholeRows(next, grid.columns.length) : next)
                  }
                  // A key space passes none, which is what makes its rows **read** rather than
                  // typed: one click there shows the key in the panel below and changes nothing.
                  // Inline editing is the right gesture on a table store — a cell is one of forty
                  // and the reader is pointing at it already — and the wrong one here, where a click
                  // is how a reader walks a key space and the value is a container the panel lays
                  // out. Editing has one place, and it is that panel.
                  onEditCell={
                    editable && !keySpace
                      ? (ref: CellRef, value) => {
                          const row = realRow(ref.row)
                          if (row >= 0) edits.setCell({ row, column: realColumn(ref.column) }, value)
                        }
                      : undefined
                  }
                  isDirty={(row, column) => {
                    const at = realRow(row)
                    return at >= 0 && edits.isDirty(at, realColumn(column))
                  }}
                  isDeleted={(row) => {
                    const at = realRow(row)
                    return at >= 0 && edits.deleted.has(at)
                  }}
                  sort={sort}
                  onSortChange={changeOrder}
                  onMoveColumn={moveColumn}
                  // Export and Filter & Sort live in this surface's own toolbar, so the grid draws
                  // no strip of its own — but the cell menu still reaches the same handlers.
                  onExport={exportRows}
                  headStrip={false}
                  onEditInPanel={editInPanel}
                  onDeleteRows={
                    editable
                      ? (rows) => edits.deleteRows(rows.map(realRow).filter((row) => row >= 0))
                      : undefined
                  }
                  onRefresh={refresh}
                  formatValue={formatValue}
                  // A key space is narrowed by the bar above rather than by the builder, so it
                  // passes none: no panel, no button, and no Add Filter on a header's own menu.
                  filtering={
                    keySpace
                      ? undefined
                      : {
                          criteria,
                          shown: filterShown,
                          onChange: changeConditions,
                          onShownChange: setFilterShown,
                          suggest,
                        }
                  }
                  cellMenu={
                    keySpace
                      ? (context) => keyMenuItems(context, { onAddKey: addRow })
                      : undefined
                  }
                  columnMenuItems={(column) => [
                    {
                      label: 'Hide column',
                      icon: ICON.hide,
                      separatorBefore: true,
                      disabled: visible.length === 1,
                      onSelect: () => toggleColumn(column.name),
                    },
                    {
                      label: 'Show all columns',
                      icon: ICON.show,
                      disabled: hidden.size === 0,
                      onSelect: showAllColumns,
                    },
                  ]}
                />
              )}
            </SplitPanel>
          </Split>
        </SplitPanel>

        {editorShown ? (
          <>
            <SplitHandle orientation="vertical" />

            {keySpace ? (
              <SplitPanel
                id="key"
                defaultSize={KEY_EDITOR_HEIGHT}
                minSize={KEY_EDITOR_MIN_HEIGHT}
                maxSize="70%"
              >
                {keyTarget === null ? (
                  <Empty title="No key selected" hint="Click a key in the grid to edit it." />
                ) : (
                  <KeyEditor
                    // The whole value arriving is a different value to edit, so the panel starts
                    // again on it: a draft typed against a sample was never going to be written.
                    key={`${keyTarget.source}:${whole.value === null ? 'sample' : 'whole'}`}
                    keyName={keyTarget.name}
                    type={keyTarget.type}
                    value={whole.value ?? keyTarget.value}
                    ttl={keyTarget.ttl}
                    busy={edits.saving}
                    onApply={applyKey}
                    formatValue={formatValue}
                  />
                )}
              </SplitPanel>
            ) : (
              <SplitPanel
                id="cell"
                defaultSize={EDITOR_HEIGHT}
                minSize={EDITOR_MIN_HEIGHT}
                maxSize="70%"
              >
                {target === null ? (
                  <Empty
                    title="No cell selected"
                    hint="Click a cell in the grid to edit its value."
                  />
                ) : (
                  <CellEditor
                    key={`${target.ref.row}:${target.ref.column}`}
                    column={target.column}
                    value={target.value}
                    readOnly={
                      editable
                        ? undefined
                        : 'This table has no primary key, so a row cannot be identified for editing.'
                    }
                    busy={edits.saving}
                    onApply={(value) => edits.setCell(target.ref, value)}
                    formatValue={formatValue}
                  />
                )}
              </SplitPanel>
            )}
          </>
        ) : null}
      </Split>

      {failure !== null ? <ErrorStrip error={failure} /> : null}

      <CurlDrawer
        open={curlOpen}
        target={{ namespace, table, fields: columns.map((column) => column.name) }}
        onOpenChange={setCurlOpen}
      />

      <GridFooter
        editable={editable}
        selected={selection !== null}
        pending={edits.pending}
        busy={edits.saving || loading}
        onAddRow={addRow}
        onDeleteRow={() =>
          edits.deleteRows(selectedRows(selection).map(realRow).filter((row) => row >= 0))
        }
        onApply={() => void edits.apply()}
        onDiscard={edits.discard}
        onRefresh={refresh}
      >
        <GridPager
          page={page}
          size={limit}
          total={data?.total ?? 0}
          loading={loading}
          onPageChange={changePage}
          onSizeChange={changePageSize}
        />
      </GridFooter>

      <GridStatus
        statement={data?.statement ?? ''}
        record={record}
        total={data?.total ?? 0}
        page={page + 1}
        pageCount={pageCount(limit, data?.total ?? 0)}
        loadedAt={loadedAt}
      />
    </div>
  )
}

/**
 * Whether the grid's row *n* is a **prefix** rather than a key. A branch has a caret, and a caret is
 * exactly what a leaf has not got — so `open` is the question, and it is asked of the tree and not of
 * `source`: a key that is also a prefix is a prefix here.
 */
function isBranch(tree: KeyRow[] | null, row: number): boolean {
  return tree !== null && tree[row]?.open !== undefined
}

/**
 * The same selection with every rectangle widened to the whole row. It is what the grid does for a
 * press in the row gutter, said here for a surface whose unit is the row wherever it is pressed — and
 * the focus lands on the last column, as a gutter press leaves it, so the arrows still walk rows.
 */
function wholeRows(selection: Selection, width: number): Selection {
  const last = Math.max(0, width - 1)

  return {
    ranges: selection.ranges.map((range) => ({
      anchor: { row: range.anchor.row, column: 0 },
      focus: { row: range.focus.row, column: last },
    })),
  }
}

/** The same set with one member in or out, which is what every toggle in this view means. */
function toggled<T>(members: ReadonlySet<T>, one: T): ReadonlySet<T> {
  const next = new Set(members)
  if (!next.delete(one)) next.add(one)
  return next
}

function sourceOf(data: TablePage | null, namespace: Namespace, table: string): EditSource | null {
  if (data === null) return null

  return {
    target: { database: namespace.database, schema: namespace.schema, table },
    columns: data.columns,
    rows: data.rows,
  }
}
