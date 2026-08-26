import { Badge, ColorDot, TagChips } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icon'
import { cn } from '@/lib/cn'
import { formatCurrency, formatDate } from '@/lib/format'
import type { Lookups } from '@/hooks/useLookups'
import type { Transaction, TransactionType } from '@/types'

const TYPE_TONE = {
  income: 'success',
  expense: 'danger',
  transfer: 'info',
} as const

export function signedAmount(
  type: TransactionType,
  amount: number,
): { value: number; className: string } {
  if (type === 'income') {
    return {
      value: amount,
      className: 'text-emerald-600 dark:text-emerald-400',
    }
  }
  if (type === 'expense') {
    return { value: -amount, className: 'text-rose-600 dark:text-rose-400' }
  }
  return { value: amount, className: 'text-slate-600 dark:text-slate-300' }
}

export function TransactionRow({
  transaction,
  currency,
  lookups,
  onEdit,
  onDelete,
}: {
  transaction: Transaction
  currency: string
  lookups: Lookups
  onEdit: (transaction: Transaction) => void
  onDelete: (transaction: Transaction) => void
}) {
  const category = lookups.category(transaction.categoryId)
  const amount = signedAmount(transaction.type, transaction.amount)

  return (
    <tr className="border-b border-slate-100 last:border-0 hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800/50">
      <td className="px-4 py-3 text-xs whitespace-nowrap text-slate-500 dark:text-slate-400">
        {formatDate(transaction.date)}
      </td>
      <td className="px-4 py-3">
        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">
          {transaction.description || '—'}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          <Badge tone={TYPE_TONE[transaction.type]}>{transaction.type}</Badge>
          <TagChips tags={transaction.tags ?? []} />
        </div>
      </td>
      <td className="px-4 py-3 text-sm whitespace-nowrap text-slate-600 dark:text-slate-300">
        {category ? (
          <span className="inline-flex items-center gap-1.5">
            <ColorDot color={category.color} />
            {category.name}
          </span>
        ) : (
          '—'
        )}
      </td>
      <td className="px-4 py-3 text-sm whitespace-nowrap text-slate-600 dark:text-slate-300">
        {lookups.accountName(transaction.accountId)}
        {transaction.type === 'transfer' && transaction.transferAccountId ? (
          <span className="block text-xs text-slate-400 dark:text-slate-500">
            → {lookups.accountName(transaction.transferAccountId)}
          </span>
        ) : null}
      </td>
      <td
        className={cn(
          'px-4 py-3 text-right text-sm font-semibold whitespace-nowrap tabular-nums',
          amount.className,
        )}
      >
        {formatCurrency(amount.value, currency)}
      </td>
      <td className="px-4 py-3 text-right whitespace-nowrap">
        <div className="flex justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            aria-label={`Edit ${transaction.description || 'transaction'}`}
            onClick={() => onEdit(transaction)}
          >
            <Icon name="edit" className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            aria-label={`Delete ${transaction.description || 'transaction'}`}
            onClick={() => onDelete(transaction)}
            className="text-rose-600 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950"
          >
            <Icon name="trash" className="size-4" />
          </Button>
        </div>
      </td>
    </tr>
  )
}
