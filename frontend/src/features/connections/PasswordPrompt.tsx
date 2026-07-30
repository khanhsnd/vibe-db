import { useEffect, useState } from 'react'

import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Field, TextInput } from '@/shared/ui/Field'

interface PasswordPromptProps {
  open: boolean
  target: string
  onOpenChange: (open: boolean) => void
  onSubmit: (password: string) => void
}

export function PasswordPrompt({ open, target, onOpenChange, onSubmit }: PasswordPromptProps) {
  const [password, setPassword] = useState('')

  useEffect(() => {
    if (open) setPassword('')
  }, [open])

  const submit = () => {
    onOpenChange(false)
    onSubmit(password)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Password required"
      description={target}
      width={360}
      footer={
        <>
          <Button variant="default" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit}>
            Connect
          </Button>
        </>
      }
    >
      <Field label="Password">
        {(id) => (
          <TextInput
            id={id}
            type="password"
            autoFocus
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />
        )}
      </Field>
    </Dialog>
  )
}
