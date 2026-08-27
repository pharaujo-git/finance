/** Calendar arithmetic that matches .NET's DateTime, and the YYYY-MM month key. */

import { Instant } from './instant.js'

export const MONTH_FORMAT_MESSAGE = 'Month must be in YYYY-MM format.'
export const YEAR_RANGE_MESSAGE = 'Year must be between 1900 and 9999.'

export const MIN_REPORT_YEAR = 1900
export const MAX_REPORT_YEAR = 9999
const MIN_WINDOW_MONTH = 1
const MAX_WINDOW_MONTH = 120

function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month + 1, 0)).getUTCDate()
}

/**
 * .NET's DateTime.AddMonths: clamps the day rather than rolling over, so
 * 31 Jan + 1 month is 28 Feb -- and adding another gives 28 Mar, not 31 Mar.
 * Clock time is preserved.
 */
export function addMonths(moment: Date, months: number): Date {
  const total = moment.getUTCMonth() + months
  const year = moment.getUTCFullYear() + Math.floor(total / 12)
  const month = ((total % 12) + 12) % 12
  const day = Math.min(moment.getUTCDate(), daysInMonth(year, month))

  return new Date(
    Date.UTC(
      year,
      month,
      day,
      moment.getUTCHours(),
      moment.getUTCMinutes(),
      moment.getUTCSeconds(),
      moment.getUTCMilliseconds(),
    ),
  )
}

/** Also clamping, so 29 Feb becomes 28 Feb in a common year. */
export function addYears(moment: Date, years: number): Date {
  return addMonths(moment, years * 12)
}

/** Midnight UTC on the first of the given moment's month. */
export function startOfMonth(moment: Date): Date {
  return new Date(Date.UTC(moment.getUTCFullYear(), moment.getUTCMonth(), 1))
}

export function firstDayUtc(year: number, month: number): Date {
  return new Date(Date.UTC(year, month, 1))
}

/** The YYYY-MM key for a moment's UTC month. */
export function monthKey(moment: Date): string {
  const year = String(moment.getUTCFullYear()).padStart(4, '0')
  const month = String(moment.getUTCMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

/** Midnight UTC on day 1 of that month, or null when it is not a real month. */
export function tryParseMonth(value: string | null | undefined): Date | null {
  const text = (value ?? '').trim()
  if (!/^\d{4}-\d{2}$/.test(text)) return null

  const year = Number(text.slice(0, 4))
  const month = Number(text.slice(5, 7))
  if (month < 1 || month > 12) return null

  return new Date(Date.UTC(year, month - 1, 1))
}

/** `count` month starts ending with the reference's own month, oldest first. */
export function trailingMonths(reference: Date, count: number): Date[] {
  const anchor = startOfMonth(reference)
  return Array.from({ length: count }, (_, index) => addMonths(anchor, index - count + 1))
}

export function clampMonths(months: number): number {
  return Math.max(MIN_WINDOW_MONTH, Math.min(MAX_WINDOW_MONTH, months))
}

export function instantOf(moment: Date): Instant {
  return Instant.fromDate(moment)
}
