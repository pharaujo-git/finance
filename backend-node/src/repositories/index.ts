/**
 * Repositories: all SQL lives here, one group per aggregate.
 *
 * They hold queries and persistence only -- no business rules, and nothing
 * that knows about HTTP. Every identifier is quoted because the schema uses
 * EF Core's PascalCase names.
 */

import type { DbClient } from '../db.js'
import { readInstant, readInstantOrNull, readMoney } from '../db.js'
import { EnumValue, enumOf } from '../domain/enums.js'
import type { Instant } from '../domain/instant.js'
import type { Money } from '../domain/money.js'
import { joinTags, splitTags } from '../domain/money.js'

/** Rows per INSERT when writing imports and materialised recurrences. */
const CHUNK = 500

export class EmailTakenError extends Error {
  constructor(email: string) {
    super(`email already registered: ${email}`)
    this.name = 'EmailTakenError'
  }
}

const UNIQUE_VIOLATION = '23505'

function isUniqueViolation(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'code' in error &&
    (error as { code?: string }).code === UNIQUE_VIOLATION
}

// --- users ------------------------------------------------------------------

export interface User {
  id: string
  email: string
  name: string
  passwordHash: string
  currency: string
  createdAt: Instant
}

const USER_COLUMNS = '"Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt"'

function toUser(row: Record<string, unknown>): User {
  return {
    id: row.Id as string,
    email: row.Email as string,
    name: row.Name as string,
    passwordHash: row.PasswordHash as string,
    currency: row.Currency as string,
    createdAt: readInstant(row.CreatedAt),
  }
}

export class UserRepository {
  constructor(private readonly db: DbClient) {}

  async findByEmail(email: string): Promise<User | null> {
    const result = await this.db.query(
      `SELECT ${USER_COLUMNS} FROM "Users" WHERE "Email" = $1`,
      [email],
    )
    return result.rows[0] ? toUser(result.rows[0]) : null
  }

  async findById(userId: string): Promise<User | null> {
    const result = await this.db.query(
      `SELECT ${USER_COLUMNS} FROM "Users" WHERE "Id" = $1`,
      [userId],
    )
    return result.rows[0] ? toUser(result.rows[0]) : null
  }

  async add(user: User): Promise<void> {
    try {
      await this.db.query(
        `INSERT INTO "Users" ("Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt")
         VALUES ($1, $2, $3, $4, $5, $6)`,
        [
          user.id,
          user.email,
          user.name,
          user.passwordHash,
          user.currency,
          user.createdAt.toParameter(),
        ],
      )
    } catch (error) {
      if (isUniqueViolation(error)) throw new EmailTakenError(user.email)
      throw error
    }
  }

  async updatePasswordHash(userId: string, passwordHash: string): Promise<void> {
    await this.db.query('UPDATE "Users" SET "PasswordHash" = $2 WHERE "Id" = $1', [
      userId,
      passwordHash,
    ])
  }

  async updateProfile(userId: string, name: string, currency: string): Promise<boolean> {
    const result = await this.db.query(
      'UPDATE "Users" SET "Name" = $2, "Currency" = $3 WHERE "Id" = $1',
      [userId, name, currency],
    )
    return (result.rowCount ?? 0) > 0
  }
}

// --- accounts ---------------------------------------------------------------

export interface Account {
  id: string
  userId: string
  name: string
  type: EnumValue
  initialBalance: Money
  currency: string
  isArchived: boolean
  createdAt: Instant
}

const ACCOUNT_COLUMNS =
  '"Id", "UserId", "Name", "Type", "InitialBalance", "Currency", "IsArchived", "CreatedAt"'

function toAccount(row: Record<string, unknown>): Account {
  return {
    id: row.Id as string,
    userId: row.UserId as string,
    name: row.Name as string,
    type: enumOf('AccountType', Number(row.Type)),
    initialBalance: readMoney(row.InitialBalance),
    currency: row.Currency as string,
    isArchived: row.IsArchived as boolean,
    createdAt: readInstant(row.CreatedAt),
  }
}

export class AccountRepository {
  constructor(private readonly db: DbClient) {}

  /** Active accounts first, then by name -- the order the UI expects. */
  async listAll(userId: string): Promise<Account[]> {
    const result = await this.db.query(
      `SELECT ${ACCOUNT_COLUMNS} FROM "Accounts" WHERE "UserId" = $1
       ORDER BY "IsArchived", "Name"`,
      [userId],
    )
    return result.rows.map(toAccount)
  }

  async get(userId: string, accountId: string): Promise<Account | null> {
    const result = await this.db.query(
      `SELECT ${ACCOUNT_COLUMNS} FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2`,
      [accountId, userId],
    )
    return result.rows[0] ? toAccount(result.rows[0]) : null
  }

  async exists(userId: string, accountId: string): Promise<boolean> {
    const result = await this.db.query(
      'SELECT 1 FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2',
      [accountId, userId],
    )
    return (result.rowCount ?? 0) > 0
  }

  async add(account: Account): Promise<void> {
    await this.db.query(
      `INSERT INTO "Accounts" ("Id", "UserId", "Name", "Type", "InitialBalance", "Currency",
        "IsArchived", "CreatedAt") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
      [
        account.id,
        account.userId,
        account.name,
        account.type.ordinal,
        account.initialBalance.toString(),
        account.currency,
        account.isArchived,
        account.createdAt.toParameter(),
      ],
    )
  }

  async update(account: Account): Promise<void> {
    await this.db.query(
      `UPDATE "Accounts" SET "Name" = $3, "Type" = $4, "InitialBalance" = $5, "Currency" = $6,
        "IsArchived" = $7 WHERE "Id" = $1 AND "UserId" = $2`,
      [
        account.id,
        account.userId,
        account.name,
        account.type.ordinal,
        account.initialBalance.toString(),
        account.currency,
        account.isArchived,
      ],
    )
  }

  async archive(userId: string, accountId: string): Promise<boolean> {
    const result = await this.db.query(
      'UPDATE "Accounts" SET "IsArchived" = true WHERE "Id" = $1 AND "UserId" = $2',
      [accountId, userId],
    )
    return (result.rowCount ?? 0) > 0
  }
}

// --- categories -------------------------------------------------------------

export const UNCATEGORIZED_NAME = 'Uncategorized'
export const UNCATEGORIZED_COLOR = '#94a3b8'

export interface Category {
  id: string
  userId: string | null
  name: string
  type: EnumValue
  icon: string
  color: string
  isDefault: boolean
}

const CATEGORY_COLUMNS = '"Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault"'

function toCategory(row: Record<string, unknown>): Category {
  return {
    id: row.Id as string,
    userId: (row.UserId as string | null) ?? null,
    name: row.Name as string,
    type: enumOf('CategoryType', Number(row.Type)),
    icon: row.Icon as string,
    color: row.Color as string,
    isDefault: row.IsDefault as boolean,
  }
}

export class CategoryRepository {
  constructor(private readonly db: DbClient) {}

  /** The shared defaults plus the caller's own. */
  async listVisible(userId: string): Promise<Category[]> {
    const result = await this.db.query(
      `SELECT ${CATEGORY_COLUMNS} FROM "Categories"
       WHERE "IsDefault" = true OR "UserId" = $1 ORDER BY "Type", "Name"`,
      [userId],
    )
    return result.rows.map(toCategory)
  }

  async get(userId: string, categoryId: string): Promise<Category | null> {
    const result = await this.db.query(
      `SELECT ${CATEGORY_COLUMNS} FROM "Categories"
       WHERE "Id" = $1 AND ("IsDefault" = true OR "UserId" = $2)`,
      [categoryId, userId],
    )
    return result.rows[0] ? toCategory(result.rows[0]) : null
  }

  async getOwned(userId: string, categoryId: string): Promise<Category | null> {
    const result = await this.db.query(
      `SELECT ${CATEGORY_COLUMNS} FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2`,
      [categoryId, userId],
    )
    return result.rows[0] ? toCategory(result.rows[0]) : null
  }

  async add(category: Category): Promise<void> {
    await this.db.query(
      `INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
       VALUES ($1, $2, $3, $4, $5, $6, $7)`,
      [
        category.id,
        category.userId,
        category.name,
        category.type.ordinal,
        category.icon,
        category.color,
        category.isDefault,
      ],
    )
  }

  async update(category: Category): Promise<void> {
    await this.db.query(
      `UPDATE "Categories" SET "Name" = $3, "Type" = $4, "Icon" = $5, "Color" = $6
       WHERE "Id" = $1 AND "UserId" = $2`,
      [
        category.id,
        category.userId,
        category.name,
        category.type.ordinal,
        category.icon,
        category.color,
      ],
    )
  }

  /**
   * Detaches the category from its transactions, then drops its budgets. All
   * three writes share one transaction: a half-applied delete would leave
   * transactions pointing at a row that no longer exists.
   */
  async remove(userId: string, categoryId: string): Promise<void> {
    const client = this.db
    await client.query('BEGIN')
    try {
      await client.query(
        'UPDATE "Transactions" SET "CategoryId" = NULL WHERE "UserId" = $1 AND "CategoryId" = $2',
        [userId, categoryId],
      )
      await client.query('DELETE FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2', [
        userId,
        categoryId,
      ])
      await client.query('DELETE FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2', [
        categoryId,
        userId,
      ])
      await client.query('COMMIT')
    } catch (error) {
      await client.query('ROLLBACK')
      throw error
    }
  }
}

// --- transactions -----------------------------------------------------------

export interface Transaction {
  id: string
  userId: string
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

/** Just enough of a transaction to compute a balance or an aggregate. */
export interface TransactionSlice {
  accountId: string
  transferAccountId: string | null
  categoryId: string | null
  type: EnumValue
  amount: Money
  date: Instant
}

const TRANSACTION_COLUMNS =
  '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Date", "Description", ' +
  '"Notes", "TagsRaw", "TransferAccountId"'

function toTransaction(row: Record<string, unknown>): Transaction {
  return {
    id: row.Id as string,
    userId: row.UserId as string,
    accountId: row.AccountId as string,
    categoryId: (row.CategoryId as string | null) ?? null,
    type: enumOf('TransactionType', Number(row.Type)),
    amount: readMoney(row.Amount),
    date: readInstant(row.Date),
    description: row.Description as string,
    notes: (row.Notes as string | null) ?? null,
    tags: splitTags(row.TagsRaw as string | null),
    transferAccountId: (row.TransferAccountId as string | null) ?? null,
  }
}

export interface TransactionFilter {
  accountId?: string | null
  categoryId?: string | null
  type?: EnumValue | null
  dateFrom?: Instant | null
  dateTo?: Instant | null
  search?: string
  limit?: number
  offset?: number
}

/** Builds the shared WHERE clause; both bounds are inclusive. */
function predicate(userId: string, filter: TransactionFilter): [string, unknown[]] {
  const clauses = ['"UserId" = $1']
  const args: unknown[] = [userId]

  const add = (clause: (n: number) => string, value: unknown): void => {
    args.push(value)
    clauses.push(clause(args.length))
  }

  if (filter.accountId) {
    add((n) => `("AccountId" = $${n} OR "TransferAccountId" = $${n})`, filter.accountId)
  }
  if (filter.categoryId) add((n) => `"CategoryId" = $${n}`, filter.categoryId)
  if (filter.type) add((n) => `"Type" = $${n}`, filter.type.ordinal)
  if (filter.dateFrom) add((n) => `"Date" >= $${n}`, filter.dateFrom.toParameter())
  if (filter.dateTo) add((n) => `"Date" <= $${n}`, filter.dateTo.toParameter())
  if (filter.search) {
    // Lowered on both sides: the term arrives already lowercased.
    add((n) => `LOWER("Description") LIKE $${n}`, `%${filter.search}%`)
  }

  return [` WHERE ${clauses.join(' AND ')}`, args]
}

export class TransactionRepository {
  constructor(private readonly db: DbClient) {}

  /** One page of matches plus the total number of them. */
  async search(
    userId: string,
    filter: TransactionFilter,
  ): Promise<{ items: Transaction[]; total: number }> {
    const [where, args] = predicate(userId, filter)

    const counted = await this.db.query(
      `SELECT COUNT(*) AS "total" FROM "Transactions"${where}`,
      args,
    )
    const total = Number(counted.rows[0]?.total ?? 0)

    const paged = [...args, filter.limit ?? 20, filter.offset ?? 0]
    const result = await this.db.query(
      `SELECT ${TRANSACTION_COLUMNS} FROM "Transactions"${where}
       ORDER BY "Date" DESC, "Id" DESC LIMIT $${paged.length - 1} OFFSET $${paged.length}`,
      paged,
    )
    return { items: result.rows.map(toTransaction), total }
  }

  async get(userId: string, transactionId: string): Promise<Transaction | null> {
    const result = await this.db.query(
      `SELECT ${TRANSACTION_COLUMNS} FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2`,
      [transactionId, userId],
    )
    return result.rows[0] ? toTransaction(result.rows[0]) : null
  }

  /** Newest first -- the order the CSV export writes. */
  async listRange(
    userId: string,
    dateFrom: Instant | null,
    dateTo: Instant | null,
  ): Promise<Transaction[]> {
    const [where, args] = predicate(userId, { dateFrom, dateTo })
    const result = await this.db.query(
      `SELECT ${TRANSACTION_COLUMNS} FROM "Transactions"${where}
       ORDER BY "Date" DESC, "Id" DESC`,
      args,
    )
    return result.rows.map(toTransaction)
  }

  /** Oldest first, so a running total accumulates in one pass. */
  async slices(
    userId: string,
    dateFrom: Instant | null,
    dateTo: Instant | null,
  ): Promise<TransactionSlice[]> {
    const [where, args] = predicate(userId, { dateFrom, dateTo })
    const result = await this.db.query(
      `SELECT "AccountId", "TransferAccountId", "CategoryId", "Type", "Amount", "Date"
       FROM "Transactions"${where} ORDER BY "Date", "Id"`,
      args,
    )
    return result.rows.map((row) => ({
      accountId: row.AccountId as string,
      transferAccountId: (row.TransferAccountId as string | null) ?? null,
      categoryId: (row.CategoryId as string | null) ?? null,
      type: enumOf('TransactionType', Number(row.Type)),
      amount: readMoney(row.Amount),
      date: readInstant(row.Date),
    }))
  }

  async add(transaction: Transaction): Promise<void> {
    await this.addMany([transaction])
  }

  /** Chunked so one oversized import cannot build an unbounded statement. */
  async addMany(transactions: readonly Transaction[]): Promise<void> {
    for (let start = 0; start < transactions.length; start += CHUNK) {
      const chunk = transactions.slice(start, start + CHUNK)
      const values: unknown[] = []
      const rows = chunk.map((item, index) => {
        const base = index * 11
        values.push(
          item.id,
          item.userId,
          item.accountId,
          item.categoryId,
          item.type.ordinal,
          item.amount.toString(),
          item.date.toParameter(),
          item.description,
          item.notes,
          joinTags(item.tags),
          item.transferAccountId,
        )
        return `($${base + 1}, $${base + 2}, $${base + 3}, $${base + 4}, $${base + 5}, $${base + 6}, $${base + 7}, $${base + 8}, $${base + 9}, $${base + 10}, $${base + 11})`
      })

      await this.db.query(
        `INSERT INTO "Transactions" ("Id", "UserId", "AccountId", "CategoryId", "Type",
          "Amount", "Date", "Description", "Notes", "TagsRaw", "TransferAccountId")
         VALUES ${rows.join(', ')}`,
        values,
      )
    }
  }

  async update(transaction: Transaction): Promise<void> {
    await this.db.query(
      `UPDATE "Transactions" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5,
        "Amount" = $6, "Date" = $7, "Description" = $8, "Notes" = $9, "TagsRaw" = $10,
        "TransferAccountId" = $11 WHERE "Id" = $1 AND "UserId" = $2`,
      [
        transaction.id,
        transaction.userId,
        transaction.accountId,
        transaction.categoryId,
        transaction.type.ordinal,
        transaction.amount.toString(),
        transaction.date.toParameter(),
        transaction.description,
        transaction.notes,
        joinTags(transaction.tags),
        transaction.transferAccountId,
      ],
    )
  }

  async remove(userId: string, transactionId: string): Promise<boolean> {
    const result = await this.db.query(
      'DELETE FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2',
      [transactionId, userId],
    )
    return (result.rowCount ?? 0) > 0
  }
}

// --- budgets ----------------------------------------------------------------

export interface Budget {
  id: string
  userId: string
  categoryId: string
  month: string
  limit: Money
}

const BUDGET_COLUMNS = '"Id", "UserId", "CategoryId", "Month", "Limit"'

function toBudget(row: Record<string, unknown>): Budget {
  return {
    id: row.Id as string,
    userId: row.UserId as string,
    categoryId: row.CategoryId as string,
    month: row.Month as string,
    limit: readMoney(row.Limit),
  }
}

export class BudgetRepository {
  constructor(private readonly db: DbClient) {}

  /** Unordered on purpose -- the service sorts by category id. */
  async listForMonth(userId: string, month: string): Promise<Budget[]> {
    const result = await this.db.query(
      `SELECT ${BUDGET_COLUMNS} FROM "Budgets" WHERE "UserId" = $1 AND "Month" = $2`,
      [userId, month],
    )
    return result.rows.map(toBudget)
  }

  async get(userId: string, budgetId: string): Promise<Budget | null> {
    const result = await this.db.query(
      `SELECT ${BUDGET_COLUMNS} FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2`,
      [budgetId, userId],
    )
    return result.rows[0] ? toBudget(result.rows[0]) : null
  }

  async exists(userId: string, categoryId: string, month: string): Promise<boolean> {
    const result = await this.db.query(
      'SELECT 1 FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2 AND "Month" = $3',
      [userId, categoryId, month],
    )
    return (result.rowCount ?? 0) > 0
  }

  async add(budget: Budget): Promise<void> {
    await this.db.query(
      `INSERT INTO "Budgets" ("Id", "UserId", "CategoryId", "Month", "Limit")
       VALUES ($1, $2, $3, $4, $5)`,
      [budget.id, budget.userId, budget.categoryId, budget.month, budget.limit.toString()],
    )
  }

  async updateLimit(userId: string, budgetId: string, limit: Money): Promise<boolean> {
    const result = await this.db.query(
      'UPDATE "Budgets" SET "Limit" = $3 WHERE "Id" = $1 AND "UserId" = $2',
      [budgetId, userId, limit.toString()],
    )
    return (result.rowCount ?? 0) > 0
  }

  async remove(userId: string, budgetId: string): Promise<boolean> {
    const result = await this.db.query(
      'DELETE FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2',
      [budgetId, userId],
    )
    return (result.rowCount ?? 0) > 0
  }
}

// --- goals ------------------------------------------------------------------

export interface Goal {
  id: string
  userId: string
  name: string
  targetAmount: Money
  currentAmount: Money
  targetDate: Instant | null
  color: string
}

const GOAL_COLUMNS =
  '"Id", "UserId", "Name", "TargetAmount", "CurrentAmount", "TargetDate", "Color"'

function toGoal(row: Record<string, unknown>): Goal {
  return {
    id: row.Id as string,
    userId: row.UserId as string,
    name: row.Name as string,
    targetAmount: readMoney(row.TargetAmount),
    currentAmount: readMoney(row.CurrentAmount),
    targetDate: readInstantOrNull(row.TargetDate),
    color: row.Color as string,
  }
}

export class GoalRepository {
  constructor(private readonly db: DbClient) {}

  async listAll(userId: string): Promise<Goal[]> {
    const result = await this.db.query(
      `SELECT ${GOAL_COLUMNS} FROM "Goals" WHERE "UserId" = $1 ORDER BY "Name"`,
      [userId],
    )
    return result.rows.map(toGoal)
  }

  async get(userId: string, goalId: string): Promise<Goal | null> {
    const result = await this.db.query(
      `SELECT ${GOAL_COLUMNS} FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2`,
      [goalId, userId],
    )
    return result.rows[0] ? toGoal(result.rows[0]) : null
  }

  async add(goal: Goal): Promise<void> {
    await this.db.query(
      `INSERT INTO "Goals" ("Id", "UserId", "Name", "TargetAmount", "CurrentAmount",
        "TargetDate", "Color") VALUES ($1, $2, $3, $4, $5, $6, $7)`,
      [
        goal.id,
        goal.userId,
        goal.name,
        goal.targetAmount.toString(),
        goal.currentAmount.toString(),
        goal.targetDate?.toParameter() ?? null,
        goal.color,
      ],
    )
  }

  async update(goal: Goal): Promise<void> {
    await this.db.query(
      `UPDATE "Goals" SET "Name" = $3, "TargetAmount" = $4, "CurrentAmount" = $5,
        "TargetDate" = $6, "Color" = $7 WHERE "Id" = $1 AND "UserId" = $2`,
      [
        goal.id,
        goal.userId,
        goal.name,
        goal.targetAmount.toString(),
        goal.currentAmount.toString(),
        goal.targetDate?.toParameter() ?? null,
        goal.color,
      ],
    )
  }

  async remove(userId: string, goalId: string): Promise<boolean> {
    const result = await this.db.query('DELETE FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2', [
      goalId,
      userId,
    ])
    return (result.rowCount ?? 0) > 0
  }
}

// --- recurring rules --------------------------------------------------------

export interface RecurringRule {
  id: string
  userId: string
  accountId: string
  categoryId: string | null
  type: EnumValue
  amount: Money
  description: string
  frequency: EnumValue
  startDate: Instant
  endDate: Instant | null
  nextRunDate: Instant
  isActive: boolean
}

const RECURRING_COLUMNS =
  '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Description", ' +
  '"Frequency", "StartDate", "EndDate", "NextRunDate", "IsActive"'

function toRecurring(row: Record<string, unknown>): RecurringRule {
  return {
    id: row.Id as string,
    userId: row.UserId as string,
    accountId: row.AccountId as string,
    categoryId: (row.CategoryId as string | null) ?? null,
    type: enumOf('TransactionType', Number(row.Type)),
    amount: readMoney(row.Amount),
    description: row.Description as string,
    frequency: enumOf('Frequency', Number(row.Frequency)),
    startDate: readInstant(row.StartDate),
    endDate: readInstantOrNull(row.EndDate),
    nextRunDate: readInstant(row.NextRunDate),
    isActive: row.IsActive as boolean,
  }
}

export class RecurringRepository {
  constructor(private readonly db: DbClient) {}

  async listAll(userId: string): Promise<RecurringRule[]> {
    const result = await this.db.query(
      `SELECT ${RECURRING_COLUMNS} FROM "RecurringRules" WHERE "UserId" = $1
       ORDER BY "NextRunDate"`,
      [userId],
    )
    return result.rows.map(toRecurring)
  }

  async get(userId: string, ruleId: string): Promise<RecurringRule | null> {
    const result = await this.db.query(
      `SELECT ${RECURRING_COLUMNS} FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2`,
      [ruleId, userId],
    )
    return result.rows[0] ? toRecurring(result.rows[0]) : null
  }

  /** Every active rule due at or before the cutoff, across all users. */
  async listDue(cutoff: Instant): Promise<RecurringRule[]> {
    const result = await this.db.query(
      `SELECT ${RECURRING_COLUMNS} FROM "RecurringRules"
       WHERE "IsActive" = true AND "NextRunDate" <= $1 ORDER BY "NextRunDate", "Id"`,
      [cutoff.toParameter()],
    )
    return result.rows.map(toRecurring)
  }

  async add(rule: RecurringRule): Promise<void> {
    await this.db.query(
      `INSERT INTO "RecurringRules" ("Id", "UserId", "AccountId", "CategoryId", "Type",
        "Amount", "Description", "Frequency", "StartDate", "EndDate", "NextRunDate",
        "IsActive") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
      [
        rule.id,
        rule.userId,
        rule.accountId,
        rule.categoryId,
        rule.type.ordinal,
        rule.amount.toString(),
        rule.description,
        rule.frequency.ordinal,
        rule.startDate.toParameter(),
        rule.endDate?.toParameter() ?? null,
        rule.nextRunDate.toParameter(),
        rule.isActive,
      ],
    )
  }

  async update(rule: RecurringRule): Promise<void> {
    await this.db.query(
      `UPDATE "RecurringRules" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5,
        "Amount" = $6, "Description" = $7, "Frequency" = $8, "StartDate" = $9,
        "EndDate" = $10, "NextRunDate" = $11, "IsActive" = $12
       WHERE "Id" = $1 AND "UserId" = $2`,
      [
        rule.id,
        rule.userId,
        rule.accountId,
        rule.categoryId,
        rule.type.ordinal,
        rule.amount.toString(),
        rule.description,
        rule.frequency.ordinal,
        rule.startDate.toParameter(),
        rule.endDate?.toParameter() ?? null,
        rule.nextRunDate.toParameter(),
        rule.isActive,
      ],
    )
  }

  async remove(userId: string, ruleId: string): Promise<boolean> {
    const result = await this.db.query(
      'DELETE FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2',
      [ruleId, userId],
    )
    return (result.rowCount ?? 0) > 0
  }
}
