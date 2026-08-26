import { useMemo, useState } from 'react'
import { useCategoryReport, useMonthlyReport } from '@/api/queries'
import { MonthlyReportChart } from '@/components/charts/MonthlyReportChart'
import { ChartPanel } from '@/components/charts/ChartPanel'
import { Badge, ColorDot } from '@/components/ui/Badge'
import { Card, CardHeader } from '@/components/ui/Card'
import { DataState } from '@/components/ui/DataState'
import { Input, Select } from '@/components/ui/Field'
import { PageHeader } from '@/components/ui/PageHeader'
import { ProgressBar } from '@/components/ui/ProgressBar'
import { SkeletonRows } from '@/components/ui/Skeleton'
import { useCurrency } from '@/context/auth-context'
import { cn } from '@/lib/cn'
import { formatCurrency, formatMonth, monthRange, toMonthInput } from '@/lib/format'

function yearOptions(count = 6): number[] {
  const current = new Date().getFullYear()
  return Array.from({ length: count }, (_, index) => current - index)
}

function defaultRange() {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth() - 5, 1)
  return {
    from: monthRange(toMonthInput(start)).from,
    to: monthRange(toMonthInput(now)).to,
  }
}

export function ReportsPage() {
  const currency = useCurrency()
  const [year, setYear] = useState(() => new Date().getFullYear())
  const [range, setRange] = useState(defaultRange)

  const monthly = useMonthlyReport(year)
  const byCategory = useCategoryReport(range)

  const rows = useMemo(() => monthly.data ?? [], [monthly.data])
  const totals = useMemo(
    () =>
      rows.reduce(
        (acc, row) => ({
          income: acc.income + row.income,
          expenses: acc.expenses + row.expenses,
          net: acc.net + row.net,
        }),
        { income: 0, expenses: 0, net: 0 },
      ),
    [rows],
  )

  const categoryRows = byCategory.data ?? []
  const maxCategoryAmount = Math.max(
    1,
    ...categoryRows.map((row) => Math.abs(row.amount)),
  )

  return (
    <>
      <PageHeader
        title="Reports"
        description="Longer-range views of income, spending and categories."
        actions={
          <Select
            label="Year"
            value={String(year)}
            wrapperClassName="w-32"
            onChange={(event) => setYear(Number(event.target.value))}
          >
            {yearOptions().map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        }
      />

      <ChartPanel
        title={`Monthly totals · ${year}`}
        subtitle="Income and expenses with the net line"
        height={320}
        isLoading={monthly.isLoading}
        isError={monthly.isError}
        error={monthly.error}
        isEmpty={rows.length === 0}
        onRetry={() => void monthly.refetch()}
        empty={{
          icon: 'reports',
          title: `Nothing recorded in ${year}`,
          description: 'Pick another year or add some transactions.',
        }}
      >
        <MonthlyReportChart data={rows} currency={currency} />
      </ChartPanel>

      <Card>
        <CardHeader
          title={`Month by month · ${year}`}
          subtitle="Every month with its net result"
        />
        <DataState
          isLoading={monthly.isLoading}
          isError={monthly.isError}
          error={monthly.error}
          isEmpty={rows.length === 0}
          onRetry={() => void monthly.refetch()}
          skeleton={<SkeletonRows rows={6} className="p-4" />}
          empty={{
            icon: 'reports',
            title: `Nothing recorded in ${year}`,
          }}
        >
          <div className="overflow-x-auto">
            <table className="w-full min-w-[520px] border-collapse text-left">
              <thead>
                <tr className="border-b border-slate-200 dark:border-slate-800">
                  <th className="px-4 py-2.5 text-xs font-medium text-slate-500 dark:text-slate-400">
                    Month
                  </th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-slate-500 dark:text-slate-400">
                    Income
                  </th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-slate-500 dark:text-slate-400">
                    Expenses
                  </th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-slate-500 dark:text-slate-400">
                    Net
                  </th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr
                    key={row.month}
                    className="border-b border-slate-100 last:border-0 dark:border-slate-800"
                  >
                    <td className="px-4 py-2.5 text-sm text-slate-900 dark:text-slate-100">
                      {formatMonth(row.month)}
                    </td>
                    <td className="px-4 py-2.5 text-right text-sm tabular-nums text-emerald-600 dark:text-emerald-400">
                      {formatCurrency(row.income, currency)}
                    </td>
                    <td className="px-4 py-2.5 text-right text-sm tabular-nums text-rose-600 dark:text-rose-400">
                      {formatCurrency(row.expenses, currency)}
                    </td>
                    <td
                      className={cn(
                        'px-4 py-2.5 text-right text-sm font-semibold tabular-nums',
                        row.net < 0
                          ? 'text-rose-600 dark:text-rose-400'
                          : 'text-slate-900 dark:text-slate-100',
                      )}
                    >
                      {formatCurrency(row.net, currency)}
                    </td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-800/50">
                  <td className="px-4 py-2.5 text-xs font-semibold text-slate-600 dark:text-slate-300">
                    Total
                  </td>
                  <td className="px-4 py-2.5 text-right text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                    {formatCurrency(totals.income, currency)}
                  </td>
                  <td className="px-4 py-2.5 text-right text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                    {formatCurrency(totals.expenses, currency)}
                  </td>
                  <td className="px-4 py-2.5 text-right text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                    {formatCurrency(totals.net, currency)}
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        </DataState>
      </Card>

      <Card>
        <CardHeader
          title="Category breakdown"
          subtitle="Totals for a custom date range"
          action={
            <div className="flex flex-wrap items-end gap-2">
              <Input
                label="From"
                type="date"
                value={range.from}
                onChange={(event) =>
                  setRange((current) => ({
                    ...current,
                    from: event.target.value,
                  }))
                }
              />
              <Input
                label="To"
                type="date"
                value={range.to}
                onChange={(event) =>
                  setRange((current) => ({ ...current, to: event.target.value }))
                }
              />
            </div>
          }
        />
        <DataState
          isLoading={byCategory.isLoading}
          isError={byCategory.isError}
          error={byCategory.error}
          isEmpty={categoryRows.length === 0}
          onRetry={() => void byCategory.refetch()}
          skeleton={<SkeletonRows rows={5} className="p-4" />}
          empty={{
            icon: 'budgets',
            title: 'No activity in this range',
            description: 'Widen the dates to see category totals.',
          }}
        >
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {categoryRows.map((row) => (
              <li key={`${row.categoryId}-${row.type}`} className="px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <span className="flex items-center gap-2 text-sm text-slate-900 dark:text-slate-100">
                    <ColorDot color={row.color} />
                    {row.categoryName}
                    <Badge tone={row.type === 'income' ? 'success' : 'danger'}>
                      {row.type}
                    </Badge>
                  </span>
                  <span className="text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                    {formatCurrency(row.amount, currency)}
                  </span>
                </div>
                <ProgressBar
                  className="mt-2"
                  value={Math.abs(row.amount)}
                  max={maxCategoryAmount}
                  color={row.color}
                  label={`${row.categoryName} share`}
                />
              </li>
            ))}
          </ul>
        </DataState>
      </Card>
    </>
  )
}
