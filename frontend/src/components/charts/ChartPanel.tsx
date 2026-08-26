import type { ReactElement, ReactNode } from 'react'
import { ResponsiveContainer } from 'recharts'
import { Card, CardHeader } from '@/components/ui/Card'
import { DataState } from '@/components/ui/DataState'
import { Skeleton } from '@/components/ui/Skeleton'
import type { EmptyStateProps } from '@/components/ui/EmptyState'

export interface ChartPanelProps {
  title: string
  subtitle?: string
  action?: ReactNode
  height?: number
  isLoading: boolean
  isError?: boolean
  error?: Error | null
  isEmpty?: boolean
  empty: EmptyStateProps
  onRetry?: () => void
  children: ReactElement
}

/** Card + header + loading/error/empty handling shared by every chart. */
export function ChartPanel({
  title,
  subtitle,
  action,
  height = 280,
  isLoading,
  isError,
  error,
  isEmpty,
  empty,
  onRetry,
  children,
}: ChartPanelProps) {
  return (
    <Card>
      <CardHeader title={title} subtitle={subtitle} action={action} />
      <div className="p-4 text-slate-500 dark:text-slate-400">
        <DataState
          isLoading={isLoading}
          isError={isError}
          error={error}
          isEmpty={isEmpty}
          empty={empty}
          onRetry={onRetry}
          skeleton={<Skeleton className="w-full" style={{ height }} />}
        >
          <div style={{ height }}>
            <ResponsiveContainer width="100%" height="100%">
              {children}
            </ResponsiveContainer>
          </div>
        </DataState>
      </div>
    </Card>
  )
}
