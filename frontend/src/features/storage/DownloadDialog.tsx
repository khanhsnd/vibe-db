import { useState } from 'react'
import { Download, FolderOpen } from 'lucide-react'

import { COLLISIONS, objects } from '@/bridge/objects'
import { toAppError, type AppError } from '@/bridge/errors'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Field, SelectInput, TextInput } from '@/shared/ui/Field'
import { useTransfersStore } from '@/stores/transfers'

import { useDownloadStore } from './downloadStore'

/**
 * Where a download goes and what it does about a file that is already there. Two boxes, and both of
 * them are answers this app must not guess at: writing over somebody's file is not a default, and
 * neither is the folder it goes into.
 *
 * It answers **at once**. The queue is handed the work and the panel is what watches it, so a folder
 * of a million objects does not hold this dialog open while its listing runs — see
 * `docs/features/object-download.md`.
 */
export function DownloadDialog() {
  const request = useDownloadStore((s) => s.request)
  const into = useDownloadStore((s) => s.into)
  const collision = useDownloadStore((s) => s.collision)
  const dismiss = useDownloadStore((s) => s.dismiss)
  const setInto = useDownloadStore((s) => s.setInto)
  const setCollision = useDownloadStore((s) => s.setCollision)
  const show = useTransfersStore((s) => s.show)
  const watch = useTransfersStore((s) => s.watch)

  const [failure, setFailure] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)

  const pick = () => {
    setFailure(null)
    void objects
      .chooseFolder('Download into')
      .then((chosen) => {
        if (chosen !== '') setInto(chosen)
      })
      .catch((thrown: unknown) => setFailure(toAppError(thrown)))
  }

  const start = () => {
    if (!request || into === '') return
    setBusy(true)
    setFailure(null)

    void objects
      .download(request.sessionId, {
        bucket: request.bucket,
        keys: request.keys,
        prefixes: request.prefixes,
        into,
        under: request.under,
        collision,
      })
      .then(() => {
        watch()
        show(true)
        dismiss()
      })
      .catch((thrown: unknown) => setFailure(toAppError(thrown)))
      .finally(() => setBusy(false))
  }

  return (
    <Dialog
      open={request !== null}
      onOpenChange={(open) => {
        if (!open) dismiss()
      }}
      title="Download"
      description={request?.label ?? ''}
      icon={<Download size={15} />}
      width={520}
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button onClick={dismiss}>Cancel</Button>
          <Button variant="primary" disabled={into === '' || busy} onClick={start}>
            Download
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label="Into this folder">
          {(id) => (
            <div className="flex items-center gap-1.5">
              <TextInput
                id={id}
                value={into}
                readOnly
                placeholder="Choose a folder on this machine"
                className="min-w-0 flex-1"
              />
              <Button icon={<FolderOpen size={13} />} onClick={pick}>
                Browse
              </Button>
            </div>
          )}
        </Field>

        <Field
          label="If a file is already there"
          hint={COLLISIONS.find((one) => one.value === collision)?.hint ?? ''}
        >
          {(id) => (
            <SelectInput id={id} value={collision} onChange={(e) => setCollision(e.target.value)}>
              {COLLISIONS.map((one) => (
                <option key={one.value} value={one.value}>
                  {one.label}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>

        {/* The one thing a reader should know before pressing it, and it is not a warning: a folder
            keeps its shape, so nothing lands in a heap. */}
        <p className="text-xs text-muted">
          Folders keep the shape they have in the bucket. A large object is read in several ranges at
          once and picks up where it left off if the download is interrupted.
        </p>

        {failure !== null ? <ErrorStrip error={failure} /> : null}
      </div>
    </Dialog>
  )
}
