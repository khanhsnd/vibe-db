import { useCallback, useState } from 'react'

import { AppError, toAppError } from '@/bridge/errors'

/**
 * What every schema dialog does with its button: run the change, close on success, and keep the
 * failure on screen so the reader can correct it rather than lose what they typed.
 */
export function useSchemaSubmit(onDone: () => void) {
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const submit = useCallback(
    async (change: () => Promise<void>) => {
      setRunning(true)
      setError(null)
      try {
        await change()
        onDone()
      } catch (thrown) {
        setError(toAppError(thrown))
      } finally {
        setRunning(false)
      }
    },
    [onDone],
  )

  // A blank name is reported against the box it was missing from, so the message sits under
  // that box instead of in the banner.
  const fieldError = useCallback(
    (name: string) =>
      error?.code === 'INVALID_NAME' && error.field === name ? error.message : undefined,
    [error],
  )

  return { submit, running, error, fieldError }
}
