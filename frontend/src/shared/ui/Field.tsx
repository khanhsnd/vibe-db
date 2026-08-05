import {
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react'

import { cn } from '@/lib/cn'

// The height is kept out of the shared part, because `cn` only joins class names — two height
// utilities on one element would be settled by the stylesheet's order rather than by this file's.
const CONTROL_BASE = [
  'w-full rounded-md border bg-canvas px-2.5 text-sm text-fg outline-none',
  'placeholder:text-faint',
  'disabled:opacity-50',
].join(' ')

const CONTROL = `h-8 ${CONTROL_BASE}`

/** A control sized for a toolbar strip rather than a form: shorter, tighter, never full width. */
export const TOOLBAR_CONTROL = [
  'h-6 rounded-md border border-line bg-canvas px-1.5 text-xs text-fg outline-none',
  'focus:border-accent',
  'disabled:opacity-45',
].join(' ')

interface FieldProps {
  label: string
  hint?: string | undefined
  error?: string | undefined
  /** Puts the label beside the control rather than over it, which is how a settings form reads. */
  inline?: boolean
  children: (controlId: string, invalid: boolean) => ReactNode
}

/** The width the labels of an inline form share, so their controls line up down the column. */
const INLINE_LABEL = '11rem'

export function Field({ label, hint, error, inline = false, children }: FieldProps) {
  const controlId = useId()
  const invalid = Boolean(error)

  // Selectable, because a rejection under a box is a rejection the reader may want to quote.
  const note = error ? (
    <span className="selectable text-xs text-danger">{error}</span>
  ) : hint ? (
    <span className="text-xs text-faint">{hint}</span>
  ) : null

  if (inline) {
    return (
      <div
        className="grid items-center gap-x-3 gap-y-1"
        style={{ gridTemplateColumns: `${INLINE_LABEL} minmax(0, 1fr)` }}
      >
        <label htmlFor={controlId} className="text-sm text-fg">
          {label}
        </label>
        {children(controlId, invalid)}
        {note ? <div className="col-start-2">{note}</div> : null}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={controlId} className="text-xs font-medium text-muted">
        {label}
      </label>
      {children(controlId, invalid)}
      {note}
    </div>
  )
}

interface TextInputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}

export function TextInput({ invalid, className, ...rest }: TextInputProps) {
  return (
    <input
      spellCheck={false}
      className={cn(
        CONTROL,
        invalid ? 'border-danger focus:border-danger' : 'border-line focus:border-accent',
        className,
      )}
      {...rest}
    />
  )
}

interface SelectInputProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean
}

export function SelectInput({ invalid, className, children, ...rest }: SelectInputProps) {
  return (
    <select
      className={cn(
        CONTROL,
        invalid ? 'border-danger focus:border-danger' : 'border-line focus:border-accent',
        className,
      )}
      {...rest}
    >
      {children}
    </select>
  )
}

interface TextAreaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean
}

/** Free text with room to be read back: a comment, a description — never a single value. */
export function TextArea({ invalid, className, ...rest }: TextAreaProps) {
  return (
    <textarea
      spellCheck={false}
      className={cn(
        CONTROL_BASE,
        'resize-none py-1.5 leading-relaxed',
        invalid ? 'border-danger focus:border-danger' : 'border-line focus:border-accent',
        className,
      )}
      {...rest}
    />
  )
}

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string
}

export function Checkbox({ label, className, ...rest }: CheckboxProps) {
  const controlId = useId()

  return (
    <div className="flex items-center gap-2">
      <input
        id={controlId}
        type="checkbox"
        className={cn('h-3.5 w-3.5 accent-[var(--accent)]', className)}
        {...rest}
      />
      <label htmlFor={controlId} className="text-sm text-fg">
        {label}
      </label>
    </div>
  )
}
