import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Check, Parentheses, Plus, X } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { IconButton } from '@/shared/ui/IconButton'
import { MenuButton } from '@/shared/ui/MenuButton'
import { ICON } from '@/shared/ui/menuIcons'

import { FieldPick, PART } from './FilterPicker'
import { ValuePick } from './FilterValuePanel'
import {
  appendTo,
  arityOf,
  fitValues,
  FILTER_OPS,
  insertAfter,
  newCondition,
  newGroup,
  replaceAt,
  symbolOf,
  type Condition,
  type Criterion,
  type GridFiltering,
  type Group,
  type Path,
} from './filter'
import type { ResultColumn, SortKey } from './resultSet'

/** How far a group's contents sit in from the parentheses that hold them. */
const INDENT = 20
/** The label column both sections share, so `Filter` and `Sort By` line their rows up. */
const LABEL_WIDTH = 56

/**
 * A field a header's `⋮` asked to filter on. An object rather than a bare name so that asking
 * twice for the same field is two asks, each adding its own row.
 */
export interface FilterAsk {
  column: string
}

interface FilterSortPanelProps {
  columns: ResultColumn[]
  filtering: GridFiltering
  sort: readonly SortKey[]
  onSortChange?: ((sort: SortKey[]) => void) | undefined
  /**
   * The field a header asked to filter on, or null. It lands in the **draft** as a fresh condition:
   * a header knows the field and not the value, so what it adds was not picked whole and is
   * therefore not the cell menu's exception — nothing narrows until Apply.
   */
  ask?: FilterAsk | null
  /** Clears that ask, so a panel closed and opened again does not add the same row twice. */
  onAskTaken?: (() => void) | undefined
}

/**
 * Where a grid is narrowed and ordered: the conditions as a tree the reader builds, and the keys it
 * is sorted by, over one **Apply**. It is opened from the Filter & Sort button over the grid and
 * from the cell menu's **Show/Hide Filter & Sort**, and what it holds goes on holding whether it is
 * showing or not — hiding the panel is not widening the grid.
 *
 * Nothing here narrows anything until **Apply**: a condition is three parts and a value picked from
 * a list, and a read per keystroke would be a read of the rows nobody asked for yet. A condition
 * added from the **cell menu** is the exception and applies at once, having been picked whole.
 */
export function FilterSortPanel({
  columns,
  filtering,
  sort,
  onSortChange,
  ask = null,
  onAskTaken,
}: FilterSortPanelProps) {
  const [criteria, setCriteria] = useState(filtering.criteria)
  const [keys, setKeys] = useState<readonly SortKey[]>(sort)
  const taken = useRef<FilterAsk | null>(null)
  const inForce = useRef(filtering.criteria)

  // The draft follows what is in force **as that changes**, so a condition added from the cell menu
  // or a sort taken off a header lands in the builder rather than being overwritten by the draft
  // beside it. The tree last seen is remembered rather than simply assigned, because an effect run
  // again for a tree that never changed would undo the row an ask below has just added to the draft.
  useEffect(() => {
    if (inForce.current === filtering.criteria) return

    inForce.current = filtering.criteria
    setCriteria(filtering.criteria)
  }, [filtering.criteria])
  useEffect(() => setKeys(sort), [sort])

  // A header's Add Filter appends its field to the draft — at the end of the root, where the cell
  // menu's own conditions land. The ask is taken **once**: React runs an effect twice for a single
  // commit in development, and one click would otherwise add two rows.
  useEffect(() => {
    if (ask === null || taken.current === ask) return

    taken.current = ask
    setCriteria((current) => appendTo(current, [], newCondition(ask.column)))
    onAskTaken?.()
  }, [ask, onAskTaken])

  const names = useMemo(() => columns.map((column) => column.name), [columns])
  const firstColumn = names[0] ?? ''
  const edited = criteria !== filtering.criteria || keys !== sort

  /** Nothing is sent that has not changed: Apply on an untouched panel is not a re-read. */
  const apply = () => {
    if (criteria !== filtering.criteria) filtering.onChange(criteria)
    if (keys !== sort) onSortChange?.([...keys])
  }

  const edit: Edit = {
    names,
    suggest: filtering.suggest,
    set: (path, node) => {
      // The root is the tree itself, so replacing it is not replacing something inside it.
      if (path.length === 0) {
        if (node?.kind === 'group') setCriteria(node)
        return
      }
      setCriteria(replaceAt(criteria, path, node))
    },
    addBeside: (path, node) => setCriteria(insertAfter(criteria, path, node)),
    addInside: (path, node) => setCriteria(appendTo(criteria, path, node)),
  }

  return (
    <div className="shrink-0 border-b border-line bg-panel">
      <div className="flex items-center gap-1.5 px-2 pt-1.5">
        <Label>Filter</Label>
        <Adders
          disabled={firstColumn === ''}
          onCondition={() => edit.addInside([], newCondition(firstColumn))}
          onGroup={() => edit.addInside([], newGroup())}
        />
        <div className="min-w-2 flex-1" />
        <ModeChoice />
      </div>

      <div className="scroll-thin max-h-52 overflow-y-auto px-2 py-1">
        <Rows group={criteria} path={[]} depth={0} edit={edit} />
      </div>

      <div className="flex items-start gap-1.5 border-t border-line px-2 py-1.5">
        <Label>Sort By</Label>
        <IconButton
          label="Add sort criteria"
          icon={<Plus size={12} />}
          disabled={firstColumn === '' || onSortChange === undefined}
          className="h-5 w-5 border border-line-strong bg-panel"
          onClick={() => setKeys([...keys, { column: firstColumn, descending: false }])}
        />

        {keys.length === 0 ? (
          <span className="pt-0.5 text-xs text-faint">Click “+” to add sort criteria</span>
        ) : (
          <div className="flex min-w-0 flex-col gap-0.5">
            {keys.map((key, index) => (
              <SortRow
                key={index}
                names={names}
                sortKey={key}
                onChange={(next) => setKeys(keys.map((one, at) => (at === index ? next : one)))}
                onRemove={() => setKeys(keys.filter((_, at) => at !== index))}
              />
            ))}
          </div>
        )}
      </div>

      <div className="flex items-center gap-3 border-t border-line px-2 py-1.5">
        <Button variant="primary" disabled={!edited} onClick={apply}>
          Apply Filter &amp; Sort
        </Button>
        <span className="text-xs text-muted">{edited ? 'Criteria Edited' : ''}</span>

        <div className="min-w-2 flex-1" />

        <IconButton
          label="Hide Filter & Sort"
          icon={<X size={13} />}
          onClick={() => filtering.onShownChange(false)}
        />
      </div>
    </div>
  )
}

/** Everything a row of the builder acts through, so the tree renders itself down one prop. */
interface Edit {
  names: readonly string[]
  suggest?: ((column: string) => Promise<string[]>) | undefined
  set: (path: Path, node: Criterion | null) => void
  /** Beside the node at `path`, inside the same parentheses — what `+` on a condition means. */
  addBeside: (path: Path, node: Criterion) => void
  /** Inside the group at `path` — what `+` on a group row, or on the Filter header, means. */
  addInside: (path: Path, node: Criterion) => void
}

interface RowsProps {
  group: Group
  path: Path
  depth: number
  edit: Edit
}

/**
 * One group's children, and — through itself — every group under them. A child after the first
 * carries the conjunction its parent joins by, which is a button: one group joins one way, so
 * flipping it anywhere flips it for the whole group.
 */
function Rows({ group, path, depth, edit }: RowsProps) {
  const firstColumn = edit.names[0] ?? ''

  if (group.children.length === 0) {
    return (
      <Line depth={depth}>
        <span className="text-xs text-faint">
          {depth === 0 ? 'Click “+” to add a filter condition' : 'Empty group'}
        </span>
      </Line>
    )
  }

  return (
    <>
      {group.children.map((child, index) => {
        const here = [...path, index]
        const grouped = child.kind === 'group'

        return (
          <Fragment key={index}>
            <Line depth={depth}>
              <Joint
                conjunction={group.conjunction}
                shown={index > 0}
                onToggle={() =>
                  edit.set(path, {
                    ...group,
                    conjunction: group.conjunction === 'and' ? 'or' : 'and',
                  })
                }
              />

              <input
                type="checkbox"
                checked={child.enabled}
                aria-label="Apply this condition"
                title="Untick to keep the condition without applying it"
                onChange={() => edit.set(here, { ...child, enabled: !child.enabled })}
                className="h-3.5 w-3.5 shrink-0 accent-[var(--accent)]"
              />

              {grouped ? (
                <span className="shrink-0 px-1 font-mono text-xs text-muted">(</span>
              ) : (
                <ConditionParts condition={child} path={here} edit={edit} />
              )}

              <Adders
                onCondition={() =>
                  grouped
                    ? edit.addInside(here, newCondition(firstColumn))
                    : edit.addBeside(here, newCondition(firstColumn))
                }
                onGroup={() =>
                  grouped ? edit.addInside(here, newGroup()) : edit.addBeside(here, newGroup())
                }
              />

              <IconButton
                label="Remove this condition"
                icon={<X size={12} />}
                onClick={() => edit.set(here, null)}
              />
            </Line>

            {grouped ? (
              <>
                <Rows group={child} path={here} depth={depth + 1} edit={edit} />
                <Line depth={depth}>
                  <span className="shrink-0 px-1 font-mono text-xs text-muted">)</span>
                </Line>
              </>
            ) : null}
          </Fragment>
        )
      })}
    </>
  )
}

/**
 * The three parts of a condition, and each opens what it may be: the column and the value a small
 * searchable panel, the operator a plain menu — its twenty are a fixed list in a deliberate order,
 * which a search box would only get in the way of. The two ends are accent-coloured and the middle
 * is not, because those ends are the reader's own and the middle is the comparison between them.
 */
function ConditionParts({
  condition,
  path,
  edit,
}: {
  condition: Condition
  path: Path
  edit: Edit
}) {
  return (
    <>
      <FieldPick
        names={edit.names}
        value={condition.column}
        onChange={(column) => edit.set(path, { ...condition, column })}
      />

      <MenuButton
        items={FILTER_OPS.map((one) => ({
          label: one.symbol,
          icon: one.operator === condition.operator ? <Check size={13} /> : undefined,
          onSelect: () =>
            edit.set(path, {
              ...condition,
              operator: one.operator,
              // A change of operator keeps what the new one can still compare: `= 30` becoming
              // `is between` keeps the 30 as its lower bound rather than starting over.
              values: fitValues(condition.values, one.arity),
            }),
        }))}
      >
        <button type="button" className={cn(PART, 'text-fg')}>
          {symbolOf(condition.operator)}
        </button>
      </MenuButton>

      {arityOf(condition.operator) === 0 ? null : (
        // The values arrive with the operator they ask for: ticking a second is what turns `= a`
        // into `is in list (a, b)`, and the operator part above shows it the moment it happens.
        <ValuePick
          condition={condition}
          suggest={edit.suggest}
          onChange={(values, operator) => edit.set(path, { ...condition, values, operator })}
        />
      )}
    </>
  )
}

function SortRow({
  names,
  sortKey,
  onChange,
  onRemove,
}: {
  names: readonly string[]
  sortKey: SortKey
  onChange: (next: SortKey) => void
  onRemove: () => void
}) {
  return (
    <div className="flex h-6 items-center gap-1.5">
      <FieldPick
        names={names}
        value={sortKey.column}
        onChange={(column) => onChange({ ...sortKey, column })}
      />

      <MenuButton
        items={[
          {
            label: 'Ascending',
            icon: ICON.sortAscending,
            onSelect: () => onChange({ ...sortKey, descending: false }),
          },
          {
            label: 'Descending',
            icon: ICON.sortDescending,
            onSelect: () => onChange({ ...sortKey, descending: true }),
          },
        ]}
      >
        <button type="button" className={cn(PART, 'text-fg')}>
          {sortKey.descending ? 'Descending' : 'Ascending'}
        </button>
      </MenuButton>

      <IconButton label="Remove this sort" icon={<X size={12} />} onClick={onRemove} />
    </div>
  )
}

const Label = ({ children }: { children: ReactNode }) => (
  <span
    className="shrink-0 pt-0.5 text-xs font-semibold text-fg"
    style={{ width: LABEL_WIDTH }}
  >
    {children}
  </span>
)

const Line = ({ depth, children }: { depth: number; children: ReactNode }) => (
  <div className="flex h-7 items-center gap-1.5" style={{ paddingLeft: depth * INDENT }}>
    {children}
  </div>
)

/** The conjunction slot, kept even on the first child so every row's tick box lines up. */
function Joint({
  conjunction,
  shown,
  onToggle,
}: {
  conjunction: string
  shown: boolean
  onToggle: () => void
}) {
  if (!shown) return <span className="w-6 shrink-0" />

  return (
    <button
      type="button"
      onClick={onToggle}
      title="Switch this group between and / or"
      className="w-6 shrink-0 rounded text-left text-xs font-medium text-accent hover:bg-hover"
    >
      {conjunction}
    </button>
  )
}

function Adders({
  disabled = false,
  onCondition,
  onGroup,
}: {
  disabled?: boolean
  onCondition: () => void
  onGroup: () => void
}) {
  return (
    <>
      <IconButton
        label="Add a condition"
        icon={<Plus size={12} />}
        disabled={disabled}
        className="h-5 w-5 border border-line-strong bg-panel"
        onClick={onCondition}
      />
      <IconButton
        label="Add a group"
        icon={<Parentheses size={12} />}
        disabled={disabled}
        className="h-5 w-5 border border-line-strong bg-panel"
        onClick={onGroup}
      />
    </>
  )
}

/**
 * Builder or Text. **Text** is drawn and disabled: the panel is where a typed condition will go,
 * and leaving the choice out would make it look as though there were only ever one way.
 */
function ModeChoice() {
  return (
    <div role="radiogroup" aria-label="How conditions are spelled" className="flex items-center gap-3">
      <Mode label="Builder" checked />
      <Mode label="Text" checked={false} />
    </div>
  )
}

function Mode({ label, checked }: { label: string; checked: boolean }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      disabled={!checked}
      title={checked ? undefined : 'Not yet — the builder is the only way in for now'}
      className="flex items-center gap-1.5 text-xs text-fg disabled:opacity-45"
    >
      <span
        className={cn(
          'grid h-3 w-3 shrink-0 place-items-center rounded-full border',
          checked ? 'border-accent' : 'border-line-strong',
        )}
      >
        {checked ? <span className="h-1.5 w-1.5 rounded-full bg-accent" /> : null}
      </span>
      {label}
    </button>
  )
}
