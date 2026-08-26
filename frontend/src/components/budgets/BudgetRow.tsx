import { Badge, ColorDot } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icon'
import { ProgressBar } from '@/components/ui/ProgressBar'
import { cn } from '@/lib/cn'
import { formatCurrency } from '@/lib/format'
import type { Budget, Category } from '@/types'

export interface BudgetStatus {
  isOver: boolean
  isNearLimit: boolean
  tone: 'danger' | 'warning' | 'success'
  label: string
}

/** Shared budget health rules used by the budgets page and the dashboard. */
export function budgetStatus(budget: Budget): BudgetStatus {
  const isOver = budget.limit > 0 && budget.spent > budget.limit
  const ratio = budget.limit > 0 ? budget.spent / budget.limit : 0
  const isNearLimit = !isOver && ratio >= 0.85
  if (isOver) return { isOver, isNearLimit, tone: 'danger', label: 'over budget' }
  if (isNearLimit)
    return { isOver, isNearLimit, tone: 'warning', label: 'almost there' }
  return { isOver, isNearLimit, tone: 'success', label: 'on track' }
}

export function BudgetRow({
  budget,
  category,
  currency,
  onEdit,
  onDelete,
}: {
  budget: Budget
  category: Category | undefined
  currency: string
  onEdit?: (budget: Budget) => void
  onDelete?: (budget: Budget) => void
}) {
  const status = budgetStatus(budget)

  return (
    <li
      className={cn(
        'px-4 py-4',
        status.isOver && 'bg-rose-50/60 dark:bg-rose-950/30',
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex items-center gap-2 text-sm font-medium text-slate-900 dark:text-slate-100">
          <ColorDot color={category?.color ?? '#94a3b8'} />
          {category?.name ?? 'Unknown category'}
          <Badge tone={status.tone}>{status.label}</Badge>
        </span>
        <span className="text-sm tabular-nums text-slate-600 dark:text-slate-300">
          <span
            className={cn(
              'font-semibold',
              status.isOver && 'text-rose-600 dark:text-rose-400',
            )}
          >
            {formatCurrency(budget.spent, currency)}
          </span>{' '}
          / {formatCurrency(budget.limit, currency)}
        </span>
      </div>

      <ProgressBar
        className="mt-2.5"
        value={budget.spent}
        max={budget.limit}
        warnOnOverflow
        color={category?.color}
        label={`${category?.name ?? 'Budget'} usage`}
      />

      <div className="mt-2 flex items-center justify-between">
        <span
          className={cn(
            'text-xs',
            status.isOver
              ? 'text-rose-600 dark:text-rose-400'
              : 'text-slate-500 dark:text-slate-400',
          )}
        >
          {status.isOver
            ? `${formatCurrency(budget.spent - budget.limit, currency)} over`
            : `${formatCurrency(budget.remaining, currency)} left`}
        </span>
        {onEdit && onDelete ? (
          <div className="flex gap-1">
            <Button
              size="sm"
              variant="ghost"
              aria-label={`Edit budget for ${category?.name ?? 'category'}`}
              onClick={() => onEdit(budget)}
            >
              <Icon name="edit" className="size-4" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              aria-label={`Delete budget for ${category?.name ?? 'category'}`}
              onClick={() => onDelete(budget)}
              className="text-rose-600 dark:text-rose-400"
            >
              <Icon name="trash" className="size-4" />
            </Button>
          </div>
        ) : null}
      </div>
    </li>
  )
}
