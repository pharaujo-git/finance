/**
 * Request bodies and the rules that guard them.
 *
 * A schema library is deliberately not used here. The .NET pipeline fails
 * *deserialisation* before model validation runs, so a payload missing a
 * `required` member reports one error under "$" and nothing else -- a
 * short-circuit no declarative validator expresses. Everything is hand-rolled
 * to keep the four backends byte-identical on error responses.
 */

import { FieldErrors } from '../core/errors.js'
import { type EnumKind, type EnumValue, parseEnum } from '../domain/enums.js'
import { MONTH_FORMAT_MESSAGE } from '../domain/dates.js'
import { type Instant, parseWireDate } from '../domain/instant.js'
import { MONEY_MIN_POSITIVE, MONEY_MIN_ZERO, Money } from '../domain/money.js'
import {
  FIELD,
  JSON_BODY_FIELD,
  LIMITS,
  MONTH_PATTERN,
  emailAddress,
  maxLength,
  minLength,
  moneyRange,
  required,
  requiredMembers,
} from '../domain/validation.js'

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/** A decoded JSON object, tracking which members the caller actually sent. */
export class Body {
  private readonly raw: Record<string, unknown>

  constructor(raw: unknown) {
    if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
      const errs = new FieldErrors()
      errs.add(JSON_BODY_FIELD, 'the JSON value could not be converted to an object')
      throw errs
    }
    this.raw = raw as Record<string, unknown>
  }

  text(key: string): string {
    const value = this.raw[key]
    return typeof value === 'string' ? value : ''
  }

  optionalText(key: string): string | null {
    const value = this.raw[key]
    return typeof value === 'string' ? value : null
  }

  /** A member counts as supplied only when it is there and not null. */
  present(key: string): boolean {
    return this.raw[key] !== null && this.raw[key] !== undefined
  }

  money(key: string): Money | null {
    return Money.fromJson(this.raw[key])
  }

  uuid(key: string): string | null {
    const value = this.raw[key]
    return typeof value === 'string' && UUID_PATTERN.test(value) ? value : null
  }

  moment(key: string): Instant | null {
    const value = this.raw[key]
    return typeof value === 'string' ? parseWireDate(value) : null
  }

  flag(key: string): boolean | null {
    const value = this.raw[key]
    return typeof value === 'boolean' ? value : null
  }

  tags(key: string): string[] {
    const value = this.raw[key]
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
  }

  /**
   * Reads an enum member. A string naming no member is a *deserialisation*
   * failure reported under "$", like the other JSON-reader errors; a number is
   * not, because any ordinal is accepted.
   */
  enum(key: string, kind: EnumKind): EnumValue | null {
    const raw = this.raw[key]
    const parsed = parseEnum(kind, raw)
    if (parsed === null && raw !== null && raw !== undefined) {
      const errs = new FieldErrors()
      errs.add(
        JSON_BODY_FIELD,
        typeof raw === 'string'
          ? `the JSON value "${raw}" could not be converted to ${kind}`
          : `the JSON value could not be converted to ${kind}`,
      )
      throw errs
    }
    return parsed
  }

  /** The `required` members the caller left out, in declaration order. */
  missing(...keys: string[]): string[] {
    return keys.filter((key) => !this.present(key))
  }
}

/** Runs the required-member check, throwing on the first failure. */
function guard(body: Body, ...members: string[]): void {
  const errs = new FieldErrors()
  if (requiredMembers(errs, body.missing(...members))) throw errs
}

// --- auth -------------------------------------------------------------------

export interface RegisterRequest {
  email: string
  password: string
  name: string
}

export function parseRegister(raw: unknown): RegisterRequest {
  const body = new Body(raw)
  const request = {
    email: body.text('email'),
    password: body.text('password'),
    name: body.text('name'),
  }

  const errs = new FieldErrors()
  required(errs, FIELD.email, request.email)
  emailAddress(errs, FIELD.email, request.email)
  maxLength(errs, FIELD.email, request.email, LIMITS.email)
  required(errs, FIELD.password, request.password)
  minLength(errs, FIELD.password, request.password, LIMITS.passwordMin)
  maxLength(errs, FIELD.password, request.password, LIMITS.passwordMax)
  required(errs, FIELD.name, request.name)
  maxLength(errs, FIELD.name, request.name, LIMITS.name)
  errs.raiseIfAny()

  return request
}

export interface LoginRequest {
  email: string
  password: string
}

export function parseLogin(raw: unknown): LoginRequest {
  const body = new Body(raw)
  const request = { email: body.text('email'), password: body.text('password') }

  // No email-shape rule here: sign-in must fail as bad credentials, not as a
  // validation error, so a wrong address cannot be told from a wrong password.
  const errs = new FieldErrors()
  required(errs, FIELD.email, request.email)
  maxLength(errs, FIELD.email, request.email, LIMITS.email)
  required(errs, FIELD.password, request.password)
  maxLength(errs, FIELD.password, request.password, LIMITS.passwordMax)
  errs.raiseIfAny()

  return request
}

export interface UpdateProfileRequest {
  name: string
  currency: string
}

export function parseUpdateProfile(raw: unknown): UpdateProfileRequest {
  const body = new Body(raw)
  const request = { name: body.text('name'), currency: body.text('currency') }

  const errs = new FieldErrors()
  required(errs, FIELD.name, request.name)
  maxLength(errs, FIELD.name, request.name, LIMITS.name)
  required(errs, FIELD.currency, request.currency)
  maxLength(errs, FIELD.currency, request.currency, LIMITS.currency)
  errs.raiseIfAny()

  return request
}

// --- accounts ---------------------------------------------------------------

export interface AccountRequest {
  name: string
  type: EnumValue
  initialBalance: Money | null
  currency: string
  isArchived: boolean | null
}

export function parseAccount(raw: unknown): AccountRequest {
  const body = new Body(raw)
  guard(body, 'type')

  // An omitted currency and an explicit null both mean USD. The Go API cannot
  // tell them apart, so neither does this one.
  const currency = body.optionalText('currency')
  const request = {
    name: body.text('name'),
    type: body.enum('type', 'AccountType')!,
    initialBalance: body.money('initialBalance'),
    currency: currency ?? 'USD',
    isArchived: body.flag('isArchived'),
  }

  const errs = new FieldErrors()
  required(errs, FIELD.name, request.name)
  maxLength(errs, FIELD.name, request.name, LIMITS.name)
  required(errs, FIELD.currency, request.currency)
  maxLength(errs, FIELD.currency, request.currency, LIMITS.currency)
  errs.raiseIfAny()

  return request
}

// --- categories -------------------------------------------------------------

export interface CategoryRequest {
  name: string
  type: EnumValue
  icon: string
  color: string
}

export function parseCategory(raw: unknown): CategoryRequest {
  const body = new Body(raw)
  guard(body, 'type')

  const request = {
    name: body.text('name'),
    type: body.enum('type', 'CategoryType')!,
    icon: body.text('icon'),
    color: body.text('color'),
  }

  const errs = new FieldErrors()
  required(errs, FIELD.name, request.name)
  maxLength(errs, FIELD.name, request.name, LIMITS.name)
  maxLength(errs, FIELD.icon, request.icon, LIMITS.icon)
  maxLength(errs, FIELD.color, request.color, LIMITS.color)
  errs.raiseIfAny()

  return request
}

// --- transactions -----------------------------------------------------------

export interface TransactionRequest {
  accountId: string
  categoryId: string | null
  type: EnumValue
  amount: Money
  date: Instant
  description: string
  notes: string | null
  tags: string[]
  transferAccountId: string | null
}

export function parseTransaction(raw: unknown): TransactionRequest {
  const body = new Body(raw)
  guard(body, 'accountId', 'type', 'amount', 'date')

  const request = {
    accountId: body.uuid('accountId') ?? '',
    categoryId: body.uuid('categoryId'),
    type: body.enum('type', 'TransactionType')!,
    amount: body.money('amount') ?? Money.zero(),
    date: body.moment('date')!,
    description: body.text('description'),
    notes: body.optionalText('notes'),
    tags: body.tags('tags'),
    transferAccountId: body.uuid('transferAccountId'),
  }

  const errs = new FieldErrors()
  moneyRange(errs, FIELD.amount, request.amount, MONEY_MIN_POSITIVE)
  required(errs, FIELD.description, request.description)
  maxLength(errs, FIELD.description, request.description, LIMITS.description)
  if (request.notes !== null) {
    maxLength(errs, FIELD.notes, request.notes, LIMITS.notes)
  }
  errs.raiseIfAny()

  return request
}

// --- budgets ----------------------------------------------------------------

export interface CreateBudgetRequest {
  categoryId: string
  month: string
  limit: Money
}

export function parseCreateBudget(raw: unknown): CreateBudgetRequest {
  const body = new Body(raw)
  guard(body, 'categoryId', 'limit')

  const request = {
    categoryId: body.uuid('categoryId') ?? '',
    month: body.text('month'),
    limit: body.money('limit') ?? Money.zero(),
  }

  const errs = new FieldErrors()
  required(errs, FIELD.month, request.month)
  if (request.month && !MONTH_PATTERN.test(request.month)) {
    errs.add(FIELD.month, MONTH_FORMAT_MESSAGE)
  }
  moneyRange(errs, FIELD.limit, request.limit, MONEY_MIN_ZERO)
  errs.raiseIfAny()

  return request
}

export interface UpdateBudgetRequest {
  limit: Money
}

export function parseUpdateBudget(raw: unknown): UpdateBudgetRequest {
  const body = new Body(raw)
  guard(body, 'limit')

  const request = { limit: body.money('limit') ?? Money.zero() }
  const errs = new FieldErrors()
  moneyRange(errs, FIELD.limit, request.limit, MONEY_MIN_ZERO)
  errs.raiseIfAny()

  return request
}

// --- goals ------------------------------------------------------------------

export interface GoalRequest {
  name: string
  targetAmount: Money
  currentAmount: Money | null
  targetDate: Instant | null
  color: string
}

export function parseGoal(raw: unknown): GoalRequest {
  const body = new Body(raw)
  guard(body, 'targetAmount')

  const request = {
    name: body.text('name'),
    targetAmount: body.money('targetAmount') ?? Money.zero(),
    currentAmount: body.money('currentAmount'),
    targetDate: body.moment('targetDate'),
    color: body.text('color'),
  }

  const errs = new FieldErrors()
  required(errs, FIELD.name, request.name)
  maxLength(errs, FIELD.name, request.name, LIMITS.name)
  moneyRange(errs, FIELD.targetAmount, request.targetAmount, MONEY_MIN_POSITIVE)
  moneyRange(errs, FIELD.currentAmount, request.currentAmount, MONEY_MIN_ZERO)
  maxLength(errs, FIELD.color, request.color, LIMITS.color)
  errs.raiseIfAny()

  return request
}

export interface ContributeRequest {
  amount: Money
}

export function parseContribute(raw: unknown): ContributeRequest {
  const body = new Body(raw)
  guard(body, 'amount')

  const request = { amount: body.money('amount') ?? Money.zero() }
  const errs = new FieldErrors()
  moneyRange(errs, FIELD.amount, request.amount, MONEY_MIN_POSITIVE)
  errs.raiseIfAny()

  return request
}

// --- recurring --------------------------------------------------------------

export interface RecurringRequest {
  accountId: string
  categoryId: string | null
  type: EnumValue
  amount: Money
  description: string
  frequency: EnumValue
  startDate: Instant
  endDate: Instant | null
  isActive: boolean | null
}

export function parseRecurring(raw: unknown): RecurringRequest {
  const body = new Body(raw)
  guard(body, 'accountId', 'type', 'amount', 'frequency', 'startDate')

  const request = {
    accountId: body.uuid('accountId') ?? '',
    categoryId: body.uuid('categoryId'),
    type: body.enum('type', 'TransactionType')!,
    amount: body.money('amount') ?? Money.zero(),
    description: body.text('description'),
    frequency: body.enum('frequency', 'Frequency')!,
    startDate: body.moment('startDate')!,
    endDate: body.moment('endDate'),
    isActive: body.flag('isActive'),
  }

  const errs = new FieldErrors()
  moneyRange(errs, FIELD.amount, request.amount, MONEY_MIN_POSITIVE)
  required(errs, FIELD.description, request.description)
  maxLength(errs, FIELD.description, request.description, LIMITS.description)
  errs.raiseIfAny()

  return request
}
