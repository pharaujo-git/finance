import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, type RenderOptions } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { AuthContext, type AuthContextValue } from '@/context/auth-context'
import { ThemeProvider } from '@/context/ThemeProvider'
import { ToastProvider } from '@/context/ToastProvider'
import type {
  Account,
  Budget,
  Category,
  Goal,
  RecurringRule,
  Transaction,
  User,
} from '@/types'

export const testUser: User = {
  id: 'u1',
  email: 'alex@example.com',
  name: 'Alex Morgan',
  currency: 'USD',
}

export function makeAuth(
  overrides: Partial<AuthContextValue> = {},
): AuthContextValue {
  return {
    user: testUser,
    isLoading: false,
    isAuthenticated: true,
    login: async () => {},
    register: async () => {},
    logout: () => {},
    setUser: () => {},
    currency: 'USD',
    ...overrides,
  }
}

export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  })
}

export interface ProviderOptions extends Omit<RenderOptions, 'wrapper'> {
  route?: string
  auth?: Partial<AuthContextValue>
  queryClient?: QueryClient
}

export function renderWithProviders(
  ui: ReactElement,
  { route = '/', auth, queryClient, ...options }: ProviderOptions = {},
) {
  const client = queryClient ?? createTestQueryClient()
  const value = makeAuth(auth)

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <ThemeProvider>
        <QueryClientProvider client={client}>
          <MemoryRouter initialEntries={[route]}>
            <AuthContext value={value}>
              <ToastProvider>{children}</ToastProvider>
            </AuthContext>
          </MemoryRouter>
        </QueryClientProvider>
      </ThemeProvider>
    )
  }

  return { ...render(ui, { wrapper: Wrapper, ...options }), queryClient: client }
}

/* --------------------------- fixture factories --------------------------- */

export const account = (overrides: Partial<Account> = {}): Account => ({
  id: 'a1',
  name: 'Everyday checking',
  type: 'checking',
  balance: 2500,
  currency: 'USD',
  isArchived: false,
  createdAt: '2024-01-05',
  ...overrides,
})

export const category = (overrides: Partial<Category> = {}): Category => ({
  id: 'c1',
  name: 'Groceries',
  type: 'expense',
  icon: 'cash',
  color: '#10b981',
  isDefault: true,
  ...overrides,
})

export const transaction = (
  overrides: Partial<Transaction> = {},
): Transaction => ({
  id: 't1',
  accountId: 'a1',
  categoryId: 'c1',
  type: 'expense',
  amount: 42.5,
  date: '2024-05-17',
  description: 'Supermarket run',
  notes: null,
  tags: ['weekly'],
  transferAccountId: null,
  ...overrides,
})

export const budget = (overrides: Partial<Budget> = {}): Budget => ({
  id: 'b1',
  categoryId: 'c1',
  month: '2024-05',
  limit: 400,
  spent: 150,
  remaining: 250,
  ...overrides,
})

export const goal = (overrides: Partial<Goal> = {}): Goal => ({
  id: 'g1',
  name: 'Emergency fund',
  targetAmount: 5000,
  currentAmount: 1250,
  targetDate: '2025-12-31',
  color: '#16a34a',
  ...overrides,
})

export const recurring = (
  overrides: Partial<RecurringRule> = {},
): RecurringRule => ({
  id: 'r1',
  accountId: 'a1',
  categoryId: 'c1',
  type: 'expense',
  amount: 1200,
  description: 'Rent',
  frequency: 'monthly',
  startDate: '2024-01-01',
  endDate: null,
  nextRunDate: '2024-06-01',
  isActive: true,
  ...overrides,
})
