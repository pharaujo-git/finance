import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
} from '@tanstack/react-query'
import * as endpoints from './endpoints'
import type { TransactionFilters } from '@/types'

export const queryKeys = {
  me: ['me'] as const,
  accounts: ['accounts'] as const,
  categories: ['categories'] as const,
  transactions: (filters: TransactionFilters) =>
    ['transactions', filters] as const,
  recurring: ['recurring'] as const,
  budgets: (month: string) => ['budgets', month] as const,
  goals: ['goals'] as const,
  dashboard: ['dashboard'] as const,
  reports: ['reports'] as const,
}

/**
 * Everything that a write can invalidate. Mutations are infrequent and the
 * payloads are small, so refetching the affected roots keeps the UI honest
 * without hand-maintaining a dependency graph per endpoint.
 */
const WRITE_SCOPES = [
  queryKeys.accounts,
  queryKeys.categories,
  ['transactions'],
  queryKeys.recurring,
  ['budgets'],
  queryKeys.goals,
  queryKeys.dashboard,
  queryKeys.reports,
] as const

export function invalidateAll(client: QueryClient): void {
  for (const key of WRITE_SCOPES) {
    void client.invalidateQueries({ queryKey: key as readonly unknown[] })
  }
}

/** Mutation wrapper that refreshes derived data once the write lands. */
function useWrite<TVars, TData>(
  fn: (vars: TVars) => Promise<TData>,
): UseMutationResult<TData, Error, TVars> {
  const client = useQueryClient()
  return useMutation<TData, Error, TVars>({
    mutationFn: fn,
    onSuccess: () => invalidateAll(client),
  })
}

/* ------------------------------- accounts ------------------------------- */

export function useAccounts() {
  return useQuery({
    queryKey: queryKeys.accounts,
    queryFn: ({ signal }) => endpoints.accounts.list(signal),
  })
}

export const useCreateAccount = () => useWrite(endpoints.accounts.create)
export const useUpdateAccount = () =>
  useWrite((vars: { id: string; body: Parameters<typeof endpoints.accounts.update>[1] }) =>
    endpoints.accounts.update(vars.id, vars.body),
  )
export const useArchiveAccount = () => useWrite(endpoints.accounts.archive)

/* ------------------------------ categories ------------------------------ */

export function useCategories() {
  return useQuery({
    queryKey: queryKeys.categories,
    queryFn: ({ signal }) => endpoints.categories.list(signal),
  })
}

export const useCreateCategory = () => useWrite(endpoints.categories.create)
export const useUpdateCategory = () =>
  useWrite((vars: { id: string; body: Parameters<typeof endpoints.categories.update>[1] }) =>
    endpoints.categories.update(vars.id, vars.body),
  )
export const useDeleteCategory = () => useWrite(endpoints.categories.remove)

/* ----------------------------- transactions ----------------------------- */

export function useTransactions(filters: TransactionFilters) {
  return useQuery({
    queryKey: queryKeys.transactions(filters),
    queryFn: ({ signal }) => endpoints.transactions.list(filters, signal),
    placeholderData: (previous) => previous,
  })
}

export const useCreateTransaction = () => useWrite(endpoints.transactions.create)
export const useUpdateTransaction = () =>
  useWrite((vars: { id: string; body: Parameters<typeof endpoints.transactions.update>[1] }) =>
    endpoints.transactions.update(vars.id, vars.body),
  )
export const useDeleteTransaction = () => useWrite(endpoints.transactions.remove)
export const useImportTransactions = () => useWrite(endpoints.transactions.importCsv)

/* ------------------------------- recurring ------------------------------ */

export function useRecurring() {
  return useQuery({
    queryKey: queryKeys.recurring,
    queryFn: ({ signal }) => endpoints.recurring.list(signal),
  })
}

export const useCreateRecurring = () => useWrite(endpoints.recurring.create)
export const useUpdateRecurring = () =>
  useWrite((vars: { id: string; body: Parameters<typeof endpoints.recurring.update>[1] }) =>
    endpoints.recurring.update(vars.id, vars.body),
  )
export const useDeleteRecurring = () => useWrite(endpoints.recurring.remove)

/* -------------------------------- budgets ------------------------------- */

export function useBudgets(month: string) {
  return useQuery({
    queryKey: queryKeys.budgets(month),
    queryFn: ({ signal }) => endpoints.budgets.list(month, signal),
  })
}

export const useCreateBudget = () => useWrite(endpoints.budgets.create)
export const useUpdateBudget = () =>
  useWrite((vars: { id: string; limit: number }) =>
    endpoints.budgets.update(vars.id, { limit: vars.limit }),
  )
export const useDeleteBudget = () => useWrite(endpoints.budgets.remove)

/* --------------------------------- goals -------------------------------- */

export function useGoals() {
  return useQuery({
    queryKey: queryKeys.goals,
    queryFn: ({ signal }) => endpoints.goals.list(signal),
  })
}

export const useCreateGoal = () => useWrite(endpoints.goals.create)
export const useUpdateGoal = () =>
  useWrite((vars: { id: string; body: Parameters<typeof endpoints.goals.update>[1] }) =>
    endpoints.goals.update(vars.id, vars.body),
  )
export const useDeleteGoal = () => useWrite(endpoints.goals.remove)
export const useContributeGoal = () =>
  useWrite((vars: { id: string; amount: number }) =>
    endpoints.goals.contribute(vars.id, vars.amount),
  )

/* ------------------------------- dashboard ------------------------------ */

export function useDashboardSummary() {
  return useQuery({
    queryKey: [...queryKeys.dashboard, 'summary'],
    queryFn: ({ signal }) => endpoints.dashboard.summary(signal),
  })
}

export function useNetWorthSeries(months = 12) {
  return useQuery({
    queryKey: [...queryKeys.dashboard, 'networth', months],
    queryFn: ({ signal }) => endpoints.dashboard.netWorth(months, signal),
  })
}

export function useCashFlowSeries(months = 6) {
  return useQuery({
    queryKey: [...queryKeys.dashboard, 'cashflow', months],
    queryFn: ({ signal }) => endpoints.dashboard.cashFlow(months, signal),
  })
}

export function useSpendingByCategory(month: string) {
  return useQuery({
    queryKey: [...queryKeys.dashboard, 'spending', month],
    queryFn: ({ signal }) => endpoints.dashboard.spending(month, signal),
  })
}

/* -------------------------------- reports ------------------------------- */

export function useMonthlyReport(year: number) {
  return useQuery({
    queryKey: [...queryKeys.reports, 'monthly', year],
    queryFn: ({ signal }) => endpoints.reports.monthly(year, signal),
  })
}

export function useCategoryReport(range: { from: string; to: string }) {
  return useQuery({
    queryKey: [...queryKeys.reports, 'categories', range.from, range.to],
    queryFn: ({ signal }) => endpoints.reports.categories(range, signal),
  })
}
