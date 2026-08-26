import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import { category, renderWithProviders } from '@/test/utils'
import { SettingsPage } from './SettingsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

const categories = [
  category({ id: 'c1', name: 'Groceries', isDefault: true }),
  category({ id: 'c2', name: 'Side hustle', type: 'income', isDefault: false }),
]

describe('SettingsPage', () => {
  beforeEach(() => seedApi({ categories }))

  it('prefills the profile form from the signed-in user', async () => {
    renderWithProviders(<SettingsPage />)

    expect(screen.getByLabelText('Name')).toHaveValue('Alex Morgan')
    expect(screen.getByLabelText('Currency')).toHaveValue('USD')
  })

  it('saves the profile', async () => {
    const setUser = vi.fn()
    renderWithProviders(<SettingsPage />, { auth: { setUser } })

    await userEvent.clear(screen.getByLabelText('Name'))
    await userEvent.type(screen.getByLabelText('Name'), 'Alexandra Morgan')
    await userEvent.selectOptions(screen.getByLabelText('Currency'), 'EUR')
    await userEvent.click(screen.getByRole('button', { name: 'Save profile' }))

    await waitFor(() =>
      expect(mocked.auth.updateMe).toHaveBeenCalledWith({
        name: 'Alexandra Morgan',
        currency: 'EUR',
      }),
    )
    expect(await screen.findByText('Profile updated')).toBeInTheDocument()
    expect(setUser).toHaveBeenCalled()
  })

  it('rejects an empty profile name', async () => {
    renderWithProviders(<SettingsPage />)

    await userEvent.clear(screen.getByLabelText('Name'))
    await userEvent.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(await screen.findByText('Name is required')).toBeInTheDocument()
    expect(mocked.auth.updateMe).not.toHaveBeenCalled()
  })

  it('separates built-in categories from the user’s own', async () => {
    renderWithProviders(<SettingsPage />)

    expect(await screen.findByText('Built-in')).toBeInTheDocument()
    expect(screen.getByText('Your categories')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /edit groceries/i }),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /edit side hustle/i }),
    ).toBeInTheDocument()
  })

  it('creates a category with a colour and icon', async () => {
    renderWithProviders(<SettingsPage />)
    await screen.findByText('Built-in')

    await userEvent.click(screen.getByRole('button', { name: 'New category' }))
    const dialog = within(screen.getByRole('dialog'))
    await userEvent.type(dialog.getByLabelText('Name'), 'Coffee')
    await userEvent.click(dialog.getByRole('button', { name: 'Use colour #0ea5e9' }))
    await userEvent.click(dialog.getByRole('button', { name: 'Use icon goals' }))
    await userEvent.click(dialog.getByRole('button', { name: 'Add category' }))

    await waitFor(() =>
      expect(mocked.categories.create).toHaveBeenCalledWith(
        { name: 'Coffee', type: 'expense', icon: 'goals', color: '#0ea5e9' },
        expect.anything(),
      ),
    )
  })

  it('deletes a custom category after confirmation', async () => {
    renderWithProviders(<SettingsPage />)
    await screen.findByText('Side hustle')

    await userEvent.click(
      screen.getByRole('button', { name: /delete side hustle/i }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() =>
      expect(mocked.categories.remove).toHaveBeenCalledWith(
        'c2',
        expect.anything(),
      ),
    )
  })

  it('shows the active API backend read-only', () => {
    window.localStorage.setItem('ft.backend', 'go')
    renderWithProviders(<SettingsPage />)

    expect(screen.getByText('API backend')).toBeInTheDocument()
    expect(
      screen.getByText('Requests from this browser go to the Go API.'),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('tablist', { name: 'API backend' }),
    ).not.toBeInTheDocument()
  })

  it('toggles dark mode and persists the choice', async () => {
    renderWithProviders(<SettingsPage />)

    expect(screen.getByText('Light mode')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /switch to dark/i }))

    expect(await screen.findByText('Dark mode')).toBeInTheDocument()
    expect(document.documentElement).toHaveClass('dark')
    expect(window.localStorage.getItem('ft.theme')).toBe('dark')
  })
})
