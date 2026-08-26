import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Lookups } from '@/hooks/useLookups'
import { account, category, recurring, renderWithProviders } from '@/test/utils'
import { RecurringForm, toRecurringInput } from './RecurringForm'

const accounts = [account({ id: 'a1', name: 'Checking' })]
const categories = [
  category({ id: 'c1', name: 'Housing', type: 'expense' }),
  category({ id: 'c2', name: 'Salary', type: 'income' }),
]

const lookups: Lookups = {
  accounts,
  activeAccounts: accounts,
  categories,
  accountName: () => 'Checking',
  category: (id) => categories.find((item) => item.id === id),
  categoryName: () => 'Housing',
  isLoading: false,
}

function setup(rule: ReturnType<typeof recurring> | null = null) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  renderWithProviders(
    <RecurringForm
      open
      rule={rule}
      lookups={lookups}
      onClose={vi.fn()}
      onSubmit={onSubmit}
    />,
  )
  return { onSubmit }
}

describe('toRecurringInput', () => {
  it('normalises optional fields', () => {
    expect(
      toRecurringInput({
        type: 'expense',
        accountId: 'a1',
        categoryId: '',
        amount: '1200',
        description: ' Rent ',
        frequency: 'monthly',
        startDate: '2024-01-01',
        endDate: '',
        isActive: true,
      }),
    ).toEqual({
      accountId: 'a1',
      categoryId: null,
      type: 'expense',
      amount: 1200,
      description: 'Rent',
      frequency: 'monthly',
      startDate: '2024-01-01',
      endDate: null,
      isActive: true,
    })
  })
})

describe('RecurringForm', () => {
  it('requires an amount and description', async () => {
    const { onSubmit } = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(await screen.findByText('Amount is required')).toBeInTheDocument()
    expect(screen.getByText('Description is required')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('rejects an end date before the start date', async () => {
    const { onSubmit } = setup()
    await userEvent.type(screen.getByLabelText('Amount'), '50')
    await userEvent.type(screen.getByLabelText('Description'), 'Gym')
    fireEvent.change(screen.getByLabelText('Starts'), {
      target: { value: '2026-06-01' },
    })
    fireEvent.change(screen.getByLabelText('Ends (optional)'), {
      target: { value: '2026-01-01' },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(
      await screen.findByText('End date must be after the start date'),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('submits a valid weekly rule', async () => {
    const { onSubmit } = setup()
    await userEvent.type(screen.getByLabelText('Amount'), '15')
    await userEvent.type(screen.getByLabelText('Description'), 'Streaming')
    await userEvent.selectOptions(screen.getByLabelText('Frequency'), 'weekly')
    await userEvent.selectOptions(screen.getByLabelText('Category'), 'c1')
    await userEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce())
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      amount: 15,
      description: 'Streaming',
      frequency: 'weekly',
      categoryId: 'c1',
      isActive: true,
    })
  })

  it('swaps the category list when the type changes', async () => {
    setup()
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'income')
    expect(screen.getByRole('option', { name: 'Salary' })).toBeInTheDocument()
    expect(
      screen.queryByRole('option', { name: 'Housing' }),
    ).not.toBeInTheDocument()
  })

  it('prefills when editing an existing rule', () => {
    setup(recurring())
    expect(screen.getByLabelText('Description')).toHaveValue('Rent')
    expect(screen.getByLabelText('Amount')).toHaveValue(1200)
    expect(screen.getByLabelText('Active')).toBeChecked()
    expect(
      screen.getByRole('button', { name: 'Save changes' }),
    ).toBeInTheDocument()
  })
})
