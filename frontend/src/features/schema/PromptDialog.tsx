import { useState } from 'react'

import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Field, TextInput } from '@/shared/ui/Field'
import type { PromptRequest } from '@/stores/schema'

import { useSchemaSubmit } from './useSchemaSubmit'

/** One name, asked for: what every rename in the tree comes down to. */
export function PromptDialog({
  request,
  onClose,
}: {
  request: PromptRequest
  onClose: () => void
}) {
  const [value, setValue] = useState(request.value)
  const { submit, running, error } = useSchemaSubmit(onClose)
  const apply = () => submit(() => request.onSubmit(value))

  return (
    <Dialog
      open
      onOpenChange={onClose}
      title={request.title}
      width={400}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={running} onClick={apply}>
            {running ? 'Applying…' : request.submitLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label={request.label} error={error?.code === 'INVALID_NAME' ? error.message : undefined}>
          {(id, invalid) => (
            <TextInput
              id={id}
              invalid={invalid}
              value={value}
              autoFocus
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void apply()
              }}
            />
          )}
        </Field>
        {error && error.code !== 'INVALID_NAME' ? (
          <Banner tone="danger" text={error.message} />
        ) : null}
      </div>
    </Dialog>
  )
}
