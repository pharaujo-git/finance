import { Button } from './Button'
import { Icon } from './Icon'

export interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}

export function totalPages(total: number, pageSize: number): number {
  if (pageSize <= 0) return 1
  return Math.max(1, Math.ceil(total / pageSize))
}

export function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
}: PaginationProps) {
  const pages = totalPages(total, pageSize)
  const first = total === 0 ? 0 : (page - 1) * pageSize + 1
  const last = Math.min(page * pageSize, total)

  return (
    <div className="flex flex-col items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 sm:flex-row dark:border-slate-800">
      <p className="text-xs text-slate-500 dark:text-slate-400">
        Showing <span className="font-medium">{first}</span>–
        <span className="font-medium">{last}</span> of{' '}
        <span className="font-medium">{total}</span>
      </p>
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          icon={<Icon name="chevronLeft" className="size-3.5" />}
        >
          Previous
        </Button>
        <span className="text-xs text-slate-500 dark:text-slate-400">
          Page {page} of {pages}
        </span>
        <Button
          size="sm"
          variant="secondary"
          disabled={page >= pages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
          <Icon name="chevronRight" className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}
