import { useId, type ReactNode } from 'react'
import type {
  InputHTMLAttributes,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from 'react'
import { cn } from '@/lib/cn'

interface FieldShellProps {
  label: string
  error?: string
  hint?: string
  htmlFor: string
  className?: string
  children: ReactNode
}

function FieldShell({
  label,
  error,
  hint,
  htmlFor,
  className,
  children,
}: FieldShellProps) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label
        htmlFor={htmlFor}
        className="text-xs font-medium text-slate-600 dark:text-slate-300"
      >
        {label}
      </label>
      {children}
      {error ? (
        <p role="alert" className="text-xs text-rose-600 dark:text-rose-400">
          {error}
        </p>
      ) : hint ? (
        <p className="text-xs text-slate-500 dark:text-slate-400">{hint}</p>
      ) : null}
    </div>
  )
}

type Common = { label: string; error?: string; hint?: string; wrapperClassName?: string }

export type InputProps = Common &
  Omit<InputHTMLAttributes<HTMLInputElement>, 'id'>

export function Input({
  label,
  error,
  hint,
  wrapperClassName,
  className,
  ...rest
}: InputProps) {
  const id = useId()
  return (
    <FieldShell
      label={label}
      error={error}
      hint={hint}
      htmlFor={id}
      className={wrapperClassName}
    >
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        className={cn('field-control', error && 'field-control-invalid', className)}
        {...rest}
      />
    </FieldShell>
  )
}

export type SelectProps = Common &
  Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> & {
    children: ReactNode
  }

export function Select({
  label,
  error,
  hint,
  wrapperClassName,
  className,
  children,
  ...rest
}: SelectProps) {
  const id = useId()
  return (
    <FieldShell
      label={label}
      error={error}
      hint={hint}
      htmlFor={id}
      className={wrapperClassName}
    >
      <select
        id={id}
        aria-invalid={error ? true : undefined}
        className={cn('field-control', error && 'field-control-invalid', className)}
        {...rest}
      >
        {children}
      </select>
    </FieldShell>
  )
}

export type TextareaProps = Common &
  Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'id'>

export function Textarea({
  label,
  error,
  hint,
  wrapperClassName,
  className,
  ...rest
}: TextareaProps) {
  const id = useId()
  return (
    <FieldShell
      label={label}
      error={error}
      hint={hint}
      htmlFor={id}
      className={wrapperClassName}
    >
      <textarea
        id={id}
        aria-invalid={error ? true : undefined}
        className={cn('field-control', error && 'field-control-invalid', className)}
        {...rest}
      />
    </FieldShell>
  )
}

export function Checkbox({
  label,
  className,
  ...rest
}: { label: string } & Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>) {
  const id = useId()
  return (
    <div className={cn('flex items-center gap-2', className)}>
      <input
        id={id}
        type="checkbox"
        className="size-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500 dark:border-slate-600 dark:bg-slate-900"
        {...rest}
      />
      <label
        htmlFor={id}
        className="text-sm text-slate-700 dark:text-slate-300"
      >
        {label}
      </label>
    </div>
  )
}
