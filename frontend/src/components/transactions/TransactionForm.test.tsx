import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Lookups } from '@/hooks/useLookups'
import { account, category, renderWithProviders, transaction } from '@/test/utils'
import { TransactionForm, toTransactionInput } from './TransactionForm'

const accounts = [
  account({ id: 'a1', name: 'Checking' }),
  account({ id: 'a2', name: 'Savings', type: 'savings' }),
]
const categories = [
  category({ id: 'c1', name: 'Groceries', type: 'expense' }),
  category({ id: 'c2', name: 'Salary', type: 'income' }),
]

const lookups: Lookups = {
  accounts,
  activeAccounts: accounts,
  categories,
  accountName: (id) => accounts.find((item) => item.id === id)?.name ?? '—',
  category: (id) => categories.find((item) => item.id === id),
  categoryName: (id) => categories.find((item) => item.id === id)?.name ?? '—',
  isLoading: false,
}

function setup(overrides: { transaction?: ReturnType<typeof transaction> | null } = {}) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  const onClose = vi.fn()
  renderWithProviders(
    <TransactionForm
      open
      transaction={overrides.transaction ?? null}
      lookups={lookups}
      onClose={onClose}
      onSubmit={onSubmit}
    />,
  )
  return { onSubmit, onClose }
}

const submitButton = () =>
  screen.getByRole('button', { name: /add transaction|save changes/i })

describe('toTransactionInput', () => {
  it('maps an expense, parsing the amount and tags', () => {
    expect(
      toTransactionInput({
        type: 'expense',
        accountId: 'a1',
        transferAccountId: '',
        categoryId: 'c1',
        amount: '12.50',
        date: '2024-05-17',
        description: '  Coffee  ',
        notes: '',
        tags: 'cafe, weekly',
      }),
    ).toEqual({
      accountId: 'a1',
      categoryId: 'c1',
      type: 'expense',
      amount: 12.5,
      date: '2024-05-17',
      description: 'Coffee',
      notes: null,
      tags: ['cafe', 'weekly'],
      transferAccountId: null,
    })
  })

  it('drops the category and keeps the destination for a transfer', () => {
    const input = toTransactionInput({
      type: 'transfer',
      accountId: 'a1',
      transferAccountId: 'a2',
      categoryId: 'c1',
      amount: '100',
      date: '2024-05-17',
      description: 'Move to savings',
      notes: 'monthly sweep',
      tags: '',
    })

    expect(input.categoryId).toBeNull()
    expect(input.transferAccountId).toBe('a2')
    expect(input.notes).toBe('monthly sweep')
  })
})

describe('TransactionForm validation', () => {
  it('requires an amount, description and category', async () => {
    const { onSubmit } = setup()
    await userEvent.click(submitButton())

    expect(await screen.findByText('Amount is required')).toBeInTheDocument()
    expect(screen.getByText('Description is required')).toBeInTheDocument()
    expect(screen.getByText('Category is required')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('rejects a zero amount', async () => {
    const { onSubmit } = setup()
    await userEvent.type(screen.getByLabelText('Amount'), '0')
    await userEvent.type(screen.getByLabelText('Description'), 'Nothing')
    await userEvent.selectOptions(screen.getByLabelText('Category'), 'c1')
    await userEvent.click(submitButton())

    expect(
      await screen.findByText('Amount must be greater than zero'),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('requires a destination account for transfers', async () => {
    const { onSubmit } = setup()
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'transfer')
    await userEvent.type(screen.getByLabelText('Amount'), '50')
    await userEvent.type(screen.getByLabelText('Description'), 'Sweep')
    await userEvent.click(submitButton())

    expect(
      await screen.findByText('Choose the account the money moves to'),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('rejects a transfer to the same account', async () => {
    const { onSubmit } = setup()
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'transfer')
    await userEvent.selectOptions(screen.getByLabelText('From account'), 'a1')
    await userEvent.selectOptions(screen.getByLabelText('To account'), 'a1')
    await userEvent.type(screen.getByLabelText('Amount'), '50')
    await userEvent.type(screen.getByLabelText('Description'), 'Sweep')
    await userEvent.click(submitButton())

    expect(
      await screen.findByText('Pick a different destination account'),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('submits a valid expense', async () => {
    const { onSubmit } = setup()
    await userEvent.type(screen.getByLabelText('Amount'), '24.99')
    await userEvent.selectOptions(screen.getByLabelText('Category'), 'c1')
    await userEvent.type(screen.getByLabelText('Description'), 'Weekly shop')
    await userEvent.type(screen.getByLabelText('Tags'), 'food, weekly')
    await userEvent.click(submitButton())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce())
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      type: 'expense',
      amount: 24.99,
      categoryId: 'c1',
      description: 'Weekly shop',
      tags: ['food', 'weekly'],
      transferAccountId: null,
    })
  })

  it('shows income categories when the type switches', async () => {
    setup()
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'income')
    expect(
      screen.getByRole('option', { name: 'Salary' }),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('option', { name: 'Groceries' }),
    ).not.toBeInTheDocument()
  })

  it('prefills the form when editing', () => {
    setup({ transaction: transaction({ description: 'Supermarket run' }) })
    expect(screen.getByLabelText('Description')).toHaveValue('Supermarket run')
    expect(screen.getByLabelText('Amount')).toHaveValue(42.5)
    expect(screen.getByLabelText('Tags')).toHaveValue('weekly')
    expect(
      screen.getByRole('button', { name: 'Save changes' }),
    ).toBeInTheDocument()
  })

  it('closes without submitting when cancelled', async () => {
    const { onClose, onSubmit } = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalledOnce()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
