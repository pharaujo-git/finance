/**
 * The DataAnnotations rules the .NET API enforces, reproduced message for
 * message. The frontend renders these strings verbatim.
 *
 * Note the asymmetry copied from .NET's resources: `required` and
 * `emailAddress` put the field name first ("The Email field ..."), while the
 * length and range rules put "The field" first ("The field Email ...").
 */

import { FieldErrors } from '../core/errors.js'
import { MONEY_MAX, Money, bound } from './money.js'

/** ModelState keys are the PascalCase property names of the .NET request DTOs. */
export const FIELD = {
  email: 'Email',
  password: 'Password',
  name: 'Name',
  currency: 'Currency',
  icon: 'Icon',
  color: 'Color',
  description: 'Description',
  notes: 'Notes',
  amount: 'Amount',
  month: 'Month',
  limit: 'Limit',
  targetAmount: 'TargetAmount',
  currentAmount: 'CurrentAmount',
  page: 'Page',
  pageSize: 'PageSize',
  search: 'Search',
} as const

/** The key MVC's JSON reader uses for a body-level failure. */
export const JSON_BODY_FIELD = '$'

export const LIMITS = {
  email: 256,
  passwordMin: 8,
  passwordMax: 128,
  name: 200,
  currency: 8,
  icon: 64,
  color: 32,
  description: 500,
  notes: 2000,
  search: 200,
} as const

/** Looser than a real month parse on purpose: "2026-13" passes here and fails later. */
export const MONTH_PATTERN = /^\d{4}-\d{2}$/

export function requiredMessage(field: string): string {
  return `The ${field} field is required.`
}

export function emailAddressMessage(field: string): string {
  return `The ${field} field is not a valid e-mail address.`
}

export function minLengthMessage(field: string, limit: number): string {
  return `The field ${field} must be a string or array type with a minimum length of '${limit}'.`
}

export function maxLengthMessage(field: string, limit: number): string {
  return `The field ${field} must be a string or array type with a maximum length of '${limit}'.`
}

export function rangeMessage(field: string, minimum: string, maximum: string): string {
  return `The field ${field} must be between ${minimum} and ${maximum}.`
}

export function missingMembersMessage(members: readonly string[]): string {
  return `The JSON payload was missing required properties, including the following: ${members.join(', ')}`
}

export function invalidValueMessage(value: string, field: string): string {
  return `The value '${value}' is not valid for ${field}.`
}

/** Whitespace-only counts as missing, as [Required] does. */
export function required(errs: FieldErrors, field: string, value: string | null): void {
  if (!(value ?? '').trim()) errs.add(field, requiredMessage(field))
}

/** Exactly one '@', neither first nor last character, on the raw value. */
export function emailAddress(errs: FieldErrors, field: string, value: string | null): void {
  const text = value ?? ''
  const at = text.indexOf('@')
  const valid = at > 0 && at === text.lastIndexOf('@') && at < text.length - 1
  if (!valid) errs.add(field, emailAddressMessage(field))
}

export function minLength(
  errs: FieldErrors,
  field: string,
  value: string | null,
  limit: number,
): void {
  if ([...(value ?? '')].length < limit) errs.add(field, minLengthMessage(field, limit))
}

export function maxLength(
  errs: FieldErrors,
  field: string,
  value: string | null,
  limit: number,
): void {
  // Counted in code points, matching the rune count the Go backend uses.
  if ([...(value ?? '')].length > limit) errs.add(field, maxLengthMessage(field, limit))
}

/** A missing value is not this rule's business; the required-member check owns it. */
export function moneyRange(
  errs: FieldErrors,
  field: string,
  value: Money | null,
  minimum: string,
): void {
  if (value === null) return
  if (value.lessThan(bound(minimum)) || value.greaterThan(bound(MONEY_MAX))) {
    errs.add(field, rangeMessage(field, minimum, MONEY_MAX))
  }
}

export function intRange(
  errs: FieldErrors,
  field: string,
  value: number | null,
  minimum: number,
  maximum: number,
): void {
  if (value === null) return
  if (value < minimum || value > maximum) {
    errs.add(field, rangeMessage(field, String(minimum), String(maximum)))
  }
}

/**
 * Reports absent `required` members under "$", and returns true when something
 * was missing. Every caller returns immediately on true, so no other rule runs
 * -- the .NET pipeline fails deserialisation before model validation starts,
 * and that short-circuit is the single most important ordering behaviour here.
 */
export function requiredMembers(errs: FieldErrors, missing: readonly string[]): boolean {
  if (missing.length === 0) return false
  errs.add(JSON_BODY_FIELD, missingMembersMessage(missing))
  return true
}
