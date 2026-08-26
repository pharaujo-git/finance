import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import {
  formatCompactCurrency,
  formatCurrency,
  formatMonthShort,
} from '@/lib/format'
import type { CashFlowPoint } from '@/types'
import { AXIS_PROPS, CHART_COLORS, GRID_PROPS, TOOLTIP_PROPS } from './chartTheme'

export function CashFlowChart({
  data,
  currency,
}: {
  data: CashFlowPoint[]
  currency: string
}) {
  return (
    <BarChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
      <CartesianGrid {...GRID_PROPS} />
      <XAxis dataKey="month" tickFormatter={formatMonthShort} {...AXIS_PROPS} />
      <YAxis
        width={70}
        tickFormatter={(value: number) => formatCompactCurrency(value, currency)}
        {...AXIS_PROPS}
      />
      <Tooltip
        {...TOOLTIP_PROPS}
        formatter={(value) => formatCurrency(Number(value), currency)}
      />
      <Legend wrapperStyle={{ fontSize: 12 }} />
      <Bar
        dataKey="income"
        name="Income"
        fill={CHART_COLORS.income}
        radius={[4, 4, 0, 0]}
      />
      <Bar
        dataKey="expenses"
        name="Expenses"
        fill={CHART_COLORS.expenses}
        radius={[4, 4, 0, 0]}
      />
    </BarChart>
  )
}
