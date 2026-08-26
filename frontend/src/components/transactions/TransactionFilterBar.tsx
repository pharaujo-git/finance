import { Button } from '@/components/ui/Button'
import { Input, Select } from '@/components/ui/Field'
import type { Lookups } from '@/hooks/useLookups'
import type { TransactionFilters } from '@/types'

export const EMPTY_FILTERS: TransactionFilters = {
  accountId: '',
  categoryId: '',
  type: '',
  from: '',
  to: '',
  search: '',
}

export function hasActiveFilters(filters: TransactionFilters): boolean {
  return Boolean(
    filters.accountId ||
      filters.categoryId ||
      filters.type ||
      filters.from ||
      filters.to ||
      filters.search,
  )
}

export function TransactionFilterBar({
  filters,
  lookups,
  onChange,
  onReset,
}: {
  filters: TransactionFilters
  lookups: Lookups
  onChange: (patch: Partial<TransactionFilters>) => void
  onReset: () => void
}) {
  return (
    <div className="grid gap-3 border-b border-slate-200 p-4 sm:grid-cols-2 lg:grid-cols-6 dark:border-slate-800">
      <Input
        label="Search"
        type="search"
        placeholder="Description or notes"
        value={filters.search ?? ''}
        onChange={(event) => onChange({ search: event.target.value })}
        wrapperClassName="sm:col-span-2"
      />
      <Select
        label="Account"
        value={filters.accountId ?? ''}
        onChange={(event) => onChange({ accountId: event.target.value })}
      >
        <option value="">All accounts</option>
        {lookups.accounts.map((account) => (
          <option key={account.id} value={account.id}>
            {account.name}
          </option>
        ))}
      </Select>
      <Select
        label="Category"
        value={filters.categoryId ?? ''}
        onChange={(event) => onChange({ categoryId: event.target.value })}
      >
        <option value="">All categories</option>
        {lookups.categories.map((category) => (
          <option key={category.id} value={category.id}>
            {category.name}
          </option>
        ))}
      </Select>
      <Select
        label="Type"
        value={filters.type ?? ''}
        onChange={(event) =>
          onChange({ type: event.target.value as TransactionFilters['type'] })
        }
      >
        <option value="">All types</option>
        <option value="income">Income</option>
        <option value="expense">Expense</option>
        <option value="transfer">Transfer</option>
      </Select>
      <div className="flex items-end">
        <Button
          variant="secondary"
          size="sm"
          onClick={onReset}
          disabled={!hasActiveFilters(filters)}
          className="w-full"
        >
          Clear filters
        </Button>
      </div>
      <Input
        label="From"
        type="date"
        value={filters.from ?? ''}
        onChange={(event) => onChange({ from: event.target.value })}
      />
      <Input
        label="To"
        type="date"
        value={filters.to ?? ''}
        onChange={(event) => onChange({ to: event.target.value })}
      />
    </div>
  )
}
