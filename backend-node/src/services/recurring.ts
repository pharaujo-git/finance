/** Recurring rules and the pass that turns them into transactions. */

import { randomUUID } from 'node:crypto'
import { notFound, validationError } from '../core/errors.js'
import { addMonths, addYears } from '../domain/dates.js'
import { Frequency, type EnumValue, TransactionType } from '../domain/enums.js'
import { Instant } from '../domain/instant.js'
import type { Money } from '../domain/money.js'
import {
  AccountRepository,
  type RecurringRule,
  RecurringRepository,
  type Transaction,
  TransactionRepository,
} from '../repositories/index.js'
import type { CategoryService } from './index.js'

const RECURRING_ENTITY = 'Recurring rule'
const RECURRING_TRANSFER_MESSAGE = 'Recurring transfers are not supported.'
const RECURRING_END_DATE_MESSAGE = 'End date must not be before the start date.'
const ACCOUNT_ENTITY = 'Account'

/**
 * A cap so one long-dormant rule cannot generate an unbounded batch in a single
 * pass; the rule stays active and the next pass carries on.
 */
export const MAX_OCCURRENCES_PER_PASS = 500
const RECURRING_TAG = 'recurring'

export function recurringDto(rule: RecurringRule): Record<string, unknown> {
  return {
    id: rule.id,
    accountId: rule.accountId,
    categoryId: rule.categoryId,
    type: rule.type,
    amount: rule.amount,
    description: rule.description,
    frequency: rule.frequency,
    startDate: rule.startDate,
    endDate: rule.endDate,
    nextRunDate: rule.nextRunDate,
    isActive: rule.isActive,
  }
}

/** The next occurrence after `moment`. Clock time is preserved throughout. */
export function advance(moment: Date, frequency: EnumValue): Date {
  if (frequency.equals(Frequency.Daily)) {
    return new Date(moment.getTime() + 24 * 60 * 60 * 1000)
  }
  if (frequency.equals(Frequency.Weekly)) {
    return new Date(moment.getTime() + 7 * 24 * 60 * 60 * 1000)
  }
  if (frequency.equals(Frequency.Yearly)) return addYears(moment, 1)
  // Monthly, and anything undefined, advance by a clamped month.
  return addMonths(moment, 1)
}

/** Emits the occurrences due at or before the cutoff, mutating the rule. */
export function materialize(rule: RecurringRule, cutoff: Instant): Transaction[] {
  const created: Transaction[] = []

  for (let index = 0; index < MAX_OCCURRENCES_PER_PASS; index += 1) {
    if (!rule.isActive || rule.nextRunDate.valueOf() > cutoff.valueOf()) break
    if (rule.endDate !== null && rule.nextRunDate.valueOf() > rule.endDate.valueOf()) {
      // An occurrence exactly *on* the end date is still created; only a run
      // past it retires the rule.
      rule.isActive = false
      break
    }

    created.push({
      id: randomUUID(),
      userId: rule.userId,
      accountId: rule.accountId,
      categoryId: rule.categoryId,
      type: rule.type,
      amount: rule.amount,
      date: rule.nextRunDate,
      description: rule.description,
      notes: null,
      tags: [RECURRING_TAG],
      transferAccountId: null,
    })

    rule.nextRunDate = Instant.fromDate(advance(rule.nextRunDate.date, rule.frequency))
  }

  return created
}

export interface RecurringInput {
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

export class RecurringService {
  constructor(
    private readonly rules: RecurringRepository,
    private readonly transactions: TransactionRepository,
    private readonly accounts: AccountRepository,
    private readonly categories: CategoryService,
  ) {}

  async listAll(userId: string) {
    return (await this.rules.listAll(userId)).map(recurringDto)
  }

  async create(userId: string, input: RecurringInput) {
    await this.check(userId, input)

    const rule: RecurringRule = {
      id: randomUUID(),
      userId,
      ...shape(input),
      // The first occurrence is the start date itself.
      nextRunDate: input.startDate,
    }
    await this.rules.add(rule)
    return recurringDto(rule)
  }

  async update(userId: string, ruleId: string, input: RecurringInput) {
    await this.check(userId, input)
    const rule = await this.load(userId, ruleId)
    Object.assign(rule, shape(input))

    // Pull the next run forward if the start moved later; never push it back.
    if (rule.nextRunDate.valueOf() < rule.startDate.valueOf()) {
      rule.nextRunDate = rule.startDate
    }

    await this.rules.update(rule)
    return recurringDto(rule)
  }

  async remove(userId: string, ruleId: string): Promise<void> {
    if (!(await this.rules.remove(userId, ruleId))) throw notFound(RECURRING_ENTITY)
  }

  /** Runs one pass. The caller owns the transaction and the lock. */
  async materializeDue(now: Instant): Promise<number> {
    const due = await this.rules.listDue(now)

    const created: Transaction[] = []
    for (const rule of due) {
      created.push(...materialize(rule, now))
    }

    if (created.length > 0) await this.transactions.addMany(created)
    for (const rule of due) {
      await this.rules.update(rule)
    }

    return created.length
  }

  /** Order fixed: it decides which error a doubly-wrong request gets. */
  private async check(userId: string, input: RecurringInput): Promise<void> {
    if (input.type.equals(TransactionType.Transfer)) {
      throw validationError(RECURRING_TRANSFER_MESSAGE)
    }
    if (!(await this.accounts.exists(userId, input.accountId))) throw notFound(ACCOUNT_ENTITY)
    if (input.endDate !== null && input.endDate.valueOf() < input.startDate.valueOf()) {
      throw validationError(RECURRING_END_DATE_MESSAGE)
    }
    await this.categories.ensureUsable(userId, input.categoryId)
  }

  private async load(userId: string, ruleId: string): Promise<RecurringRule> {
    const rule = await this.rules.get(userId, ruleId)
    if (!rule) throw notFound(RECURRING_ENTITY)
    return rule
  }
}

function shape(input: RecurringInput) {
  return {
    accountId: input.accountId,
    categoryId: input.categoryId,
    type: input.type,
    amount: input.amount.round(),
    description: input.description.trim(),
    frequency: input.frequency,
    startDate: input.startDate,
    endDate: input.endDate,
    // An omitted flag leaves the rule running.
    isActive: input.isActive ?? true,
  }
}
