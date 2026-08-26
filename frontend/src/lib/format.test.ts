import { describe, expect, it } from 'vitest'
import {
  currentMonthInput,
  formatCompactCurrency,
  formatCurrency,
  formatDate,
  formatMonth,
  formatMonthShort,
  formatPercent,
  monthRange,
  progressPercent,
  toDateInput,
  toMonthInput,
  todayInput,
} from './format'

describe('formatCurrency', () => {
  it('formats using the given currency', () => {
    expect(formatCurrency(1234.5, 'USD')).toBe('$1,234.50')
    expect(formatCurrency(1234.5, 'EUR')).toContain('1,234.50')
  })

  it('defaults to USD and handles lowercase codes', () => {
    expect(formatCurrency(10)).toBe('$10.00')
    expect(formatCurrency(10, 'usd')).toBe('$10.00')
  })

  it('renders negatives and zero', () => {
    expect(formatCurrency(-99.99, 'USD')).toBe('-$99.99')
    expect(formatCurrency(0, 'USD')).toBe('$0.00')
  })

  it('falls back to USD for an invalid code', () => {
    expect(formatCurrency(5, 'NOT-A-CODE')).toBe('$5.00')
  })

  it('treats non-finite values as zero', () => {
    expect(formatCurrency(Number.NaN, 'USD')).toBe('$0.00')
    expect(formatCurrency(Number.POSITIVE_INFINITY, 'USD')).toBe('$0.00')
  })
})

describe('formatCompactCurrency', () => {
  it('shortens large numbers', () => {
    expect(formatCompactCurrency(1500, 'USD')).toBe('$1.5K')
    expect(formatCompactCurrency(2_400_000, 'USD')).toBe('$2.4M')
  })

  it('falls back for invalid codes', () => {
    expect(formatCompactCurrency(1500, '###')).toBe('$1,500.00')
  })
})

describe('formatPercent', () => {
  it('appends a percent sign with one decimal by default', () => {
    expect(formatPercent(23.456)).toBe('23.5%')
    expect(formatPercent(23.456, 0)).toBe('23%')
    expect(formatPercent(Number.NaN)).toBe('0.0%')
  })
})

describe('date helpers', () => {
  it('formats an ISO day without shifting timezone', () => {
    expect(formatDate('2024-05-17')).toContain('2024')
    expect(formatDate('2024-05-17')).toContain('17')
  })

  it('returns a dash for missing or invalid dates', () => {
    expect(formatDate(null)).toBe('—')
    expect(formatDate('')).toBe('—')
    expect(formatDate('nonsense')).toBe('—')
  })

  it('formats months', () => {
    expect(formatMonth('2024-05')).toContain('2024')
    expect(formatMonthShort('2024-05')).toHaveLength(3)
    expect(formatMonth('garbage')).toBe('garbage')
    expect(formatMonthShort('garbage')).toBe('garbage')
  })

  it('builds local date and month inputs', () => {
    const date = new Date(2024, 4, 7)
    expect(toDateInput(date)).toBe('2024-05-07')
    expect(toMonthInput(date)).toBe('2024-05')
    expect(todayInput()).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(currentMonthInput()).toMatch(/^\d{4}-\d{2}$/)
  })

  it('derives the first and last day of a month', () => {
    expect(monthRange('2024-02')).toEqual({ from: '2024-02-01', to: '2024-02-29' })
    expect(monthRange('2023-12')).toEqual({ from: '2023-12-01', to: '2023-12-31' })
  })

  it('falls back to the current month for an unparseable value', () => {
    const range = monthRange('nope')
    expect(range.from).toMatch(/^\d{4}-\d{2}-01$/)
  })
})

describe('progressPercent', () => {
  it('returns the ratio as a percentage', () => {
    expect(progressPercent(50, 200)).toBe(25)
    expect(progressPercent(0, 200)).toBe(0)
  })

  it('clamps above 100', () => {
    expect(progressPercent(300, 200)).toBe(100)
  })

  it('handles a zero or missing limit', () => {
    expect(progressPercent(0, 0)).toBe(0)
    expect(progressPercent(10, 0)).toBe(100)
    expect(progressPercent(10, Number.NaN)).toBe(100)
  })

  it('never returns a negative percentage', () => {
    expect(progressPercent(-10, 100)).toBe(0)
  })
})
