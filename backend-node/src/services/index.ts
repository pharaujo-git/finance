/**
 * Services: the business logic, testable without HTTP.
 *
 * They take domain arguments and return plain objects shaped for the wire;
 * routers only bind and render. Errors are AppError/FieldErrors, which the
 * error middleware maps to a status.
 */

import { randomUUID } from 'node:crypto'
import { conflict, notFound, unauthorized, validationError } from '../core/errors.js'
import { hashPassword, verifyPassword, type TokenService } from '../core/security.js'
import {
  MONTH_FORMAT_MESSAGE,
  addMonths,
  monthKey,
  startOfMonth,
  tryParseMonth,
} from '../domain/dates.js'
import {
  CategoryType,
  EnumValue,
  TransactionType,
  enumOf,
} from '../domain/enums.js'
import { Instant } from '../domain/instant.js'
import { MONEY_ZERO, Money, joinTags, splitTags, trimmedOrNull } from '../domain/money.js'
import {
  type Account,
  AccountRepository,
  type Budget,
  BudgetRepository,
  type Category,
  CategoryRepository,
  EmailTakenError,
  type Goal,
  GoalRepository,
  type RecurringRule,
  RecurringRepository,
  type Transaction,
  TransactionRepository,
  UNCATEGORIZED_COLOR,
  UNCATEGORIZED_NAME,
  type User,
  UserRepository,
} from '../repositories/index.js'
import { balanceOf } from './balance.js'

// Strings the frontend surfaces verbatim, byte-identical to the other backends'.
export const DEFAULT_CURRENCY = 'USD'
const DUPLICATE_EMAIL_MESSAGE = 'An account with that email already exists.'
const INVALID_CREDENTIALS_MESSAGE = 'Invalid email or password.'
const DEFAULT_CATEGORY_MESSAGE = 'Default categories cannot be modified.'
const TRANSFER_TARGET_MESSAGE = 'A transfer requires a destination account.'
const TRANSFER_SAME_ACCOUNT_MESSAGE = 'A transfer must use two different accounts.'
const DUPLICATE_BUDGET_MESSAGE = 'A budget already exists for that category and month.'
const CONTRIBUTION_MESSAGE = 'Contribution amount must be greater than zero.'

const ACCOUNT_ENTITY = 'Account'
const CATEGORY_ENTITY = 'Category'
const TRANSACTION_ENTITY = 'Transaction'
const BUDGET_ENTITY = 'Budget'
const GOAL_ENTITY = 'Goal'
const USER_ENTITY = 'User'

/** Trim, then lowercase -- so "  Owner@Example.COM " is one account. */
export function normalizeEmail(email: string): string {
  return email.trim().toLowerCase()
}

function normalizeCurrency(currency: string): string {
  return currency.trim().toUpperCase()
}

// --- auth -------------------------------------------------------------------

function userDto(user: User): Record<string, unknown> {
  return { id: user.id, email: user.email, name: user.name, currency: user.currency }
}

export class AuthService {
  constructor(
    private readonly users: UserRepository,
    private readonly tokens: TokenService,
  ) {}

  /** Creates an account and signs the caller straight in. */
  async register(email: string, password: string, name: string) {
    const normalized = normalizeEmail(email)
    if (await this.users.findByEmail(normalized)) throw conflict(DUPLICATE_EMAIL_MESSAGE)

    const user: User = {
      id: randomUUID(),
      email: normalized,
      name: name.trim(),
      passwordHash: hashPassword(password),
      currency: DEFAULT_CURRENCY,
      createdAt: Instant.now(),
    }

    try {
      await this.users.add(user)
    } catch (error) {
      // The lookup above is racy on its own; the unique index decides, and the
      // loser reports the same conflict as the early check.
      if (error instanceof EmailTakenError) throw conflict(DUPLICATE_EMAIL_MESSAGE)
      throw error
    }

    return this.authResponse(user)
  }

  /**
   * Verifies credentials and issues a token. An unknown address and a wrong
   * password fail identically, so the response cannot enumerate accounts.
   */
  async login(email: string, password: string) {
    const user = await this.users.findByEmail(normalizeEmail(email))
    if (!user) throw unauthorized(INVALID_CREDENTIALS_MESSAGE)

    const outcome = verifyPassword(user.passwordHash, password)
    if (outcome === 'failed') throw unauthorized(INVALID_CREDENTIALS_MESSAGE)

    if (outcome === 'successRehashNeeded') {
      // The stored blob predates the current parameters; upgrade it now that
      // the plaintext is in hand.
      const upgraded = hashPassword(password)
      await this.users.updatePasswordHash(user.id, upgraded)
      user.passwordHash = upgraded
    }

    return this.authResponse(user)
  }

  async profile(userId: string) {
    return userDto(await this.load(userId))
  }

  async updateProfile(userId: string, name: string, currency: string) {
    const user = await this.load(userId)
    user.name = name.trim()
    user.currency = normalizeCurrency(currency)

    if (!(await this.users.updateProfile(user.id, user.name, user.currency))) {
      throw notFound(USER_ENTITY)
    }
    return userDto(user)
  }

  /** A token whose subject no longer exists gets a 404, not a 401. */
  private async load(userId: string): Promise<User> {
    const user = await this.users.findById(userId)
    if (!user) throw notFound(USER_ENTITY)
    return user
  }

  private authResponse(user: User) {
    return { token: this.tokens.issue(user.id, user.email), user: userDto(user) }
  }
}

// --- accounts ---------------------------------------------------------------

function accountDto(account: Account, balance: Money): Record<string, unknown> {
  return {
    id: account.id,
    name: account.name,
    type: account.type,
    balance,
    currency: account.currency,
    isArchived: account.isArchived,
    createdAt: account.createdAt,
  }
}

export interface AccountInput {
  name: string
  type: EnumValue
  initialBalance: Money | null
  currency: string
  isArchived: boolean | null
}

export class AccountService {
  constructor(
    private readonly accounts: AccountRepository,
    private readonly transactions: TransactionRepository,
  ) {}

  async listAll(userId: string) {
    const accounts = await this.accounts.listAll(userId)
    const slices = await this.transactions.slices(userId, null, null)
    return accounts.map((account) => accountDto(account, balanceOf(account, slices)))
  }

  async get(userId: string, accountId: string) {
    const account = await this.load(userId, accountId)
    const slices = await this.transactions.slices(userId, null, null)
    return accountDto(account, balanceOf(account, slices))
  }

  async create(userId: string, input: AccountInput) {
    const account: Account = {
      id: randomUUID(),
      userId,
      name: input.name.trim(),
      type: input.type,
      // Stored verbatim, not rounded: the other backends echo back whatever
      // scale the caller sent, and the column rounds on write.
      initialBalance: input.initialBalance ?? MONEY_ZERO,
      currency: normalizeCurrency(input.currency),
      isArchived: false,
      createdAt: Instant.now(),
    }
    await this.accounts.add(account)
    // A brand-new account has no transactions, so the opening balance is it.
    return accountDto(account, account.initialBalance)
  }

  async update(userId: string, accountId: string, input: AccountInput) {
    const account = await this.load(userId, accountId)
    account.name = input.name.trim()
    account.type = input.type
    account.currency = normalizeCurrency(input.currency)
    // An omitted flag un-archives, matching `request.IsArchived ?? false`.
    account.isArchived = input.isArchived === true

    await this.accounts.update(account)
    const slices = await this.transactions.slices(userId, null, null)
    return accountDto(account, balanceOf(account, slices))
  }

  /** The DELETE handler's work: flag the row so history stays intact. */
  async archive(userId: string, accountId: string): Promise<void> {
    if (!(await this.accounts.archive(userId, accountId))) throw notFound(ACCOUNT_ENTITY)
  }

  private async load(userId: string, accountId: string): Promise<Account> {
    const account = await this.accounts.get(userId, accountId)
    if (!account) throw notFound(ACCOUNT_ENTITY)
    return account
  }
}

// --- categories -------------------------------------------------------------

function categoryDto(category: Category): Record<string, unknown> {
  return {
    id: category.id,
    name: category.name,
    type: category.type,
    icon: category.icon,
    color: category.color,
    isDefault: category.isDefault,
  }
}

export interface CategoryInfo {
  name: string
  color: string
  type: EnumValue
}

/** Labels a slice, falling back to the shared "Uncategorized" grey. */
export function describe(
  lookup: Map<string, CategoryInfo>,
  categoryId: string | null,
): CategoryInfo {
  if (categoryId !== null) {
    const found = lookup.get(categoryId)
    if (found) return found
  }
  return {
    name: UNCATEGORIZED_NAME,
    color: UNCATEGORIZED_COLOR,
    type: enumOf('CategoryType', CategoryType.Expense),
  }
}

export interface CategoryInput {
  name: string
  type: EnumValue
  icon: string
  color: string
}

export class CategoryService {
  constructor(private readonly categories: CategoryRepository) {}

  async listAll(userId: string) {
    return (await this.categories.listVisible(userId)).map(categoryDto)
  }

  /** Id -> label. A blank stored colour falls back to the grey too. */
  async lookup(userId: string): Promise<Map<string, CategoryInfo>> {
    const visible = await this.categories.listVisible(userId)
    return new Map(
      visible.map((item) => [
        item.id,
        { name: item.name, color: item.color || UNCATEGORIZED_COLOR, type: item.type },
      ]),
    )
  }

  /** A null category is fine; one the caller cannot see is a 404. */
  async ensureUsable(userId: string, categoryId: string | null): Promise<void> {
    if (categoryId === null) return
    if (!(await this.categories.get(userId, categoryId))) throw notFound(CATEGORY_ENTITY)
  }

  async create(userId: string, input: CategoryInput) {
    const category: Category = {
      id: randomUUID(),
      userId,
      name: input.name.trim(),
      type: input.type,
      icon: input.icon.trim(),
      color: input.color.trim(),
      isDefault: false,
    }
    await this.categories.add(category)
    return categoryDto(category)
  }

  async update(userId: string, categoryId: string, input: CategoryInput) {
    const category = await this.loadOwned(userId, categoryId)
    category.name = input.name.trim()
    category.type = input.type
    category.icon = input.icon.trim()
    category.color = input.color.trim()

    await this.categories.update(category)
    return categoryDto(category)
  }

  async remove(userId: string, categoryId: string): Promise<void> {
    await this.loadOwned(userId, categoryId)
    await this.categories.remove(userId, categoryId)
  }

  /** A shared default is visible to everyone but editable by no one. */
  private async loadOwned(userId: string, categoryId: string): Promise<Category> {
    const visible = await this.categories.get(userId, categoryId)
    if (!visible) throw notFound(CATEGORY_ENTITY)
    if (visible.isDefault) throw validationError(DEFAULT_CATEGORY_MESSAGE)

    const owned = await this.categories.getOwned(userId, categoryId)
    if (!owned) throw notFound(CATEGORY_ENTITY)
    return owned
  }
}

// --- transactions -----------------------------------------------------------

export function transactionDto(item: Transaction): Record<string, unknown> {
  return {
    id: item.id,
    accountId: item.accountId,
    categoryId: item.categoryId,
    type: item.type,
    amount: item.amount,
    date: item.date,
    description: item.description,
    notes: item.notes,
    tags: item.tags,
    transferAccountId: item.transferAccountId,
  }
}

export interface TransactionInput {
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

export interface TransactionQuery {
  page: number
  pageSize: number
  accountId: string | null
  categoryId: string | null
  type: EnumValue | null
  dateFrom: Instant | null
  dateTo: Instant | null
  search: string
}

export class TransactionService {
  constructor(
    private readonly transactions: TransactionRepository,
    private readonly accounts: AccountRepository,
    private readonly categories: CategoryService,
  ) {}

  async search(userId: string, query: TransactionQuery) {
    const { items, total } = await this.transactions.search(userId, {
      accountId: query.accountId,
      categoryId: query.categoryId,
      type: query.type,
      dateFrom: query.dateFrom,
      dateTo: query.dateTo,
      search: query.search.trim().toLowerCase(),
      limit: query.pageSize,
      offset: (query.page - 1) * query.pageSize,
    })
    return {
      items: items.map(transactionDto),
      total,
      page: query.page,
      pageSize: query.pageSize,
    }
  }

  async get(userId: string, transactionId: string) {
    return transactionDto(await this.load(userId, transactionId))
  }

  async create(userId: string, input: TransactionInput) {
    const item = await this.build(userId, randomUUID(), input)
    await this.transactions.add(item)
    return transactionDto(item)
  }

  async update(userId: string, transactionId: string, input: TransactionInput) {
    await this.load(userId, transactionId)
    const item = await this.build(userId, transactionId, input)
    await this.transactions.update(item)
    return transactionDto(item)
  }

  async remove(userId: string, transactionId: string): Promise<void> {
    if (!(await this.transactions.remove(userId, transactionId))) {
      throw notFound(TRANSACTION_ENTITY)
    }
  }

  /**
   * Validates the references, then shapes the row. The order of the checks
   * decides which error a doubly-wrong request gets, so it is fixed: account,
   * then category, then the transfer rules.
   */
  private async build(
    userId: string,
    id: string,
    input: TransactionInput,
  ): Promise<Transaction> {
    await this.ensureAccount(userId, input.accountId)
    await this.categories.ensureUsable(userId, input.categoryId)

    let transferAccountId: string | null = null
    if (input.type.equals(TransactionType.Transfer)) {
      if (input.transferAccountId === null) throw validationError(TRANSFER_TARGET_MESSAGE)
      if (input.transferAccountId === input.accountId) {
        throw validationError(TRANSFER_SAME_ACCOUNT_MESSAGE)
      }
      await this.ensureAccount(userId, input.transferAccountId)
      transferAccountId = input.transferAccountId
    }

    return {
      id,
      userId,
      accountId: input.accountId,
      categoryId: input.categoryId,
      type: input.type,
      amount: input.amount.round(),
      date: input.date,
      description: input.description.trim(),
      notes: trimmedOrNull(input.notes),
      // Normalised here, not just on the way to the column: the response has to
      // show the tags as stored, trimmed and without the blanks.
      tags: splitTags(joinTags(input.tags)),
      transferAccountId,
    }
  }

  private async ensureAccount(userId: string, accountId: string): Promise<void> {
    if (!(await this.accounts.exists(userId, accountId))) throw notFound(ACCOUNT_ENTITY)
  }

  private async load(userId: string, transactionId: string): Promise<Transaction> {
    const item = await this.transactions.get(userId, transactionId)
    if (!item) throw notFound(TRANSACTION_ENTITY)
    return item
  }
}

// --- budgets ----------------------------------------------------------------

function budgetDto(budget: Budget, spent: Money): Record<string, unknown> {
  return {
    id: budget.id,
    categoryId: budget.categoryId,
    month: budget.month,
    limit: budget.limit,
    spent,
    // Allowed to go negative: an overspent budget should show how far over.
    remaining: budget.limit.subtract(spent),
  }
}

/**
 * A sort key matching .NET's Guid.CompareTo, which is not byte order: the
 * first three groups compare as signed integers, so a uuid starting 0x80 sorts
 * before one starting 0x7f. Reproduced because it decides budget ordering.
 */
export function compareGuid(left: string, right: string): number {
  const bytesOf = (value: string): Buffer => Buffer.from(value.replace(/-/g, ''), 'hex')
  const a = bytesOf(left)
  const b = bytesOf(right)

  const groups: [number, number, boolean][] = [
    [0, 4, true],
    [4, 6, true],
    [6, 8, true],
  ]
  for (const [start, end, signed] of groups) {
    const x = signed ? a.readIntBE(start, end - start) : a.readUIntBE(start, end - start)
    const y = signed ? b.readIntBE(start, end - start) : b.readUIntBE(start, end - start)
    if (x !== y) return x < y ? -1 : 1
  }
  for (let index = 8; index < 16; index += 1) {
    const x = a[index] ?? 0
    const y = b[index] ?? 0
    if (x !== y) return x < y ? -1 : 1
  }
  return 0
}

export class BudgetService {
  constructor(
    private readonly budgets: BudgetRepository,
    private readonly transactions: TransactionRepository,
    private readonly categories: CategoryService,
  ) {}

  async listAll(userId: string, month: string | null) {
    const key = month ?? monthKey(new Date())
    if (month !== null && tryParseMonth(month) === null) {
      // An explicitly supplied month that is not a real one is a 400; an
      // *absent* key means "this month", but "?month=" does not.
      throw validationError(MONTH_FORMAT_MESSAGE)
    }

    const start = tryParseMonth(key)
    if (start === null) throw validationError(MONTH_FORMAT_MESSAGE)

    const budgets = await this.budgets.listForMonth(userId, key)
    // Nothing to measure, so skip the spend query entirely.
    if (budgets.length === 0) return []

    const spent = await this.spentByCategory(userId, start)
    return [...budgets]
      .sort((left, right) => compareGuid(left.categoryId, right.categoryId))
      .map((budget) => budgetDto(budget, spent.get(budget.categoryId) ?? MONEY_ZERO))
  }

  async create(userId: string, categoryId: string, month: string, limit: Money) {
    const start = tryParseMonth(month)
    if (start === null) throw validationError(MONTH_FORMAT_MESSAGE)

    await this.categories.ensureUsable(userId, categoryId)
    if (await this.budgets.exists(userId, categoryId, month)) {
      throw conflict(DUPLICATE_BUDGET_MESSAGE)
    }

    const budget: Budget = {
      id: randomUUID(),
      userId,
      categoryId,
      month,
      limit: limit.round(),
    }
    await this.budgets.add(budget)

    const spent = await this.spentByCategory(userId, start)
    return budgetDto(budget, spent.get(categoryId) ?? MONEY_ZERO)
  }

  /** Only the limit moves; the category and month are immutable. */
  async update(userId: string, budgetId: string, limit: Money) {
    const budget = await this.load(userId, budgetId)
    budget.limit = limit.round()
    await this.budgets.updateLimit(userId, budgetId, budget.limit)

    const start = tryParseMonth(budget.month)
    const spent =
      start === null ? new Map<string, Money>() : await this.spentByCategory(userId, start)
    return budgetDto(budget, spent.get(budget.categoryId) ?? MONEY_ZERO)
  }

  async remove(userId: string, budgetId: string): Promise<void> {
    if (!(await this.budgets.remove(userId, budgetId))) throw notFound(BUDGET_ENTITY)
  }

  private async load(userId: string, budgetId: string): Promise<Budget> {
    const budget = await this.budgets.get(userId, budgetId)
    if (!budget) throw notFound(BUDGET_ENTITY)
    return budget
  }

  /** Expenses in [start, start+1 month). Income and transfers never count. */
  private async spentByCategory(userId: string, start: Date): Promise<Map<string, Money>> {
    const end = addMonths(start, 1)
    const slices = await this.transactions.slices(userId, Instant.fromDate(start), null)

    const totals = new Map<string, Money>()
    for (const item of slices) {
      if (!item.type.equals(TransactionType.Expense)) continue
      // Uncategorized spending is not measured against any budget.
      if (item.categoryId === null) continue
      if (item.date.valueOf() >= end.getTime()) continue

      const running = totals.get(item.categoryId) ?? MONEY_ZERO
      totals.set(item.categoryId, running.add(item.amount))
    }
    return totals
  }
}

// --- goals ------------------------------------------------------------------

function goalDto(goal: Goal): Record<string, unknown> {
  return {
    id: goal.id,
    name: goal.name,
    targetAmount: goal.targetAmount,
    currentAmount: goal.currentAmount,
    targetDate: goal.targetDate,
    color: goal.color,
  }
}

export interface GoalInput {
  name: string
  targetAmount: Money
  currentAmount: Money | null
  targetDate: Instant | null
  color: string
}

export class GoalService {
  constructor(private readonly goals: GoalRepository) {}

  async listAll(userId: string) {
    return (await this.goals.listAll(userId)).map(goalDto)
  }

  async create(userId: string, input: GoalInput) {
    const goal: Goal = {
      id: randomUUID(),
      userId,
      name: input.name.trim(),
      targetAmount: input.targetAmount.round(),
      currentAmount: (input.currentAmount ?? MONEY_ZERO).round(),
      targetDate: input.targetDate,
      color: input.color.trim(),
    }
    await this.goals.add(goal)
    return goalDto(goal)
  }

  async update(userId: string, goalId: string, input: GoalInput) {
    const goal = await this.load(userId, goalId)
    goal.name = input.name.trim()
    goal.targetAmount = input.targetAmount.round()
    // An omitted currentAmount resets the goal rather than leaving it be,
    // matching the other backends.
    goal.currentAmount = (input.currentAmount ?? MONEY_ZERO).round()
    goal.targetDate = input.targetDate
    goal.color = input.color.trim()

    await this.goals.update(goal)
    return goalDto(goal)
  }

  async remove(userId: string, goalId: string): Promise<void> {
    if (!(await this.goals.remove(userId, goalId))) throw notFound(GOAL_ENTITY)
  }

  async contribute(userId: string, goalId: string, amount: Money) {
    if (!amount.greaterThan(MONEY_ZERO)) throw validationError(CONTRIBUTION_MESSAGE)

    const goal = await this.load(userId, goalId)
    // Not clamped at the target: a goal is allowed to be exceeded.
    goal.currentAmount = goal.currentAmount.add(amount).round()
    await this.goals.update(goal)
    return goalDto(goal)
  }

  private async load(userId: string, goalId: string): Promise<Goal> {
    const goal = await this.goals.get(userId, goalId)
    if (!goal) throw notFound(GOAL_ENTITY)
    return goal
  }
}

export {
  AccountRepository,
  BudgetRepository,
  CategoryRepository,
  GoalRepository,
  RecurringRepository,
  TransactionRepository,
  UserRepository,
  type RecurringRule,
  startOfMonth,
}
