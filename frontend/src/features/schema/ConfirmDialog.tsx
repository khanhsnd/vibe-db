import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import type { ConfirmRequest } from '@/stores/schema'

import { useSchemaSubmit } from './useSchemaSubmit'

/** The one gate in front of a change that cannot be undone: dropping, emptying, truncating. */
export function ConfirmDialog({
  request,
  onClose,
}: {
  request: ConfirmRequest
  onClose: () => void
}) {
  const { submit, running, error } = useSchemaSubmit(onClose)

  return (
    <Dialog
      open
      onOpenChange={onClose}
      title={request.title}
      width={420}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant={request.danger ? 'danger' : 'primary'}
            disabled={running}
            onClick={() => submit(request.onSubmit)}
          >
            {running ? 'Working…' : request.submitLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <p className="selectable text-sm text-fg">{request.message}</p>
        {error ? <Banner tone="danger" text={error.message} /> : null}
      </div>
    </Dialog>
  )
}
