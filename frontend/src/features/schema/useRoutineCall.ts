import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { AppError, toAppError } from '@/bridge/errors'
import { schema, type CallResult, type RoutineArg, type SourceRef } from '@/bridge/schema'

/** One routine call as the Run pane holds it: what it takes, what will run, and what it answered. */
export interface RoutineCall {
  /** What the routine takes, with the reader's values. `null` while the catalogue is being read. */
  args: RoutineArg[] | null
  /** Exactly what `run` will send, in order — `PreviewCall`, which is what `CallRoutine` runs. */
  statements: string[]
  answer: CallResult | null
  running: boolean
  error: AppError | null
  setArg: (at: number, value: string) => void
  /** Re-reads what the routine takes, keeping whatever values still belong to it. */
  reload: () => Promise<void>
  run: () => Promise<void>
}

/**
 * The Run pane's whole state, held **here** rather than in the component that draws it: only one
 * pane of a definition tab is mounted at a time, so a reader who looks at the definition and comes
 * back would otherwise find their arguments blank and the answer gone.
 *
 * Nothing here spells a statement. What the routine takes is a catalogue read, what will run is
 * `PreviewCall`, and `run` is `CallRoutine` running exactly that — rule 13, on the one surface
 * where what runs is the reader's own program.
 */
export function useRoutineCall(
  sessionId: string,
  namespace: Namespace,
  routine: SourceRef,
  /** False for a definition that is not called at all — a view, a trigger, an object being made. */
  enabled: boolean,
): RoutineCall {
  const [args, setArgs] = useState<RoutineArg[] | null>(null)
  const [statements, setStatements] = useState<string[]>([])
  const [answer, setAnswer] = useState<CallResult | null>(null)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  // The parameters are read from the catalogue and never out of the definition beside them: a body
  // is text, and reading a parameter list out of one would be a second parser on the wrong side of
  // the wire.
  //
  // A re-read **keeps the values that still belong to the routine**, matched by name, because the
  // commonest reason to re-read is a Save that changed the body rather than the signature — and
  // clearing the boxes on every Save would make testing a change a retyping exercise.
  const reload = useCallback(async () => {
    if (!enabled) return

    setError(null)
    try {
      const params = await schema.routineParams(sessionId, namespace, routine)
      setArgs((held) =>
        params.map((param) => {
          const kept = held?.find((arg) => arg.name === param.name && arg.mode === param.mode)
          return { ...param, value: param.mode === 'out' ? null : (kept?.value ?? '') }
        }),
      )
    } catch (thrown) {
      setError(toAppError(thrown))
      setArgs([])
    }
  }, [enabled, sessionId, namespace, routine])

  useEffect(() => {
    void reload()
  }, [reload])

  // The preview follows the boxes, so what is shown is always what Run would send — the variable an
  // OUT parameter is handed back in included, which is a statement the reader never typed.
  useEffect(() => {
    if (args === null) return

    let current = true
    schema
      .previewCall(sessionId, namespace, routine, args)
      .then((planned) => {
        if (current) setStatements(planned)
      })
      .catch(() => {
        if (current) setStatements([])
      })

    return () => {
      current = false
    }
  }, [sessionId, namespace, routine, args])

  // An OUT parameter takes no value, so a write to its cell is ignored rather than kept: what the
  // reader typed there would otherwise be spelled into the call as the variable's starting value,
  // which is not what the cell says it is.
  const setArg = useCallback((at: number, value: string) => {
    setArgs(
      (held) =>
        held?.map((arg, index) =>
          index === at && arg.mode !== 'out' ? { ...arg, value } : arg,
        ) ?? held,
    )
  }, [])

  const run = useCallback(async () => {
    if (args === null) return

    setRunning(true)
    setError(null)
    try {
      setAnswer(await schema.callRoutine(sessionId, namespace, routine, args))
    } catch (thrown) {
      setError(toAppError(thrown))
      setAnswer(null)
    } finally {
      setRunning(false)
    }
  }, [sessionId, namespace, routine, args])

  return { args, statements, answer, running, error, setArg, reload, run }
}
