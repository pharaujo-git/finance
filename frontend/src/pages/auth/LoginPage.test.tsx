import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test/utils'
import { LoginPage } from './LoginPage'
import { RegisterPage } from './RegisterPage'

describe('LoginPage', () => {
  it('requires an email and a password', async () => {
    const login = vi.fn()
    renderWithProviders(<LoginPage />, { auth: { login } })

    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByText('Email is required')).toBeInTheDocument()
    expect(screen.getByText('Password is required')).toBeInTheDocument()
    expect(login).not.toHaveBeenCalled()
  })

  it('rejects a malformed email', async () => {
    const login = vi.fn()
    renderWithProviders(<LoginPage />, { auth: { login } })

    await userEvent.type(screen.getByLabelText('Email'), 'not-an-email')
    await userEvent.type(screen.getByLabelText('Password'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByText('Enter a valid email')).toBeInTheDocument()
    expect(login).not.toHaveBeenCalled()
  })

  it('signs in with valid credentials', async () => {
    const login = vi.fn().mockResolvedValue(undefined)
    renderWithProviders(<LoginPage />, { auth: { login } })

    await userEvent.type(screen.getByLabelText('Email'), 'alex@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() =>
      expect(login).toHaveBeenCalledWith('alex@example.com', 'secret123'),
    )
  })

  it('shows the API error when sign-in fails', async () => {
    const login = vi.fn().mockRejectedValue(new Error('Invalid credentials'))
    renderWithProviders(<LoginPage />, { auth: { login } })

    await userEvent.type(screen.getByLabelText('Email'), 'alex@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Invalid credentials',
    )
  })

  it('links to the register screen', () => {
    renderWithProviders(<LoginPage />)
    expect(
      screen.getByRole('link', { name: 'Create an account' }),
    ).toHaveAttribute('href', '/register')
  })
})

describe('RegisterPage', () => {
  it('requires a name, email and a long enough password', async () => {
    const register = vi.fn()
    renderWithProviders(<RegisterPage />, { auth: { register } })

    await userEvent.type(screen.getByLabelText('Password'), 'short')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByText('Name is required')).toBeInTheDocument()
    expect(screen.getByText('Email is required')).toBeInTheDocument()
    expect(screen.getByText('Use at least 8 characters')).toBeInTheDocument()
    expect(register).not.toHaveBeenCalled()
  })

  it('registers with the documented field order', async () => {
    const register = vi.fn().mockResolvedValue(undefined)
    renderWithProviders(<RegisterPage />, { auth: { register } })

    await userEvent.type(screen.getByLabelText('Name'), 'Alex Morgan')
    await userEvent.type(screen.getByLabelText('Email'), 'alex@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    await waitFor(() =>
      expect(register).toHaveBeenCalledWith(
        'alex@example.com',
        'secret123',
        'Alex Morgan',
      ),
    )
  })

  it('reports a failed registration', async () => {
    const register = vi.fn().mockRejectedValue(new Error('Email already used'))
    renderWithProviders(<RegisterPage />, { auth: { register } })

    await userEvent.type(screen.getByLabelText('Name'), 'Alex')
    await userEvent.type(screen.getByLabelText('Email'), 'alex@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Email already used',
    )
  })
})
