/**
 * The four enums, twins of FinanceTracker.Domain.Enums.
 *
 * Two wire formats have to agree at once: the database stores the ordinal in an
 * integer column, and JSON carries the camelCase member name. An ordinal
 * outside the declared set is preserved rather than rejected -- the .NET
 * converter accepts any number on read and writes an undefined value straight
 * back out as a number, and rows already holding one must round-trip.
 */

export type EnumKind = 'AccountType' | 'CategoryType' | 'TransactionType' | 'Frequency'

/** Wire names in ordinal order. "creditCard" is the one that is not a plain lowercase. */
const WIRE_NAMES: Record<EnumKind, readonly string[]> = {
  AccountType: ['checking', 'savings', 'creditCard', 'cash', 'investment'],
  CategoryType: ['income', 'expense'],
  TransactionType: ['income', 'expense', 'transfer'],
  Frequency: ['daily', 'weekly', 'monthly', 'yearly'],
}

export const AccountType = {
  Checking: 0,
  Savings: 1,
  CreditCard: 2,
  Cash: 3,
  Investment: 4,
} as const

export const CategoryType = {
  Income: 0,
  Expense: 1,
} as const

export const TransactionType = {
  Income: 0,
  Expense: 1,
  Transfer: 2,
} as const

export const Frequency = {
  Daily: 0,
  Weekly: 1,
  Monthly: 2,
  Yearly: 3,
} as const

/** An ordinal tagged with the enum it belongs to, so it can render itself. */
export class EnumValue {
  readonly kind: EnumKind
  readonly ordinal: number

  constructor(kind: EnumKind, ordinal: number) {
    this.kind = kind
    this.ordinal = ordinal
  }

  get isDefined(): boolean {
    return this.ordinal >= 0 && this.ordinal < WIRE_NAMES[this.kind].length
  }

  /** The camelCase name, or the bare ordinal for an undefined value. */
  get wireName(): string {
    return WIRE_NAMES[this.kind][this.ordinal] ?? String(this.ordinal)
  }

  equals(ordinal: number): boolean {
    return this.ordinal === ordinal
  }

  toString(): string {
    return this.wireName
  }
}

export function enumOf(kind: EnumKind, ordinal: number): EnumValue {
  return new EnumValue(kind, ordinal)
}

/**
 * Reads a member name in any casing, or an ordinal as digits. Any integer is
 * accepted, defined or not; anything else is null.
 */
export function parseEnum(kind: EnumKind, value: unknown): EnumValue | null {
  if (value === null || value === undefined || typeof value === 'boolean') return null

  if (typeof value === 'number') {
    return Number.isInteger(value) ? new EnumValue(kind, value) : null
  }
  if (typeof value !== 'string') return null

  const text = value.trim()
  if (!text) return null

  const index = WIRE_NAMES[kind].findIndex(
    (name) => name.toLowerCase() === text.toLowerCase(),
  )
  if (index >= 0) return new EnumValue(kind, index)

  return /^-?\d+$/.test(text) ? new EnumValue(kind, Number(text)) : null
}
