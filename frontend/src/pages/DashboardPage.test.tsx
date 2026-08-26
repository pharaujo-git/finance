import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import {
  account,
  budget,
  category,
  renderWithProviders,
  transaction,
} from '@/test/utils'
import { DashboardPage } from './DashboardPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

describe('DashboardPage', () => {
  beforeEach(() => {
    seedApi({
      accounts: [account()],
      categories: [category()],
      summary: {
        netWorth: 12500,
        totalIncome: 4200,
        totalExpenses: 2750,
        savingsRate: 34.5,
      },
      netWorth: [
        { month: '2024-04', value: 12000 },
        { month: '2024-05', value: 12500 },
      ],
      cashFlow: [{ month: '2024-05', income: 4200, expenses: 2750 }],
      spending: [
        {
          categoryId: 'c1',
          categoryName: 'Groceries',
          color: '#10b981',
          amount: 320,
        },
      ],
      budgets: [budget()],
      transactions: {
        items: [transaction({ description: 'Supermarket run' })],
        total: 1,
        page: 1,
        pageSize: 6,
      },
    })
  })

  it('greets the user and renders the summary cards', async () => {
    renderWithProviders(<DashboardPage />)

    expect(await screen.findByText('Hello, Alex')).toBeInTheDocument()
    expect(await screen.findByText('$12,500.00')).toBeInTheDocument()
    expect(screen.getByText('$4,200.00')).toBeInTheDocument()
    expect(screen.getByText('$2,750.00')).toBeInTheDocument()
    expect(screen.getByText('34.5%')).toBeInTheDocument()
  })

  it('lists the most recent transactions', async () => {
    renderWithProviders(<DashboardPage />)

    expect(await screen.findByText('Supermarket run')).toBeInTheDocument()
    expect(screen.getByText('-$42.50')).toBeInTheDocument()
  })

  it('shows budget progress for the current month', async () => {
    renderWithProviders(<DashboardPage />)

    expect(await screen.findByText('on track')).toBeInTheDocument()
    expect(screen.getAllByText('Groceries').length).toBeGreaterThan(0)
    expect(screen.getByRole('progressbar', { name: /groceries usage/i })).toBeInTheDocument()
  })

  it('requests the dashboard endpoints with the documented windows', async () => {
    renderWithProviders(<DashboardPage />)
    await screen.findByText('Hello, Alex')

    expect(mocked.dashboard.netWorth).toHaveBeenCalledWith(
      12,
      expect.anything(),
    )
    expect(mocked.dashboard.cashFlow).toHaveBeenCalledWith(6, expect.anything())
    expect(mocked.dashboard.spending).toHaveBeenCalledWith(
      expect.stringMatching(/^\d{4}-\d{2}$/),
      expect.anything(),
    )
  })

  it('shows empty states when there is nothing to show', async () => {
    seedApi()
    renderWithProviders(<DashboardPage />)

    expect(await screen.findByText('Nothing recorded yet')).toBeInTheDocument()
    expect(screen.getByText('No budgets set')).toBeInTheDocument()
    expect(screen.getByText('No spending this month')).toBeInTheDocument()
  })

  it('surfaces an error when the summary fails', async () => {
    seedApi()
    mocked.dashboard.summary.mockRejectedValue(new Error('Service unavailable'))
    renderWithProviders(<DashboardPage />)

    expect(await screen.findByText('Service unavailable')).toBeInTheDocument()
  })
})
