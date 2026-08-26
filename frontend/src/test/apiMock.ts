import { vi } from 'vitest'
import type {
  Account,
  Budget,
  CashFlowPoint,
  Category,
  CategoryReportRow,
  DashboardSummary,
  Goal,
  MonthlyReportRow,
  NetWorthPoint,
  Paged,
  RecurringRule,
  SpendingSlice,
  Transaction,
} from '@/types'

/**
 * Stand-in for `@/api/endpoints`. Test files opt in with:
 *   vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)
 */
export const mocked = {
  auth: {
    register: vi.fn(),
    login: vi.fn(),
    me: vi.fn(),
    updateMe: vi.fn(),
  },
  accounts: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    archive: vi.fn(),
  },
  categories: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
  transactions: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    exportCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  recurring: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
  budgets: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
  goals: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    contribute: vi.fn(),
  },
  dashboard: {
    summary: vi.fn(),
    netWorth: vi.fn(),
    cashFlow: vi.fn(),
    spending: vi.fn(),
  },
  reports: {
    monthly: vi.fn(),
    categories: vi.fn(),
  },
}

export interface ApiSeed {
  accounts?: Account[]
  categories?: Category[]
  transactions?: Paged<Transaction>
  recurring?: RecurringRule[]
  budgets?: Budget[]
  goals?: Goal[]
  summary?: DashboardSummary
  netWorth?: NetWorthPoint[]
  cashFlow?: CashFlowPoint[]
  spending?: SpendingSlice[]
  monthlyReport?: MonthlyReportRow[]
  categoryReport?: CategoryReportRow[]
}

const emptyPage: Paged<Transaction> = {
  items: [],
  total: 0,
  page: 1,
  pageSize: 20,
}

/** Clears every mock and points the read endpoints at `seed`. */
export function seedApi(seed: ApiSeed = {}): void {
  for (const group of Object.values(mocked)) {
    for (const fn of Object.values(group)) fn.mockReset()
  }

  mocked.accounts.list.mockResolvedValue(seed.accounts ?? [])
  mocked.categories.list.mockResolvedValue(seed.categories ?? [])
  mocked.transactions.list.mockResolvedValue(seed.transactions ?? emptyPage)
  mocked.recurring.list.mockResolvedValue(seed.recurring ?? [])
  mocked.budgets.list.mockResolvedValue(seed.budgets ?? [])
  mocked.goals.list.mockResolvedValue(seed.goals ?? [])
  mocked.dashboard.summary.mockResolvedValue(
    seed.summary ?? {
      netWorth: 0,
      totalIncome: 0,
      totalExpenses: 0,
      savingsRate: 0,
    },
  )
  mocked.dashboard.netWorth.mockResolvedValue(seed.netWorth ?? [])
  mocked.dashboard.cashFlow.mockResolvedValue(seed.cashFlow ?? [])
  mocked.dashboard.spending.mockResolvedValue(seed.spending ?? [])
  mocked.reports.monthly.mockResolvedValue(seed.monthlyReport ?? [])
  mocked.reports.categories.mockResolvedValue(seed.categoryReport ?? [])

  mocked.accounts.create.mockResolvedValue(seed.accounts?.[0])
  mocked.accounts.update.mockResolvedValue(seed.accounts?.[0])
  mocked.accounts.archive.mockResolvedValue(undefined)
  mocked.categories.create.mockResolvedValue(seed.categories?.[0])
  mocked.categories.update.mockResolvedValue(seed.categories?.[0])
  mocked.categories.remove.mockResolvedValue(undefined)
  mocked.transactions.create.mockResolvedValue(seed.transactions?.items[0])
  mocked.transactions.update.mockResolvedValue(seed.transactions?.items[0])
  mocked.transactions.remove.mockResolvedValue(undefined)
  mocked.transactions.importCsv.mockResolvedValue({ imported: 3, skipped: 1 })
  mocked.transactions.exportCsv.mockResolvedValue(
    new Blob(['date,amount'], { type: 'text/csv' }),
  )
  mocked.recurring.create.mockResolvedValue(seed.recurring?.[0])
  mocked.recurring.update.mockResolvedValue(seed.recurring?.[0])
  mocked.recurring.remove.mockResolvedValue(undefined)
  mocked.budgets.create.mockResolvedValue(seed.budgets?.[0])
  mocked.budgets.update.mockResolvedValue(seed.budgets?.[0])
  mocked.budgets.remove.mockResolvedValue(undefined)
  mocked.goals.create.mockResolvedValue(seed.goals?.[0])
  mocked.goals.update.mockResolvedValue(seed.goals?.[0])
  mocked.goals.remove.mockResolvedValue(undefined)
  mocked.goals.contribute.mockResolvedValue(seed.goals?.[0])
  mocked.auth.updateMe.mockResolvedValue({
    id: 'u1',
    email: 'alex@example.com',
    name: 'Alex Morgan',
    currency: 'USD',
  })
}
