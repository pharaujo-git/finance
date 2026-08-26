import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import { account, renderWithProviders } from '@/test/utils'
import { AccountsPage } from './AccountsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

describe('AccountsPage', () => {
  beforeEach(() => seedApi())

  it('shows a skeleton then the account cards', async () => {
    seedApi({
      accounts: [
        account({ id: 'a1', name: 'Everyday checking', balance: 2500 }),
        account({
          id: 'a2',
          name: 'Travel card',
          type: 'creditCard',
          balance: -320,
        }),
      ],
    })
    renderWithProviders(<AccountsPage />)

    expect(screen.getAllByTestId('skeleton').length).toBeGreaterThan(0)
    expect(await screen.findByText('Everyday checking')).toBeInTheDocument()
    expect(screen.getByText('$2,500.00')).toBeInTheDocument()
    expect(screen.getByText('-$320.00')).toBeInTheDocument()
    expect(screen.getByText('Credit card · USD')).toBeInTheDocument()
  })

  it('marks archived accounts and hides their archive action', async () => {
    seedApi({
      accounts: [account({ name: 'Old savings', isArchived: true })],
    })
    renderWithProviders(<AccountsPage />)

    expect(await screen.findByText('archived')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /archive old savings/i }),
    ).not.toBeInTheDocument()
  })

  it('offers an empty state when there are no accounts', async () => {
    renderWithProviders(<AccountsPage />)
    expect(await screen.findByText('No accounts yet')).toBeInTheDocument()
  })

  it('creates an account through the modal', async () => {
    renderWithProviders(<AccountsPage />)
    await screen.findByText('No accounts yet')

    await userEvent.click(screen.getByRole('button', { name: 'Add account' }))
    await userEvent.type(screen.getByLabelText('Name'), 'Holiday fund')
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'savings')
    await userEvent.clear(screen.getByLabelText('Initial balance'))
    await userEvent.type(screen.getByLabelText('Initial balance'), '750')
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add account' })[1],
    )

    await waitFor(() =>
      expect(mocked.accounts.create).toHaveBeenCalledWith(
        {
          name: 'Holiday fund',
          type: 'savings',
          initialBalance: 750,
          currency: 'USD',
        },
        expect.anything(),
      ),
    )
    expect(await screen.findByText('Account added')).toBeInTheDocument()
  })

  it('validates that a name is required', async () => {
    renderWithProviders(<AccountsPage />)
    await screen.findByText('No accounts yet')

    await userEvent.click(screen.getByRole('button', { name: 'Add account' }))
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add account' })[1],
    )

    expect(await screen.findByText('Name is required')).toBeInTheDocument()
    expect(mocked.accounts.create).not.toHaveBeenCalled()
  })

  it('archives an account after confirmation', async () => {
    seedApi({ accounts: [account({ name: 'Everyday checking' })] })
    renderWithProviders(<AccountsPage />)
    await screen.findByText('Everyday checking')

    await userEvent.click(
      screen.getByRole('button', { name: /archive everyday checking/i }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Archive' }))

    await waitFor(() =>
      expect(mocked.accounts.archive).toHaveBeenCalledWith(
        'a1',
        expect.anything(),
      ),
    )
    expect(await screen.findByText('Account archived')).toBeInTheDocument()
  })

  it('reports a failed load', async () => {
    mocked.accounts.list.mockRejectedValue(new Error('Server is asleep'))
    renderWithProviders(<AccountsPage />)

    expect(await screen.findByText('Server is asleep')).toBeInTheDocument()
  })
})
