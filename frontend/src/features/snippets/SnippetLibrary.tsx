import { useMemo, useState } from 'react'
import { Bookmark } from 'lucide-react'

import { engineInfo } from '@/bridge/connections'
import { emptySnippet, toSnippetInput, type Snippet, type SnippetInput } from '@/bridge/snippets'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import {
  focusOf,
  NO_SORT,
  oneLine,
  type Cell,
  type GridData,
  type Selection,
  type SortKey,
} from '@/features/results/resultSet'
import { useGridOrder } from '@/features/results/useGridOrder'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { ICON } from '@/shared/ui/menuIcons'
import { useSchemaStore } from '@/stores/schema'
import { useSnippetsStore } from '@/stores/snippets'

import { SnippetDialog } from './SnippetDialog'

interface SnippetLibraryProps {
  /** What a New snippet is tagged with by default: the engine the reader is in front of. */
  engine: string
  /** Two clicks put the snippet's body in the editor above — see `QueryWorkspace`. */
  onPick: (one: Snippet) => void
}

/**
 * The statements the reader has named. It sits beside `History` because the two are the same kind of
 * thing looked at from opposite ends: history is every statement that ran, and this is the handful
 * somebody decided to keep.
 *
 * It is a `ResultGrid`, because it is rows and columns, and it is read-only: a body is edited in the
 * dialog, which has the room for a script. A **double-click inserts** the snippet into the editor
 * above, fields and all — which is what the library is for; the toolbar acts on whichever row the
 * selection names.
 *
 * The **whole** library is listed here rather than only this engine's. The editor's completion is
 * narrowed, because a name offered where it cannot run is noise — but a reader looking at their
 * library is looking for everything in it, and a snippet they cannot see is one they cannot edit.
 *
 * **Nothing is read when it mounts.** The library is read once when the window opens, every editor's
 * completion needing it whether or not this pane has ever been looked at, so opening the pane draws
 * what is already held; `Refresh` is for a file somebody has edited underneath the app.
 */
export function SnippetLibrary({ engine, onPick }: SnippetLibraryProps) {
  const library = useSnippetsStore((s) => s.library)
  const loading = useSnippetsStore((s) => s.loading)
  const error = useSnippetsStore((s) => s.error)
  const load = useSnippetsStore((s) => s.load)
  const remove = useSnippetsStore((s) => s.remove)
  const ask = useSchemaStore((s) => s.ask)

  const [selection, setSelection] = useState<Selection | null>(null)
  const [sort, setSort] = useState<readonly SortKey[]>(NO_SORT)
  /** The dialog's draft, and non-null exactly while it is up. */
  const [draft, setDraft] = useState<SnippetInput | null>(null)

  const data = useMemo(() => libraryRows(library), [library])
  const order = useGridOrder(data.rows, data.columns, sort)
  const rows = useMemo(() => order.map((row) => data.rows[row] ?? []), [order, data.rows])

  /** The grid draws the ordered rows, so a row it reports is a place in that order. */
  const at = (row: number): Snippet | null => library[order[row] ?? -1] ?? null

  const focus = focusOf(selection)
  const picked = focus ? at(focus.row) : null

  const confirmDelete = (one: Snippet) =>
    ask({
      kind: 'confirm',
      title: 'Delete Snippet',
      message: `Forget “${one.name}”? Nothing on any server changes.`,
      submitLabel: 'Delete',
      danger: true,
      onSubmit: () => remove(one.name),
    })

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        role="toolbar"
        className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
      >
        <Button variant="subtle" icon={ICON.add} onClick={() => setDraft(emptySnippet(engine))}>
          New Snippet
        </Button>

        <span className="mx-0.5 h-4 w-px bg-line" />

        <Button
          variant="subtle"
          icon={ICON.rename}
          disabled={picked === null}
          onClick={() => picked && setDraft(toSnippetInput(picked))}
        >
          Edit
        </Button>

        <Button
          variant="subtle"
          icon={ICON.delete}
          disabled={picked === null}
          onClick={() => picked && confirmDelete(picked)}
          className="text-danger hover:bg-danger-subtle hover:text-danger"
        >
          Delete
        </Button>

        <div className="min-w-2 flex-1" />

        {/* Insert is the double-click drawn as a button, because a double-click is not discoverable
            and this is the one thing the pane exists to do. */}
        <Button
          variant="default"
          icon={ICON.paste}
          disabled={picked === null}
          onClick={() => picked && onPick(picked)}
        >
          Insert
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        {/* A library that could not be read is not an empty one, and saying "no snippets yet" over a
            failed read would tell the reader their file is empty when it may be full. */}
        {library.length === 0 ? (
          error !== null ? (
            <Empty title={error.message} hint={error.detail} />
          ) : (
            <Empty
              icon={<Bookmark size={22} strokeWidth={1.5} />}
              title="No snippets yet"
              hint="Name a statement you keep coming back to and it is offered by that name in every editor — here, and in completion as you type."
              action={
                <Button
                  variant="primary"
                  icon={ICON.add}
                  onClick={() => setDraft(emptySnippet(engine))}
                >
                  New Snippet
                </Button>
              }
            />
          )
        ) : (
          <ResultGrid
            result={{ columns: data.columns, rows }}
            selection={selection}
            onSelectionChange={setSelection}
            onActivateRow={(row) => {
              const one = at(row)
              if (one) onPick(one)
            }}
            onRenameRow={(row) => {
              const one = at(row)
              if (one) setDraft(toSnippetInput(one))
            }}
            sort={sort}
            onSortChange={setSort}
            onRefresh={() => void load()}
          />
        )}
      </div>

      {/* A failed read over a library that *is* drawn leaves it where it is and says so under it,
          exactly as the History pane does with the same failure. */}
      {error !== null && library.length > 0 ? <ErrorStrip error={error} /> : null}

      <GridFooter editable={false} busy={loading} onRefresh={() => void load()}>
        <span>{library.length === 1 ? '1 snippet' : `${library.length} snippets`}</span>
      </GridFooter>

      {draft !== null ? (
        <SnippetDialog draft={draft} onClose={() => setDraft(null)} />
      ) : null}
    </div>
  )
}

function libraryRows(library: Snippet[]): GridData {
  return {
    columns: [column('Name'), column('Engine'), column('Description'), column('Statement')],
    rows: library.map((one): Cell[] => [
      one.name,
      // What an engine is *called* is the engine's own label, never a comparison in a component; a
      // snippet tagged with none is offered everywhere, and the cell says so rather than being blank.
      one.engine === '' ? 'Every engine' : engineInfo(one.engine).label,
      one.description,
      oneLine(one.body),
    ]),
  }
}

const column = (name: string, type = 'text') => ({ name, type, fullType: type })
