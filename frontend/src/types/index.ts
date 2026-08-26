export type AccountType =
  | 'checking'
  | 'savings'
  | 'creditCard'
  | 'cash'
  | 'investment'

export type TransactionType = 'income' | 'expense' | 'transfer'
export type CategoryType = 'income' | 'expense'
export type Frequency = 'daily' | 'weekly' | 'monthly' | 'yearly'

export interface User {
  id: string
  email: string
  name: string
  currency: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Account {
  id: string
  name: string
  type: AccountType
  balance: number
  currency: string
  isArchived: boolean
  createdAt: string
}

export interface AccountInput {
  name: string
  type: AccountType
  initialBalance: number
  currency: string
}

export interface AccountUpdate {
  name: string
  type: AccountType
  currency: string
  isArchived: boolean
}

export interface Category {
  id: string
  name: string
  type: CategoryType
  icon: string
  color: string
  isDefault: boolean
}

export interface CategoryInput {
  name: string
  type: CategoryType
  icon: string
  color: string
}

export interface Transaction {
  id: string
  accountId: string
  categoryId: string | null
  type: TransactionType
  amount: number
  date: string
  description: string
  notes: string | null
  tags: string[]
  transferAccountId: string | null
}

export type TransactionInput = Omit<Transaction, 'id'>

export interface Paged<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface TransactionFilters {
  page?: number
  pageSize?: number
  accountId?: string
  categoryId?: string
  type?: TransactionType | ''
  from?: string
  to?: string
  search?: string
}

export interface ImportResult {
  imported: number
  skipped: number
}

export interface RecurringRule {
  id: string
  accountId: string
  categoryId: string | null
  type: TransactionType
  amount: number
  description: string
  frequency: Frequency
  startDate: string
  endDate: string | null
  nextRunDate: string
  isActive: boolean
}

export type RecurringInput = Omit<RecurringRule, 'id' | 'nextRunDate'>

export interface Budget {
  id: string
  categoryId: string
  month: string
  limit: number
  spent: number
  remaining: number
}

export interface BudgetInput {
  categoryId: string
  month: string
  limit: number
}

export interface Goal {
  id: string
  name: string
  targetAmount: number
  currentAmount: number
  targetDate: string | null
  color: string
}

export type GoalInput = Omit<Goal, 'id' | 'currentAmount'>

export interface DashboardSummary {
  netWorth: number
  totalIncome: number
  totalExpenses: number
  savingsRate: number
}

export interface NetWorthPoint {
  month: string
  value: number
}

export interface CashFlowPoint {
  month: string
  income: number
  expenses: number
}

export interface SpendingSlice {
  categoryId: string
  categoryName: string
  color: string
  amount: number
}

export interface MonthlyReportRow {
  month: string
  income: number
  expenses: number
  net: number
}

export interface CategoryReportRow {
  categoryId: string
  categoryName: string
  type: CategoryType
  color: string
  amount: number
}
