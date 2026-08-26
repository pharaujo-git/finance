import type { ReactNode } from 'react'
import { EmptyState, ErrorState, type EmptyStateProps } from './EmptyState'
import { SkeletonRows } from './Skeleton'

export interface DataStateProps {
  isLoading: boolean
  isError?: boolean
  error?: Error | null
  isEmpty?: boolean
  onRetry?: () => void
  skeleton?: ReactNode
  empty?: EmptyStateProps
  children: ReactNode
}

/**
 * Single place where the loading / error / empty / ready decision lives, so
 * every list and panel in the app behaves the same way.
 */
export function DataState({
  isLoading,
  isError = false,
  error,
  isEmpty = false,
  onRetry,
  skeleton,
  empty,
  children,
}: DataStateProps) {
  if (isLoading) return <>{skeleton ?? <SkeletonRows />}</>
  if (isError) return <ErrorState message={error?.message} onRetry={onRetry} />
  if (isEmpty && empty) return <EmptyState {...empty} />
  return <>{children}</>
}
