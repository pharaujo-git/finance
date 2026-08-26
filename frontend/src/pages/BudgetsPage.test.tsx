import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import { budget, category, renderWithProviders } from '@/test/utils'
import { BudgetsPage } from './BudgetsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

const categories = [
  category({ id: 'c1', name: 'Groceries', type: 'expense' }),
  category({ id: 'c2', name: 'Salary', type: 'income' }),
]

describe('BudgetsPage', () => {
  beforeEach(() => seedApi({ categories }))

  it('summarises the month and lists each budget', async () => {
    seedApi({
      categories,
      budgets: [
        budget({ id: 'b1', categoryId: 'c1', limit: 400, spent: 150, remaining: 250 }),
        budget({ id: 'b2', categoryId: 'c2', limit: 200, spent: 260, remaining: -60 }),
      ],
    })
    renderWithProviders(<BudgetsPage />)

    expect(await screen.findByText('$600.00')).toBeInTheDocument()
    expect(screen.getByText('$410.00')).toBeInTheDocument()
    expect(screen.getByText('over budget')).toBeInTheDocument()
    expect(screen.getByText('on track')).toBeInTheDocument()
  })

  it('offers only expense categories when adding a budget', async () => {
    renderWithProviders(<BudgetsPage />)
    await screen.findByText('No budgets for this month')

    await userEvent.click(screen.getByRole('button', { name: 'Add budget' }))

    expect(screen.getByRole('option', { name: 'Groceries' })).toBeInTheDocument()
    expect(
      screen.queryByRole('option', { name: 'Salary' }),
    ).not.toBeInTheDocument()
  })

  it('creates a budget for the selected month', async () => {
    renderWithProviders(<BudgetsPage />)
    await screen.findByText('No budgets for this month')

    fireEvent.change(screen.getByLabelText('Month'), {
      target: { value: '2024-05' },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Add budget' }))
    await userEvent.selectOptions(screen.getByLabelText('Category'), 'c1')
    await userEvent.type(screen.getByLabelText('Monthly limit'), '350')
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add budget' })[1],
    )

    await waitFor(() =>
      expect(mocked.budgets.create).toHaveBeenCalledWith(
        { categoryId: 'c1', limit: 350, month: '2024-05' },
        expect.anything(),
      ),
    )
  })

  it('validates the limit', async () => {
    renderWithProviders(<BudgetsPage />)
    await screen.findByText('No budgets for this month')

    await userEvent.click(screen.getByRole('button', { name: 'Add budget' }))
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add budget' })[1],
    )

    expect(await screen.findByText('Category is required')).toBeInTheDocument()
    expect(screen.getByText('Amount is required')).toBeInTheDocument()
    expect(mocked.budgets.create).not.toHaveBeenCalled()
  })

  it('updates only the limit when editing', async () => {
    seedApi({ categories, budgets: [budget()] })
    renderWithProviders(<BudgetsPage />)
    await screen.findByText('Groceries')

    await userEvent.click(
      screen.getByRole('button', { name: /edit budget for groceries/i }),
    )
    await userEvent.clear(screen.getByLabelText('Monthly limit'))
    await userEvent.type(screen.getByLabelText('Monthly limit'), '500')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(mocked.budgets.update).toHaveBeenCalledWith('b1', { limit: 500 }),
    )
  })

  it('deletes a budget after confirmation', async () => {
    seedApi({ categories, budgets: [budget()] })
    renderWithProviders(<BudgetsPage />)
    await screen.findByText('Groceries')

    await userEvent.click(
      screen.getByRole('button', { name: /delete budget for groceries/i }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() =>
      expect(mocked.budgets.remove).toHaveBeenCalledWith(
        'b1',
        expect.anything(),
      ),
    )
  })
})
