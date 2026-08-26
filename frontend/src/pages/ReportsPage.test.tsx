import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import { renderWithProviders } from '@/test/utils'
import { ReportsPage } from './ReportsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

const monthlyReport = [
  { month: '2026-01', income: 4000, expenses: 2500, net: 1500 },
  { month: '2026-02', income: 3800, expenses: 4100, net: -300 },
]

const categoryReport = [
  {
    categoryId: 'c1',
    categoryName: 'Groceries',
    type: 'expense' as const,
    color: '#10b981',
    amount: 620,
  },
]

describe('ReportsPage', () => {
  beforeEach(() => seedApi({ monthlyReport, categoryReport }))

  it('totals the monthly rows', async () => {
    renderWithProviders(<ReportsPage />)

    expect(await screen.findByText('$7,800.00')).toBeInTheDocument()
    expect(screen.getByText('$6,600.00')).toBeInTheDocument()
    expect(screen.getByText('$1,200.00')).toBeInTheDocument()
    expect(screen.getByText('-$300.00')).toBeInTheDocument()
  })

  it('requests the selected year', async () => {
    renderWithProviders(<ReportsPage />)
    await screen.findByText('$7,800.00')

    const previousYear = String(new Date().getFullYear() - 1)
    await userEvent.selectOptions(screen.getByLabelText('Year'), previousYear)

    await waitFor(() =>
      expect(mocked.reports.monthly).toHaveBeenCalledWith(
        Number(previousYear),
        expect.anything(),
      ),
    )
  })

  it('renders the category breakdown', async () => {
    renderWithProviders(<ReportsPage />)

    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.getByText('$620.00')).toBeInTheDocument()
  })

  it('refetches the breakdown when the range changes', async () => {
    renderWithProviders(<ReportsPage />)
    await screen.findByText('Groceries')

    await userEvent.clear(screen.getByLabelText('From'))
    await userEvent.type(screen.getByLabelText('From'), '2026-01-01')

    await waitFor(() =>
      expect(mocked.reports.categories).toHaveBeenCalledWith(
        expect.objectContaining({ from: '2026-01-01' }),
        expect.anything(),
      ),
    )
  })

  it('shows empty states when a year has no data', async () => {
    seedApi()
    renderWithProviders(<ReportsPage />)

    expect(
      await screen.findByText('No activity in this range'),
    ).toBeInTheDocument()
    expect(
      screen.getAllByText(new RegExp(`Nothing recorded in ${new Date().getFullYear()}`))
        .length,
    ).toBeGreaterThan(0)
  })
})
