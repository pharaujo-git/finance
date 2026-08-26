import { Link } from 'react-router-dom'
import {
  useBudgets,
  useCashFlowSeries,
  useDashboardSummary,
  useNetWorthSeries,
  useSpendingByCategory,
  useTransactions,
} from '@/api/queries'
import { BudgetRow } from '@/components/budgets/BudgetRow'
import { CashFlowChart } from '@/components/charts/CashFlowChart'
import { ChartPanel } from '@/components/charts/ChartPanel'
import { NetWorthChart } from '@/components/charts/NetWorthChart'
import { SpendingDonut } from '@/components/charts/SpendingDonut'
import { signedAmount } from '@/components/transactions/TransactionRow'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader } from '@/components/ui/Card'
import { DataState } from '@/components/ui/DataState'
import { PageHeader } from '@/components/ui/PageHeader'
import { SkeletonCards, SkeletonRows } from '@/components/ui/Skeleton'
import { StatCard } from '@/components/ui/StatCard'
import { useAuth } from '@/context/auth-context'
import { useLookups } from '@/hooks/useLookups'
import { cn } from '@/lib/cn'
import {
  currentMonthInput,
  formatCurrency,
  formatDate,
  formatMonth,
  formatPercent,
} from '@/lib/format'

const RECENT_PAGE = { page: 1, pageSize: 6 }

export function DashboardPage() {
  const { user, currency } = useAuth()
  const month = currentMonthInput()
  const lookups = useLookups()

  const summary = useDashboardSummary()
  const netWorth = useNetWorthSeries(12)
  const cashFlow = useCashFlowSeries(6)
  const spending = useSpendingByCategory(month)
  const budgets = useBudgets(month)
  const recent = useTransactions(RECENT_PAGE)

  const stats = summary.data
  const recentItems = recent.data?.items ?? []
  const budgetItems = (budgets.data ?? []).slice(0, 5)

  return (
    <>
      <PageHeader
        title={user ? `Hello, ${user.name.split(' ')[0]}` : 'Dashboard'}
        description={`Here is how ${formatMonth(month)} is going.`}
      />

      <DataState
        isLoading={summary.isLoading}
        isError={summary.isError}
        error={summary.error}
        onRetry={() => void summary.refetch()}
        skeleton={<SkeletonCards count={4} />}
      >
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <StatCard
            label="Net worth"
            value={formatCurrency(stats?.netWorth ?? 0, currency)}
            hint="Across all accounts"
            icon="wallet"
          />
          <StatCard
            label="Income this month"
            value={formatCurrency(stats?.totalIncome ?? 0, currency)}
            hint={formatMonth(month)}
            icon="investment"
            tone="positive"
          />
          <StatCard
            label="Expenses this month"
            value={formatCurrency(stats?.totalExpenses ?? 0, currency)}
            hint={formatMonth(month)}
            icon="transactions"
            tone="negative"
          />
          <StatCard
            label="Savings rate"
            value={formatPercent(stats?.savingsRate ?? 0)}
            hint="Income kept this month"
            icon="goals"
            tone={(stats?.savingsRate ?? 0) >= 0 ? 'positive' : 'negative'}
          />
        </div>
      </DataState>

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <ChartPanel
            title="Net worth"
            subtitle="Last 12 months"
            isLoading={netWorth.isLoading}
            isError={netWorth.isError}
            error={netWorth.error}
            isEmpty={(netWorth.data ?? []).length === 0}
            onRetry={() => void netWorth.refetch()}
            empty={{
              icon: 'reports',
              title: 'Not enough history yet',
              description: 'Your net worth trend appears once you add data.',
            }}
          >
            <NetWorthChart data={netWorth.data ?? []} currency={currency} />
          </ChartPanel>
        </div>

        <ChartPanel
          title="Spending by category"
          subtitle={formatMonth(month)}
          isLoading={spending.isLoading}
          isError={spending.isError}
          error={spending.error}
          isEmpty={(spending.data ?? []).length === 0}
          onRetry={() => void spending.refetch()}
          empty={{
            icon: 'budgets',
            title: 'No spending this month',
            description: 'Expenses you record will be broken down here.',
          }}
        >
          <SpendingDonut data={spending.data ?? []} currency={currency} />
        </ChartPanel>
      </div>

      <ChartPanel
        title="Cash flow"
        subtitle="Income vs expenses, last 6 months"
        isLoading={cashFlow.isLoading}
        isError={cashFlow.isError}
        error={cashFlow.error}
        isEmpty={(cashFlow.data ?? []).length === 0}
        onRetry={() => void cashFlow.refetch()}
        empty={{
          icon: 'reports',
          title: 'No cash flow yet',
          description: 'Add income and expenses to compare them month by month.',
        }}
      >
        <CashFlowChart data={cashFlow.data ?? []} currency={currency} />
      </ChartPanel>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader
            title="Recent transactions"
            subtitle="Your latest six entries"
            action={
              <Link
                to="/transactions"
                className="text-xs font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400"
              >
                View all
              </Link>
            }
          />
          <DataState
            isLoading={recent.isLoading}
            isError={recent.isError}
            error={recent.error}
            isEmpty={recentItems.length === 0}
            onRetry={() => void recent.refetch()}
            skeleton={<SkeletonRows rows={5} className="p-4" />}
            empty={{
              icon: 'transactions',
              title: 'Nothing recorded yet',
              description: 'Your most recent transactions will show up here.',
            }}
          >
            <ul className="divide-y divide-slate-100 dark:divide-slate-800">
              {recentItems.map((transaction) => {
                const amount = signedAmount(
                  transaction.type,
                  transaction.amount,
                )
                return (
                  <li
                    key={transaction.id}
                    className="flex items-center justify-between gap-3 px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-slate-900 dark:text-slate-100">
                        {transaction.description || '—'}
                      </p>
                      <p className="mt-0.5 flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                        {formatDate(transaction.date)}
                        <Badge>{lookups.categoryName(transaction.categoryId)}</Badge>
                      </p>
                    </div>
                    <span
                      className={cn(
                        'text-sm font-semibold tabular-nums',
                        amount.className,
                      )}
                    >
                      {formatCurrency(amount.value, currency)}
                    </span>
                  </li>
                )
              })}
            </ul>
          </DataState>
        </Card>

        <Card>
          <CardHeader
            title="Budget progress"
            subtitle={formatMonth(month)}
            action={
              <Link
                to="/budgets"
                className="text-xs font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400"
              >
                Manage
              </Link>
            }
          />
          <DataState
            isLoading={budgets.isLoading}
            isError={budgets.isError}
            error={budgets.error}
            isEmpty={budgetItems.length === 0}
            onRetry={() => void budgets.refetch()}
            skeleton={<SkeletonRows rows={4} className="p-4" />}
            empty={{
              icon: 'budgets',
              title: 'No budgets set',
              description: 'Set limits per category to track them here.',
            }}
          >
            <ul className="divide-y divide-slate-100 dark:divide-slate-800">
              {budgetItems.map((budget) => (
                <BudgetRow
                  key={budget.id}
                  budget={budget}
                  category={lookups.category(budget.categoryId)}
                  currency={currency}
                />
              ))}
            </ul>
          </DataState>
        </Card>
      </div>
    </>
  )
}
