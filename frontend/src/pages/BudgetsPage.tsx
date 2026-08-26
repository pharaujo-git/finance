import { useMemo, useState } from 'react'
import {
  useBudgets,
  useCreateBudget,
  useDeleteBudget,
  useUpdateBudget,
} from '@/api/queries'
import { BudgetForm } from '@/components/budgets/BudgetForm'
import { BudgetRow } from '@/components/budgets/BudgetRow'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Icon } from '@/components/ui/Icon'
import { MonthPicker } from '@/components/ui/MonthPicker'
import { PageHeader } from '@/components/ui/PageHeader'
import { SkeletonRows } from '@/components/ui/Skeleton'
import { StatCard } from '@/components/ui/StatCard'
import { useCurrency } from '@/context/auth-context'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import { useLookups } from '@/hooks/useLookups'
import { useToastAction } from '@/hooks/useToastAction'
import { currentMonthInput, formatCurrency, formatMonth } from '@/lib/format'
import type { Budget } from '@/types'

export function BudgetsPage() {
  const currency = useCurrency()
  const lookups = useLookups()
  const run = useToastAction()

  const [month, setMonth] = useState(currentMonthInput)
  const query = useBudgets(month)
  const editor = useEditor<Budget>()
  const confirm = useConfirm<Budget>()

  const create = useCreateBudget()
  const update = useUpdateBudget()
  const remove = useDeleteBudget()

  const budgets = useMemo(() => query.data ?? [], [query.data])
  const expenseCategories = lookups.categories.filter(
    (category) => category.type === 'expense',
  )

  const totals = useMemo(
    () =>
      budgets.reduce(
        (acc, budget) => ({
          limit: acc.limit + budget.limit,
          spent: acc.spent + budget.spent,
        }),
        { limit: 0, spent: 0 },
      ),
    [budgets],
  )
  const overCount = budgets.filter(
    (budget) => budget.limit > 0 && budget.spent > budget.limit,
  ).length

  const save = async (values: { categoryId: string; limit: number }) => {
    const editing = editor.editing
    const result = await run(
      () =>
        editing
          ? update.mutateAsync({ id: editing.id, limit: values.limit })
          : create.mutateAsync({ ...values, month }),
      editing ? 'Budget updated' : 'Budget added',
    )
    if (result) editor.close()
  }

  const confirmDelete = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => remove.mutateAsync(target.id), 'Budget deleted')
    confirm.cancel()
  }

  return (
    <>
      <PageHeader
        title="Budgets"
        description="Spending limits per category, month by month."
        actions={
          <>
            <MonthPicker value={month} onChange={setMonth} />
            <Button
              onClick={editor.openCreate}
              icon={<Icon name="plus" className="size-4" />}
            >
              Add budget
            </Button>
          </>
        }
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Budgeted"
          value={formatCurrency(totals.limit, currency)}
          hint={formatMonth(month)}
          icon="budgets"
        />
        <StatCard
          label="Spent"
          value={formatCurrency(totals.spent, currency)}
          hint={`${budgets.length} budget${budgets.length === 1 ? '' : 's'}`}
          icon="transactions"
          tone={totals.spent > totals.limit ? 'negative' : 'neutral'}
        />
        <StatCard
          label="Over budget"
          value={String(overCount)}
          hint={overCount === 0 ? 'All within limits' : 'Needs attention'}
          icon="alert"
          tone={overCount > 0 ? 'negative' : 'positive'}
        />
      </div>

      <Card>
        <CardHeader
          title={`Budgets for ${formatMonth(month)}`}
          subtitle="Progress is calculated from this month's expenses."
        />
        <DataState
          isLoading={query.isLoading || lookups.isLoading}
          isError={query.isError}
          error={query.error}
          isEmpty={budgets.length === 0}
          onRetry={() => void query.refetch()}
          skeleton={<SkeletonRows rows={4} className="p-4" />}
          empty={{
            icon: 'budgets',
            title: 'No budgets for this month',
            description:
              'Set a limit on a category to see how the month is tracking.',
            action: (
              <Button size="sm" onClick={editor.openCreate}>
                Add a budget
              </Button>
            ),
          }}
        >
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {budgets.map((budget) => (
              <BudgetRow
                key={budget.id}
                budget={budget}
                category={lookups.category(budget.categoryId)}
                currency={currency}
                onEdit={editor.openEdit}
                onDelete={confirm.request}
              />
            ))}
          </ul>
        </DataState>
      </Card>

      {editor.isOpen ? (
        <BudgetForm
          open
          budget={editor.editing}
          month={month}
          categories={expenseCategories}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Delete budget"
        message="Delete this budget? Your transactions are not affected."
        loading={remove.isPending}
        onConfirm={() => void confirmDelete()}
        onCancel={confirm.cancel}
      />
    </>
  )
}
