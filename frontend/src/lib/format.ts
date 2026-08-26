const currencyCache = new Map<string, Intl.NumberFormat>()

function currencyFormatter(currency: string): Intl.NumberFormat {
  const code = (currency || 'USD').toUpperCase()
  const cached = currencyCache.get(code)
  if (cached) return cached
  let formatter: Intl.NumberFormat
  try {
    formatter = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: code,
      maximumFractionDigits: 2,
    })
  } catch {
    formatter = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: 'USD',
      maximumFractionDigits: 2,
    })
  }
  currencyCache.set(code, formatter)
  return formatter
}

export function formatCurrency(value: number, currency = 'USD'): string {
  if (!Number.isFinite(value)) return currencyFormatter(currency).format(0)
  return currencyFormatter(currency).format(value)
}

/** Compact form for chart axes: $1.2K, $3.4M. */
export function formatCompactCurrency(value: number, currency = 'USD'): string {
  const code = (currency || 'USD').toUpperCase()
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: code,
      notation: 'compact',
      maximumFractionDigits: 1,
    }).format(Number.isFinite(value) ? value : 0)
  } catch {
    return formatCurrency(value, currency)
  }
}

export function formatPercent(value: number, fractionDigits = 1): string {
  const safe = Number.isFinite(value) ? value : 0
  return `${safe.toFixed(fractionDigits)}%`
}

/** `2024-05-17` / ISO timestamp -> `17 May 2024` (locale aware). */
export function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  const date = parseDate(value)
  if (!date) return '—'
  return new Intl.DateTimeFormat(undefined, {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(date)
}

/** `2024-05` -> `May 2024`. Falls back to the raw value when unparseable. */
export function formatMonth(month: string): string {
  const match = /^(\d{4})-(\d{2})/.exec(month)
  if (!match) return month
  const date = new Date(Number(match[1]), Number(match[2]) - 1, 1)
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    year: 'numeric',
  }).format(date)
}

/** `2024-05` -> `May` (for dense chart axes). */
export function formatMonthShort(month: string): string {
  const match = /^(\d{4})-(\d{2})/.exec(month)
  if (!match) return month
  const date = new Date(Number(match[1]), Number(match[2]) - 1, 1)
  return new Intl.DateTimeFormat(undefined, { month: 'short' }).format(date)
}

function parseDate(value: string): Date | null {
  const isoDay = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  const date = isoDay
    ? new Date(Number(isoDay[1]), Number(isoDay[2]) - 1, Number(isoDay[3]))
    : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

/** Local-time `YYYY-MM-DD` (never shifts a day like `toISOString` can). */
export function toDateInput(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** Local-time `YYYY-MM`. */
export function toMonthInput(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}`
}

export function todayInput(): string {
  return toDateInput(new Date())
}

export function currentMonthInput(): string {
  return toMonthInput(new Date())
}

/** First and last day of a `YYYY-MM` month, as date inputs. */
export function monthRange(month: string): { from: string; to: string } {
  const match = /^(\d{4})-(\d{2})$/.exec(month)
  if (!match) {
    const now = new Date()
    return monthRange(toMonthInput(now))
  }
  const year = Number(match[1])
  const monthIndex = Number(match[2]) - 1
  return {
    from: toDateInput(new Date(year, monthIndex, 1)),
    to: toDateInput(new Date(year, monthIndex + 1, 0)),
  }
}

/** Percentage of `limit` used by `spent`, clamped to 0..100 for display. */
export function progressPercent(spent: number, limit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) return spent > 0 ? 100 : 0
  const ratio = (spent / limit) * 100
  if (!Number.isFinite(ratio) || ratio < 0) return 0
  return Math.min(100, ratio)
}
