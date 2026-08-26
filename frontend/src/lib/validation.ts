import { z } from 'zod'

/**
 * Money is kept as a string in form state (that is what `<input>` gives us) and
 * converted on submit, so validation messages stay friendly instead of
 * surfacing coercion errors.
 */
export const moneyString = z
  .string()
  .min(1, 'Amount is required')
  .refine(
    (value) => Number.isFinite(Number(value)),
    'Enter a valid amount',
  )
  .refine((value) => Number(value) > 0, 'Amount must be greater than zero')

/** Same as `moneyString` but allows zero (opening balances, budget limits). */
export const nonNegativeMoneyString = z
  .string()
  .min(1, 'Amount is required')
  .refine((value) => Number.isFinite(Number(value)), 'Enter a valid amount')
  .refine((value) => Number(value) >= 0, 'Amount cannot be negative')

export const requiredString = (message: string) => z.string().min(1, message)

/** `"food, travel"` -> `["food", "travel"]`, de-duplicated. */
export function parseTags(value: string): string[] {
  const seen = new Set<string>()
  for (const raw of value.split(',')) {
    const tag = raw.trim()
    if (tag) seen.add(tag)
  }
  return [...seen]
}

export function formatTags(tags: string[] | null | undefined): string {
  return (tags ?? []).join(', ')
}

export function toNumber(value: string): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

export function toMoneyInput(value: number | null | undefined): string {
  return value === null || value === undefined ? '' : String(value)
}
