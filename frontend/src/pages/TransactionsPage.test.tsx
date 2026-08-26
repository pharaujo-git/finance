import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import {
  account,
  category,
  recurring,
  renderWithProviders,
  transaction,
} from '@/test/utils'
import { TransactionsPage } from './TransactionsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

const baseSeed = {
  accounts: [account({ id: 'a1', name: 'Checking' })],
  categories: [category({ id: 'c1', name: 'Groceries' })],
  transactions: {
    items: [transaction({ description: 'Supermarket run' })],
    total: 45,
    page: 1,
    pageSize: 20,
  },
  recurring: [recurring()],
}

describe('TransactionsPage', () => {
  beforeEach(() => seedApi(baseSeed))

  it('renders the table with formatted rows', async () => {
    renderWithProviders(<TransactionsPage />)

    expect(await screen.findByText('Supermarket run')).toBeInTheDocument()
    expect(screen.getByText('-$42.50')).toBeInTheDocument()
    expect(screen.getAllByText('Groceries').length).toBeGreaterThan(0)
    expect(screen.getByText('weekly')).toBeInTheDocument()
  })

  it('paginates and asks the API for the next page', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    expect(screen.getByText(/Page 1 of 3/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() =>
      expect(mocked.transactions.list).toHaveBeenCalledWith(
        expect.objectContaining({ page: 2, pageSize: 20 }),
        expect.anything(),
      ),
    )
  })

  it('passes filters through to the API', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    await userEvent.selectOptions(screen.getByLabelText('Type'), 'income')

    await waitFor(() =>
      expect(mocked.transactions.list).toHaveBeenCalledWith(
        expect.objectContaining({ type: 'income', page: 1 }),
        expect.anything(),
      ),
    )
  })

  it('enables the clear button only when a filter is set', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    const clear = screen.getByRole('button', { name: /clear filters/i })
    expect(clear).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Search'), 'coffee')
    await waitFor(() => expect(clear).toBeEnabled())

    await userEvent.click(clear)
    expect(screen.getByLabelText('Search')).toHaveValue('')
  })

  it('deletes a transaction after confirmation', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    await userEvent.click(
      screen.getByRole('button', { name: /delete supermarket run/i }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() =>
      expect(mocked.transactions.remove).toHaveBeenCalledWith(
        't1',
        expect.anything(),
      ),
    )
    expect(await screen.findByText('Transaction deleted')).toBeInTheDocument()
  })

  it('exports a CSV using the current date filters', async () => {
    const createObjectURL = vi.fn(() => 'blob:csv')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL })

    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    await userEvent.click(screen.getByRole('button', { name: /export csv/i }))

    await waitFor(() => expect(mocked.transactions.exportCsv).toHaveBeenCalled())
    expect(createObjectURL).toHaveBeenCalled()
    expect(await screen.findByText('Export ready')).toBeInTheDocument()
  })

  it('imports a CSV and reports the counts', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    const file = new File(['date,amount'], 'tx.csv', { type: 'text/csv' })
    await userEvent.upload(
      screen.getByLabelText('Import transactions CSV'),
      file,
    )

    await waitFor(() =>
      expect(mocked.transactions.importCsv).toHaveBeenCalledWith(
        file,
        expect.anything(),
      ),
    )
    expect(await screen.findByText('Imported 3 · skipped 1')).toBeInTheDocument()
  })

  it('shows an empty state when nothing matches', async () => {
    seedApi({ ...baseSeed, transactions: { items: [], total: 0, page: 1, pageSize: 20 } })
    renderWithProviders(<TransactionsPage />)

    expect(await screen.findByText('No transactions found')).toBeInTheDocument()
  })

  it('switches to the recurring tab and lists rules', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')

    await userEvent.click(screen.getByRole('tab', { name: 'Recurring' }))

    expect(await screen.findByText('Rent')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
    expect(screen.getByText('$1,200.00')).toBeInTheDocument()
  })

  it('pauses a recurring rule', async () => {
    renderWithProviders(<TransactionsPage />)
    await screen.findByText('Supermarket run')
    await userEvent.click(screen.getByRole('tab', { name: 'Recurring' }))
    await screen.findByText('Rent')

    await userEvent.click(screen.getByRole('button', { name: 'Pause' }))

    await waitFor(() =>
      expect(mocked.recurring.update).toHaveBeenCalledWith(
        'r1',
        expect.objectContaining({ isActive: false }),
      ),
    )
    expect(await screen.findByText('Rule paused')).toBeInTheDocument()
  })
})
