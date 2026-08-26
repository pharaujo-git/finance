import { Cell, Legend, Pie, PieChart, Tooltip } from 'recharts'
import { formatCurrency } from '@/lib/format'
import type { SpendingSlice } from '@/types'
import { sliceColor, TOOLTIP_PROPS } from './chartTheme'

export function SpendingDonut({
  data,
  currency,
}: {
  data: SpendingSlice[]
  currency: string
}) {
  return (
    <PieChart>
      <Pie
        data={data}
        dataKey="amount"
        nameKey="categoryName"
        innerRadius="55%"
        outerRadius="80%"
        paddingAngle={2}
        stroke="none"
      >
        {data.map((slice, index) => (
          <Cell key={slice.categoryId} fill={sliceColor(slice.color, index)} />
        ))}
      </Pie>
      <Tooltip
        {...TOOLTIP_PROPS}
        formatter={(value) => formatCurrency(Number(value), currency)}
      />
      <Legend wrapperStyle={{ fontSize: 12 }} />
    </PieChart>
  )
}
