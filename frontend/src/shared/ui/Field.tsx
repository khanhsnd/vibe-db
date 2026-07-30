import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

const CONTROL = [
  'h-8 w-full rounded-md border bg-canvas px-2.5 text-sm text-fg outline-none',
  'placeholder:text-faint',
  'disabled:opacity-50',
].join(' ')

interface FieldProps {
  label: string
  hint?: string | undefined
  error?: string | undefined
  children: (controlId: string, invalid: boolean) => ReactNode
}

export function Field({ label, hint, error, children }: FieldProps) {
  const controlId = useId()
  const invalid = Boolean(error)

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={controlId} className="text-xs font-medium text-muted">
        {label}
      </label>
      {children(controlId, invalid)}
      {error ? (
        <span className="text-xs text-danger">{error}</span>
      ) : hint ? (
        <span className="text-xs text-faint">{hint}</span>
      ) : null}
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
