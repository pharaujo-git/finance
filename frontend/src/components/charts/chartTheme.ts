export const CHART_COLORS = {
  income: '#10b981',
  expenses: '#f43f5e',
  net: '#6366f1',
  brand: '#16a34a',
} as const

export const AXIS_PROPS = {
  stroke: 'currentColor',
  tickLine: false,
  axisLine: false,
  tick: { fontSize: 11, fill: 'currentColor' },
} as const

export const GRID_PROPS = {
  strokeDasharray: '3 3',
  stroke: 'currentColor',
  strokeOpacity: 0.15,
  vertical: false,
} as const

export const TOOLTIP_PROPS = {
  cursor: { fill: 'currentColor', fillOpacity: 0.06 },
  contentStyle: {
    borderRadius: '0.75rem',
    border: '1px solid rgb(148 163 184 / 0.35)',
    background: 'rgb(255 255 255 / 0.97)',
    color: '#0f172a',
    fontSize: '12px',
    boxShadow: '0 10px 25px -12px rgb(15 23 42 / 0.35)',
  },
  labelStyle: { fontWeight: 600, marginBottom: 4 },
} as const

/** Deterministic fallback palette for slices the API returns without a colour. */
const FALLBACK_PALETTE = [
  '#16a34a',
  '#0ea5e9',
  '#f59e0b',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#f43f5e',
  '#64748b',
]

export function sliceColor(color: string | null | undefined, index: number) {
  return color || FALLBACK_PALETTE[index % FALLBACK_PALETTE.length]
}
