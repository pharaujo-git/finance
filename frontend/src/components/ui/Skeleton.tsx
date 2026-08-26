import type { CSSProperties } from 'react'
import { cn } from '@/lib/cn'

export function Skeleton({
  className,
  style,
}: {
  className?: string
  style?: CSSProperties
}) {
  return (
    <div
      data-testid="skeleton"
      style={style}
      className={cn(
        'animate-pulse rounded-md bg-slate-200 dark:bg-slate-800',
        className,
      )}
    />
  )
}

export function SkeletonRows({
  rows = 5,
  className,
}: {
  rows?: number
  className?: string
}) {
  return (
    <div className={cn('space-y-2', className)}>
      {Array.from({ length: rows }, (_, index) => (
        <Skeleton key={index} className="h-10 w-full" />
      ))}
    </div>
  )
}

export function SkeletonCards({ count = 4 }: { count?: number }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {Array.from({ length: count }, (_, index) => (
        <Skeleton key={index} className="h-28 w-full" />
      ))}
    </div>
  )
}
