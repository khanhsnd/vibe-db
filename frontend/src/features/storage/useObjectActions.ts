import { useMemo } from 'react'

import { downloadObject, objects, type ObjectRow, type ObjectRules } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import { useBucketViewsStore, type BucketView } from '@/stores/bucketViews'
import { useObjectStoresStore } from '@/stores/objectStores'
import { useSchemaStore } from '@/stores/schema'
import type { BucketTab } from '@/stores/tabs'
import { useTransfersStore } from '@/stores/transfers'

import { useDownloadStore } from './downloadStore'
import { sizeText } from './objectRows'

/**
 * Everything a reader can do to what a bucket holds, in **one place**, because it is offered two
 * ways: the toolbar over the grid and the right-click menu in it. That is the rule
 * [`ContextMenu`](../../shared/ui/ContextMenu.tsx) already sets — *an action offered both ways is one
 * object drawn one way* — and here it matters more than usual, since what a delete *confirms* and
 * what it re-reads must not differ between a button and a menu.
 *
 * Every action reads the **same `ObjectRules`** the session arrived with: whether it has to be
 * answered for and whether it will be sent at all are `domain`'s answers, travelling with the
 * connection, so nothing here compares an environment against anything.
 */
export interface ObjectActionSet {
  open: (row: ObjectRow) => void
  details: (row: ObjectRow) => void
  download: () => void
  upload: () => void
  newFolder: () => void
  remove: () => void
  rename: (row: ObjectRow) => void
  copyLink: (row: ObjectRow) => void
}

export function useObjectActions(
  tab: BucketTab,
  view: BucketView,
  rules: ObjectRules,
  picked: readonly ObjectRow[],
  report: (failure: AppError | null) => void,
): ObjectActionSet {
  const sessionId = useObjectStoresStore((s) => s.sessions[tab.profileId]?.sessionId ?? '')
  const ask = useSchemaStore((s) => s.ask)
  const askDownload = useDownloadStore((s) => s.ask)
  const showTransfers = useTransfersStore((s) => s.show)
  const watchTransfers = useTransfersStore((s) => s.watch)
  const read = useBucketViewsStore((s) => s.read)
  const goTo = useBucketViewsStore((s) => s.goTo)
  const focus = useBucketViewsStore((s) => s.focus)

  return useMemo(() => {
    const failed = (thrown: unknown) => report(toAppError(thrown))
    const ticked = picked.filter((row) => !row.folder)
    const prefixes = picked.filter((row) => row.folder).map((row) => row.key)
    const reread = () => read(tab.id, sessionId)

    return {
      open: (row) => {
        report(null)
        if (row.folder) void goTo(tab.id, sessionId, row.key).catch(failed)
        else focus(tab.id, row.key)
      },

      details: (row) => {
        report(null)
        focus(tab.id, row.key)
      },

      download: () => {
        report(null)
        askDownload({
          sessionId,
          bucket: view.bucket,
          under: view.prefix,
          keys: ticked.map(downloadObject),
          prefixes,
          label: describe(picked),
        })
      },

      upload: () => {
        report(null)
        void objects
          .chooseFiles('Files to upload')
          .then(async (paths) => {
            if (paths.length === 0) return
            await objects.upload(sessionId, {
              bucket: view.bucket,
              prefix: view.prefix,
              paths,
            })
            watchTransfers()
            showTransfers(true)
          })
          .catch(failed)
      },

      newFolder: () => {
        report(null)
        ask({
          kind: 'prompt',
          title: 'New Folder',
          label: 'Folder name',
          value: '',
          submitLabel: 'Create',
          onSubmit: async (name) => {
            await objects.newFolder(sessionId, view.bucket, view.prefix, name)
            await reread()
          },
        })
      },

      remove: () => {
        report(null)
        // A folder names *nothing* — it stands for everything under it — which is why `domain` calls
        // it unfiltered and why the two are different rules and different sentences.
        const rule = prefixes.length > 0 ? rules.deleteFolder : rules.delete
        const run = async () => {
          for (const prefix of prefixes) {
            await objects.removeFolder(sessionId, view.bucket, prefix, `delete-${tab.id}-${prefix}`)
          }
          const keys = ticked.map((row) => row.key)
          if (keys.length > 0) await objects.remove(sessionId, view.bucket, keys)
          await reread()
        }

        if (!rule.confirm) {
          void run().catch(failed)
          return
        }

        ask({
          kind: 'confirm',
          title: rule.verb,
          message: removalMessage(picked, prefixes.length > 0),
          submitLabel: rule.verb,
          danger: true,
          onSubmit: run,
        })
      },

      rename: (row) => {
        report(null)
        ask({
          kind: 'prompt',
          title: 'Rename Object',
          label: 'New name',
          value: row.name,
          submitLabel: 'Rename',
          onSubmit: async (name) => {
            const clean = name.trim()
            if (clean === '' || clean === row.name) return
            // The key keeps its folder: a rename here is the name and never the path, which is what
            // Move is for — and a name with a slash typed into it would be a move nobody asked for.
            const to = view.prefix + clean.replace(/\//g, '_')
            await objects.move(sessionId, view.bucket, row.key, to)
            await reread()
          },
        })
      },

      copyLink: (row) => {
        report(null)
        void objects
          .presign(sessionId, view.bucket, row.key, 0)
          .then((link) => navigator.clipboard.writeText(link))
          .catch(failed)
      },
    }
  }, [
    tab.id,
    sessionId,
    view.bucket,
    view.prefix,
    picked,
    rules,
    ask,
    askDownload,
    showTransfers,
    watchTransfers,
    read,
    goTo,
    focus,
    report,
  ])
}

/** What is picked, in the reader's words: the one thing, or how many and what they come to. */
export function describe(picked: readonly ObjectRow[]): string {
  if (picked.length === 0) return ''
  const first = picked[0]
  if (picked.length === 1 && first) return first.name

  const folders = picked.filter((row) => row.folder).length
  const bytes = picked.reduce((total, row) => total + row.size, 0)
  const parts: string[] = []
  if (folders > 0) parts.push(`${folders} ${folders === 1 ? 'folder' : 'folders'}`)

  const objects = picked.length - folders
  if (objects > 0) parts.push(`${objects} ${objects === 1 ? 'object' : 'objects'}`)
  if (bytes > 0) parts.push(sizeText(bytes))
  return parts.join(' · ')
}

/**
 * What a delete is answered for with. A **folder** is named as the thing it is — everything under it,
 * however much that turns out to be — because that is the whole reason it is confirmed on a laptop as
 * well as on production.
 */
function removalMessage(picked: readonly ObjectRow[], folders: boolean): string {
  const what = describe(picked)
  if (folders) {
    return `${what} will be deleted, and so will everything inside it — however many objects that turns out to be. A bucket does not say how many until they have been read, and none of it can be undone.`
  }
  return `${what} will be deleted from this bucket. It cannot be undone.`
}
