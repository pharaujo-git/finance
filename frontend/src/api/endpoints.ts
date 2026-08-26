import { api, toQuery } from '@/lib/api'
import type {
  Account,
  AccountInput,
  AccountUpdate,
  AuthResponse,
  Budget,
  BudgetInput,
  CashFlowPoint,
  Category,
  CategoryInput,
  CategoryReportRow,
  DashboardSummary,
  Goal,
  GoalInput,
  ImportResult,
  MonthlyReportRow,
  NetWorthPoint,
  Paged,
  RecurringInput,
  RecurringRule,
  SpendingSlice,
  Transaction,
  TransactionFilters,
  TransactionInput,
  User,
} from '@/types'

export const auth = {
  register: (body: { email: string; password: string; name: string }) =>
    api.post<AuthResponse>('/auth/register', body),
  login: (body: { email: string; password: string }) =>
    api.post<AuthResponse>('/auth/login', body),
  me: (signal?: AbortSignal) => api.get<User>('/auth/me', signal),
  updateMe: (body: { name: string; currency: string }) =>
    api.put<User>('/auth/me', body),
}

export const accounts = {
  list: (signal?: AbortSignal) => api.get<Account[]>('/accounts', signal),
  create: (body: AccountInput) => api.post<Account>('/accounts', body),
  update: (id: string, body: AccountUpdate) =>
    api.put<Account>(`/accounts/${id}`, body),
  archive: (id: string) => api.del<void>(`/accounts/${id}`),
}

export const categories = {
  list: (signal?: AbortSignal) => api.get<Category[]>('/categories', signal),
  create: (body: CategoryInput) => api.post<Category>('/categories', body),
  update: (id: string, body: CategoryInput) =>
    api.put<Category>(`/categories/${id}`, body),
  remove: (id: string) => api.del<void>(`/categories/${id}`),
}

export const transactions = {
  list: (filters: TransactionFilters, signal?: AbortSignal) =>
    api.get<Paged<Transaction>>(
      `/transactions${toQuery({ ...filters })}`,
      signal,
    ),
  create: (body: TransactionInput) =>
    api.post<Transaction>('/transactions', body),
  update: (id: string, body: TransactionInput) =>
    api.put<Transaction>(`/transactions/${id}`, body),
  remove: (id: string) => api.del<void>(`/transactions/${id}`),
  exportCsv: (range: { from?: string; to?: string }) =>
    api.blob(`/transactions/export${toQuery({ ...range })}`),
  importCsv: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.upload<ImportResult>('/transactions/import', form)
  },
}

export const recurring = {
  list: (signal?: AbortSignal) => api.get<RecurringRule[]>('/recurring', signal),
  create: (body: RecurringInput) => api.post<RecurringRule>('/recurring', body),
  update: (id: string, body: RecurringInput) =>
    api.put<RecurringRule>(`/recurring/${id}`, body),
  remove: (id: string) => api.del<void>(`/recurring/${id}`),
}

export const budgets = {
  list: (month: string, signal?: AbortSignal) =>
    api.get<Budget[]>(`/budgets${toQuery({ month })}`, signal),
  create: (body: BudgetInput) => api.post<Budget>('/budgets', body),
  update: (id: string, body: { limit: number }) =>
    api.put<Budget>(`/budgets/${id}`, body),
  remove: (id: string) => api.del<void>(`/budgets/${id}`),
}

export const goals = {
  list: (signal?: AbortSignal) => api.get<Goal[]>('/goals', signal),
  create: (body: GoalInput) => api.post<Goal>('/goals', body),
  update: (id: string, body: GoalInput) => api.put<Goal>(`/goals/${id}`, body),
  remove: (id: string) => api.del<void>(`/goals/${id}`),
  contribute: (id: string, amount: number) =>
    api.post<Goal>(`/goals/${id}/contribute`, { amount }),
}

export const dashboard = {
  summary: (signal?: AbortSignal) =>
    api.get<DashboardSummary>('/dashboard/summary', signal),
  netWorth: (months: number, signal?: AbortSignal) =>
    api.get<NetWorthPoint[]>(
      `/dashboard/networth${toQuery({ months })}`,
      signal,
    ),
  cashFlow: (months: number, signal?: AbortSignal) =>
    api.get<CashFlowPoint[]>(
      `/dashboard/cashflow${toQuery({ months })}`,
      signal,
    ),
  spending: (month: string, signal?: AbortSignal) =>
    api.get<SpendingSlice[]>(`/dashboard/spending${toQuery({ month })}`, signal),
}

export const reports = {
  monthly: (year: number, signal?: AbortSignal) =>
    api.get<MonthlyReportRow[]>(`/reports/monthly${toQuery({ year })}`, signal),
  categories: (range: { from: string; to: string }, signal?: AbortSignal) =>
    api.get<CategoryReportRow[]>(
      `/reports/categories${toQuery({ ...range })}`,
      signal,
    ),
}
