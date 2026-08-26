import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

export function Card({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return <div className={cn('card', className)}>{children}</div>
}

export function CardHeader({
  title,
  subtitle,
  action,
}: {
  title: string
  subtitle?: string
  action?: ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-slate-200 px-5 py-4 dark:border-slate-800">
      <div>
        <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
          {title}
        </h2>
        {subtitle ? (
          <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            {subtitle}
          </p>
        ) : null}
      </div>
      {action}
    </div>
  )
}

export function CardBody({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return <div className={cn('p-5', className)}>{children}</div>
}
