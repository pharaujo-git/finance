import type { ReactNode } from 'react'
import { Icon, type IconName } from './Icon'

export interface EmptyStateProps {
  icon?: IconName
  title: string
  description?: string
  action?: ReactNode
}

export function EmptyState({
  icon = 'inbox',
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-12 text-center">
      <span className="rounded-full bg-slate-100 p-3 text-slate-400 dark:bg-slate-800 dark:text-slate-500">
        <Icon name={icon} className="size-6" />
      </span>
      <div>
        <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
          {title}
        </p>
        {description ? (
          <p className="mt-1 max-w-sm text-xs text-slate-500 dark:text-slate-400">
            {description}
          </p>
        ) : null}
      </div>
      {action}
    </div>
  )
}

export function ErrorState({
  message,
  onRetry,
}: {
  message?: string
  onRetry?: () => void
}) {
  return (
    <EmptyState
      icon="alert"
      title="Something went wrong"
      description={message ?? 'We could not load this data. Please try again.'}
      action={
        onRetry ? (
          <button
            type="button"
            onClick={onRetry}
            className="rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            Try again
          </button>
        ) : null
      }
    />
  )
}
