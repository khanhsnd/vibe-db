import { useCallback, useEffect, useMemo, useState } from 'react'
import { Archive, CornerLeftUp, SearchX } from 'lucide-react'

import type { ObjectRow } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import { onFileDrop } from '@/bridge/filedrop'
import { GridFooter } from '@/features/results/GridFooter'
import { ResultGrid } from '@/features/results/ResultGrid'
import {
  focusOf,
  selectedRows,
  type GridOutline,
  type Selection,
  type SortKey,
} from '@/features/results/resultSet'
import { keyBadge } from '@/features/table/keyTree'
import { Button } from '@/shared/ui/Button'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { IconButton } from '@/shared/ui/IconButton'
import { Loading } from '@/shared/ui/Loading'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { cn } from '@/lib/cn'
import { rowsOf, useBucketViewsStore, type BucketView } from '@/stores/bucketViews'
import { useObjectStoresStore } from '@/stores/objectStores'
import type { BucketTab } from '@/stores/tabs'

import { BucketBreadcrumb } from './BucketBreadcrumb'
import { BucketToolbar } from './BucketToolbar'
import { objectIcon } from './objectIcon'
import { objectMenuItems } from './objectMenu'
import { ObjectPanel } from './ObjectPanel'
import { ObjectSearchBar } from './ObjectSearchBar'
import { objectGrid, objectTree, sizeText, sortRows, treeGrid } from './objectRows'
import { useObjectActions } from './useObjectActions'

/**
 * One bucket, a folder at a time. It is the object-storage counterpart of a table tab and is built
 * the same way — a toolbar, a `ResultGrid`, a panel under it, an action bar — because it is the same
 * question asked of a different kind of thing.
 *
 * Two things about the read are the whole design and are worth having in front of you:
 *
 *   1. **A folder is one request.** The service is asked for one prefix with a delimiter, so it
 *      answers with the folders inside and the objects inside and nothing else. Browsing a bucket of
 *      forty million objects therefore costs what browsing one of forty costs — which is the reason
 *      there is no index anywhere in this feature.
 *   2. **A page is forward only, and the reader never sees one.** A continuation token is what a
 *      service gives instead of an offset, so there is nothing to go back to and no page to number.
 *      The rows are **appended**, which is the arrangement that fits a thing with no size: a pager
 *      would have had to draw a fraction whose denominator is a read of the whole bucket.
 *   3. **The pages read themselves onward.** A folder's *folders* are not a thing the service will
 *      answer separately — `CommonPrefixes` and the objects come back interleaved in one key order,
 *      counting against one thousand-key page — so a root holding a few thousand loose objects can
 *      spend a whole page before reaching the prefixes that sort after them. Waiting for a scroll
 *      there would hide a folder behind four thousand rows of `.xlsx`, so the view keeps reading
 *      until the listing is complete or `AUTO_ROWS` rows are in hand, and `onReachEnd` carries it on
 *      from there.
 */
export function BucketView({ tab }: { tab: BucketTab }) {
  const session = useObjectStoresStore((s) => s.sessions[tab.profileId] ?? null)
  const sessionId = session?.sessionId ?? ''

  const view = useBucketViewsStore((s) => s.views[tab.id] ?? null)
  const open = useBucketViewsStore((s) => s.open)
  const read = useBucketViewsStore((s) => s.read)
  const goTo = useBucketViewsStore((s) => s.goTo)
  const up = useBucketViewsStore((s) => s.up)
  const more = useBucketViewsStore((s) => s.more)
  const setFlat = useBucketViewsStore((s) => s.setFlat)
  const setSearch = useBucketViewsStore((s) => s.setSearch)
  const runSearch = useBucketViewsStore((s) => s.runSearch)
  const stopSearch = useBucketViewsStore((s) => s.stopSearch)
  const clearSearch = useBucketViewsStore((s) => s.clearSearch)
  const focus = useBucketViewsStore((s) => s.focus)

  const [selection, setSelection] = useState<Selection | null>(null)
  const [failure, setFailure] = useState<AppError | null>(null)
  /** Which prefixes of a flattened page are folded. It is the tab's, like everything else here. */
  const [shut, setShut] = useState<ReadonlySet<string>>(new Set())
  /**
   * What a header was asked to order by. It sorts **what has been read** and nothing else — a
   * listing is forward only, so there is no order to ask the service for — and rows that arrive
   * afterwards are sorted in as they land.
   */
  const [sort, setSort] = useState<SortKey[]>([])

  useEffect(() => {
    open(tab.id, tab.bucket, '')
  }, [open, tab.id, tab.bucket])

  // The first read, and every re-read after a folder, a page or a Flatten. It is here rather than in
  // each action so that a tab reopened on a connection that has just come back reads itself.
  useEffect(() => {
    if (view && view.page === null && !view.loading && sessionId !== '') {
      void read(tab.id, sessionId)
    }
  }, [view, read, tab.id, sessionId])

  const rows = useMemo(() => sortRows(view ? rowsOf(view) : [], sort), [view, sort])

  /**
   * **A page that arrives with folders in it lets go of what was ticked.** Folders come first, so
   * such a page slides every object row down — and a tick that stayed where it was would go on being
   * a tick at an index that now names a different object. An index is not an identity, and a delete
   * aimed by one is the accident this feature confirms against everywhere else.
   */
  const folders = view?.page?.folders ?? 0
  useEffect(() => setSelection(null), [folders, sort])

  /**
   * **The listing reads itself onward.** One page is a thousand *keys* and a folder counts as one of
   * them, so which prefixes a reader can see is decided by what sorts before them: a root holding
   * four thousand loose objects answers its first page without ever reaching the folders below them
   * in key order. That is a fact about S3 and not about this app — a service will not list prefixes
   * on their own — so the only honest answer is to keep reading.
   *
   * It stops at `AUTO_ROWS`, and past that the scroll carries it on. The bound is what keeps a root
   * of forty million objects from being read because somebody opened it: a listing this app went on
   * fetching unattended is the unbounded read [decisions.md](../../../../docs/decisions.md) refuses
   * everywhere else.
   */
  useEffect(() => {
    if (!view || view.result !== null || view.loading) return
    if ((view.page?.next ?? '') === '' || rows.length >= AUTO_ROWS) return

    void more(tab.id, sessionId)
  }, [view, rows.length, more, tab.id, sessionId])

  /**
   * The page grouped into the tree its keys spell, and `null` where the rows are one folder's. It is
   * only ever built for a **flattened** page or a search's answer: a folder's own rows are already
   * one level deep, and grouping them would draw a tree of one branch.
   */
  const tree = useMemo(
    () => (view && (view.flat || view.result) ? objectTree(rows, shut) : null),
    [view, rows, shut],
  )

  /** Which fetched row a grid row stands for. A branch that is no object of its own answers null. */
  const rowAt = useCallback(
    (row: number): ObjectRow | null => {
      if (tree === null) return rows[row] ?? null
      const entry = tree[row]
      return entry === undefined || entry.source < 0 ? null : (rows[entry.source] ?? null)
    },
    [tree, rows],
  )

  /**
   * The name column draws its cells as the rows of a tree, which is what carries the **glyph**: a
   * folder is drawn as a folder and an object as what its name says it is. It is passed whether or
   * not the page is flattened, because an icon is not a hierarchy — a flat page is a tree of one
   * level, and `depth` answering nought is what says so.
   */
  const outline = useMemo<GridOutline>(
    () => ({
      depth: (row) => tree?.[row]?.depth ?? 0,
      open: (row) => tree?.[row]?.open,
      onToggle: (row) => {
        const branch = tree?.[row]?.branch
        if (branch === undefined) return
        setShut((current) => {
          const held = new Set(current)
          if (held.has(branch)) held.delete(branch)
          else held.add(branch)
          return held
        })
      },
      icon: (row) => {
        const held = rowAt(row)
        // A branch of a flattened page stands for a prefix no object is at, so it is drawn as the
        // folder it is rather than as nothing.
        return objectIcon(held ?? { ...EMPTY_ROW, folder: true })
      },
      badge: (row) => {
        const held = tree?.[row]
        return held === undefined ? undefined : keyBadge(held)
      },
    }),
    [tree, rowAt],
  )

  const grid = useMemo(() => {
    const data = tree === null ? objectGrid(rows) : treeGrid(rows, tree)
    // The hierarchy and the glyph are one *column's* business and not a mode of the grid.
    return {
      ...data,
      columns: data.columns.map((column, at) => (at === 0 ? { ...column, outline } : column)),
    }
  }, [tree, rows, outline])

  /** What the reader has ticked, as objects and folders — which is what every action is aimed at. */
  const picked = useMemo(
    () =>
      selectedRows(selection).flatMap((row) => {
        const held = rowAt(row)
        return held ? [held] : []
      }),
    [selection, rowAt],
  )

  const on = useObjectActions(
    tab,
    view ?? EMPTY_VIEW,
    session?.rules ?? NO_RULES,
    picked,
    setFailure,
  )

  /**
   * **Files let go over this view are uploaded into the folder it is standing in**, which is what the
   * Upload button does with the ones a dialog picked — one action, three ways of asking for it. What
   * arrives is a list of *paths*, so a dropped folder is walked on this machine exactly as a picked
   * one is and nothing here reads a byte.
   *
   * The slot is the window's rather than the tab's, because Wails registers its listeners once; only
   * the tab in front is mounted, so there is never a second view holding it.
   */
  useEffect(() => onFileDrop(on.uploadPaths), [on.uploadPaths])

  /**
   * **One click shows the object below.** A bucket's rows are things rather than values, so what a
   * press means here is "tell me about this one" — which is the arrangement a [key
   * space](../../../../docs/features/key-space.md) has, and the reason the panel is not waiting for a
   * double-click. A **folder** shows nothing: it is a prefix the reader is walking through, and there
   * is no object there to describe.
   */
  const showSelected = useCallback(
    (next: Selection) => {
      setSelection(next)

      const cell = focusOf(next)
      if (cell === null) return

      const held = rowAt(cell.row)
      focus(tab.id, held && !held.folder ? held.key : null)
    },
    [rowAt, focus, tab.id],
  )

  const act = useCallback((run: () => Promise<unknown>) => {
    setFailure(null)
    void run().catch((thrown: unknown) => setFailure(toAppError(thrown)))
  }, [])

  if (session === null) {
    return <Empty title="Not connected" hint="Reconnect this store to read its buckets." />
  }
  if (view === null) return <Loading label="Opening the bucket…" />

  // A read that nobody pressed a button for still has to be able to report a failure: `more` runs
  // off a scroll, so the store's own error is shown beside the ones an action threw.
  const shown = failure ?? view.error
  const searched = view.result !== null
  const scanned = searched
    ? `${view.result?.scanned.toLocaleString() ?? 0} keys read${view.result?.whole ? '' : ', and more to come'}`
    : ''

  // A store that refuses an upload is not a drop target at all: the rule that leaves the button out
  // leaves the zone out, so there is nothing to let go of and no refusal to explain afterwards.
  const takesFiles = !session.rules.upload.refused
  const into = view.prefix === '' ? view.bucket : `${view.bucket}/${view.prefix}`

  return (
    <div
      className={cn('flex h-full min-h-0 flex-col bg-canvas', takesFiles && 'drop-zone')}
      data-drop-label={takesFiles ? `Drop files to upload into ${into}` : undefined}
    >
      <BucketToolbar
        view={view}
        rules={session.rules}
        picked={picked}
        on={on}
        onRefresh={() => act(() => read(tab.id, sessionId))}
        onUp={() => act(() => up(tab.id, sessionId))}
        onFlatten={(flat) => act(() => setFlat(tab.id, sessionId, flat))}
      />

      <div className="flex h-7 shrink-0 items-center gap-1 border-b border-line bg-panel">
        <IconButton
          label="Up one folder"
          icon={<CornerLeftUp size={13} />}
          disabled={view.prefix === ''}
          onClick={() => act(() => up(tab.id, sessionId))}
        />
        <BucketBreadcrumb
          bucket={view.bucket}
          prefix={view.prefix}
          onGoTo={(prefix) => act(() => goTo(tab.id, sessionId, prefix))}
        />
      </div>

      <ObjectSearchBar
        search={view.search}
        searching={view.searching}
        scanned={scanned}
        onChange={(edit) => setSearch(tab.id, edit)}
        onRun={() => act(() => runSearch(tab.id, sessionId))}
        onStop={() => act(() => stopSearch(tab.id))}
        onClear={() => act(() => clearSearch(tab.id, sessionId))}
      />

      <div className="min-h-0 flex-1">
        <Split
          id="vibe-db.bucket"
          orientation="vertical"
          panelIds={view.focused === null ? ['rows'] : ['rows', 'object']}
          className="h-full"
        >
          <SplitPanel id="rows" minSize="25%">
            {view.page === null && view.loading ? (
              <Loading label="Reading the folder…" />
            ) : rows.length === 0 ? (
              searched ? (
                <Empty
                  icon={<SearchX size={24} strokeWidth={1.5} />}
                  title="Nothing matches"
                  hint={emptyHint(view)}
                  action={
                    // The one dead end worth a way out of. `Key starts with` is the mode a service can
                    // narrow, so it is the quick one — and it is also the one that finds nothing for a
                    // file name, because a key is a path and the name is at the end of it. Rather than
                    // leave a reader believing their object has gone, the pane names the likely reason
                    // and offers the other mode in one press.
                    view.search.mode === 'prefix' ? (
                      <Button
                        variant="primary"
                        onClick={() => {
                          setSearch(tab.id, { mode: 'contains' })
                          act(() => runSearch(tab.id, sessionId))
                        }}
                      >
                        Look for it anywhere in the key
                      </Button>
                    ) : undefined
                  }
                />
              ) : (
                <Empty
                  icon={<Archive size={24} strokeWidth={1.5} />}
                  title={view.prefix === '' ? 'This bucket is empty' : 'This folder is empty'}
                  hint={
                    takesFiles
                      ? 'Drop files here, or press Upload — either writes into wherever you are standing.'
                      : 'This store is read-only, so nothing can be written into it.'
                  }
                />
              )
            ) : (
              <ResultGrid
                result={grid}
                selection={selection}
                onSelectionChange={showSelected}
                // A header orders what has been read — see `sortRows` for why it is the rows and not
                // the cells that are put in order.
                sort={sort}
                onSortChange={setSort}
                // Reaching the bottom is what asks for the page after this one. A **search's** answer
                // is not paged at all — it is a bounded read of the bucket and says so in the footer
                // — so nothing is asked for while one is on screen.
                onReachEnd={searched ? undefined : () => act(() => more(tab.id, sessionId))}
                onActivateRow={(row) => {
                  const held = rowAt(row)
                  if (held) on.open(held)
                }}
                onRefresh={() => act(() => read(tab.id, sessionId))}
                // The built-in menu is written for a table's rows and nearly none of it means
                // anything over a bucket — see `objectMenuItems`.
                cellMenu={(context) =>
                  objectMenuItems(context, drawnRows(rows, tree), picked, session.rules, {
                    onOpen: on.open,
                    onDownload: on.download,
                    onUpload: on.upload,
                    onNewFolder: on.newFolder,
                    onDelete: on.remove,
                    onRename: on.rename,
                    onCopyLink: on.copyLink,
                    onDetails: on.details,
                  })
                }
              />
            )}
          </SplitPanel>

          {view.focused !== null ? (
            <>
              <SplitHandle orientation="vertical" />
              <SplitPanel id="object" defaultSize="42%" minSize="20%">
                <ObjectPanel
                  sessionId={sessionId}
                  bucket={view.bucket}
                  objectKey={view.focused}
                  headerRule={session.rules.headers}
                  onClose={() => focus(tab.id, null)}
                />
              </SplitPanel>
            </>
          ) : null}
        </Split>
      </div>

      {shown !== null ? <ErrorStrip error={shown} /> : null}

      <GridFooter
        editable={false}
        busy={view.loading || view.searching}
        onRefresh={() => act(() => read(tab.id, sessionId))}
      >
        <span className="flex items-center gap-3">
          {/* What has been read so far, and never a page number: "page 3 of 40" would mean knowing
              how many objects there are, which is a read of the whole bucket. Where more is coming
              the count says so, so a total that climbs as the reader scrolls is expected rather than
              a number that keeps being wrong. */}
          <span>
            {summary(
              view.page?.folders ?? 0,
              rows.length,
              view.page?.bytes ?? 0,
              searched,
              (view.page?.next ?? '') !== '',
              rows.length >= AUTO_ROWS,
            )}
          </span>

          {searched && view.result?.more === true ? (
            <Button variant="subtle" onClick={() => act(() => runSearch(tab.id, sessionId))}>
              There are more — search again
            </Button>
          ) : null}
        </span>
      </GridFooter>
    </div>
  )
}

/**
 * Why a search found nothing, in the order the reasons are worth hearing.
 *
 * The first is the one that catches people: `Key starts with` is matched against the **whole key**, so
 * a file name typed into it matches nothing however certainly the object is there. Saying "0 of
 * 12,000 keys" and stopping would be technically complete and useless.
 */
function emptyHint(view: BucketView): string {
  if (view.search.mode === 'prefix') {
    return `A key is a path, and this mode matches from its start — so “${view.search.text}” finds nothing unless it is the beginning of the whole key, folders and all.`
  }
  if (view.result?.whole === false) {
    return `It stopped after reading ${view.result.scanned.toLocaleString()} keys, so there may be more it did not reach. Open a folder first to search inside it.`
  }
  return `${view.result?.scanned.toLocaleString() ?? 0} keys were read.`
}

/**
 * The rows **as the grid drew them**, which is what a menu raised on grid row *n* has to be given: a
 * tree reorders and adds rows, so the page's own list would name the wrong one. A branch that is no
 * object of its own becomes a folder row, there being no object there to name.
 */
function drawnRows(
  rows: readonly ObjectRow[],
  tree: ReturnType<typeof objectTree> | null,
): ObjectRow[] {
  if (tree === null) return [...rows]

  return tree.map((entry) => {
    const held = entry.source < 0 ? undefined : rows[entry.source]
    return held ?? { ...EMPTY_ROW, key: entry.branch ?? entry.label, name: entry.label, folder: true }
  })
}

const EMPTY_ROW: ObjectRow = {
  key: '',
  name: '',
  folder: false,
  size: 0,
  modifiedAt: '',
  etag: '',
  storageClass: '',
}

/** What the actions are handed while the view is still opening: nothing, refusing nothing. */
const EMPTY_VIEW = {
  bucket: '',
  prefix: '',
  flat: false,
  page: null,
  loading: false,
  search: { text: '', mode: 'prefix', caseInsensitive: false, minSize: 0, maxSize: 0, since: '', until: '' },
  result: null,
  searching: false,
  runId: '',
  picked: [],
  focused: null,
  error: null,
}

const NO_RULE = { verb: '', hazard: '', confirm: false, refused: true }
const NO_RULES = {
  upload: NO_RULE,
  delete: NO_RULE,
  deleteFolder: NO_RULE,
  emptyBucket: NO_RULE,
  deleteBucket: NO_RULE,
  copy: NO_RULE,
  move: NO_RULE,
  createBucket: NO_RULE,
  headers: NO_RULE,
}

/**
 * How far the listing reads on its own before it waits to be scrolled. Fifty pages is a few seconds
 * against a service and about ten megabytes of XML — far more than any real folder needs to have
 * shown every prefix it holds, and short of the read nobody asked for.
 */
const AUTO_ROWS = 50_000

/**
 * What the footer counts: **what has been read**, which with a listing that appends is a number that
 * climbs. A bucket has no size a listing knows, so the tail is the honest half — without it a total
 * that keeps growing would read as a total that keeps being wrong. It says which of the two is
 * happening, because they are different sentences: one is working, the other is waiting.
 */
function summary(
  folders: number,
  rows: number,
  bytes: number,
  searched: boolean,
  more: boolean,
  waiting: boolean,
): string {
  if (searched) return `${rows.toLocaleString()} matches`

  const objects = rows - folders
  const parts: string[] = []
  if (folders > 0) parts.push(`${folders} ${folders === 1 ? 'folder' : 'folders'}`)
  parts.push(`${objects.toLocaleString()} ${objects === 1 ? 'object' : 'objects'}`)
  if (bytes > 0) parts.push(sizeText(bytes))

  const tail = !more ? '' : waiting ? ' · more as you scroll' : ' · still reading'
  return parts.join(' · ') + tail
}
