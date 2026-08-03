import { Check, TriangleAlert } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { ICON } from '@/shared/ui/menuIcons'
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
      icon={request.danger ? <TriangleAlert size={15} /> : <Check size={15} />}
      tone={request.danger ? 'danger' : 'accent'}
      width={420}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant={request.danger ? 'danger' : 'primary'}
            icon={request.danger ? ICON.delete : <Check size={13} />}
            disabled={running}
            onClick={() => submit(request.onSubmit)}
          >
            {running ? 'Working…' : request.submitLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        {/* What is about to happen sits in a block of its own tone, so the weight of the answer
            is readable before the wording is. */}
        <p
          className={cn(
            'selectable rounded-md border px-3 py-2.5 text-sm text-fg',
            request.danger ? 'border-danger/40 bg-danger-subtle' : 'border-line bg-bar',
          )}
        >
          {request.message}
        </p>
        {error ? <Banner tone="danger" text={error.message} /> : null}
      </div>
    </Dialog>
  )
}
