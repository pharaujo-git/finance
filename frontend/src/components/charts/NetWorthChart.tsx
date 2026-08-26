import {
  Area,
  AreaChart,
  CartesianGrid,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { formatCompactCurrency, formatCurrency, formatMonthShort } from '@/lib/format'
import type { NetWorthPoint } from '@/types'
import { AXIS_PROPS, CHART_COLORS, GRID_PROPS, TOOLTIP_PROPS } from './chartTheme'

export function NetWorthChart({
  data,
  currency,
}: {
  data: NetWorthPoint[]
  currency: string
}) {
  return (
    <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
      <defs>
        <linearGradient id="netWorthFill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={CHART_COLORS.brand} stopOpacity={0.35} />
          <stop offset="100%" stopColor={CHART_COLORS.brand} stopOpacity={0} />
        </linearGradient>
      </defs>
      <CartesianGrid {...GRID_PROPS} />
      <XAxis dataKey="month" tickFormatter={formatMonthShort} {...AXIS_PROPS} />
      <YAxis
        width={70}
        tickFormatter={(value: number) => formatCompactCurrency(value, currency)}
        {...AXIS_PROPS}
      />
      <Tooltip
        {...TOOLTIP_PROPS}
        formatter={(value) => [formatCurrency(Number(value), currency), 'Net worth']}
      />
      <Area
        type="monotone"
        dataKey="value"
        stroke={CHART_COLORS.brand}
        strokeWidth={2}
        fill="url(#netWorthFill)"
      />
    </AreaChart>
  )
}
