import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, setUnauthorizedHandler } from '@/lib/api'
import { getToken, setToken } from '@/lib/token'
import { mocked, seedApi } from '@/test/apiMock'
import { testUser } from '@/test/utils'
import { useAuth } from './auth-context'
import { AuthProvider } from './AuthProvider'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

function Harness() {
  const { user, isLoading, isAuthenticated, login, register, logout, currency } =
    useAuth()

  if (isLoading) return <p>loading</p>
  return (
    <div>
      <p>{isAuthenticated ? `signed in as ${user?.name}` : 'signed out'}</p>
      <p>currency: {currency}</p>
      <button type="button" onClick={() => void login('a@b.c', 'pw')}>
        login
      </button>
      <button type="button" onClick={() => void register('a@b.c', 'pw', 'Alex')}>
        register
      </button>
      <button type="button" onClick={logout}>
        logout
      </button>
    </div>
  )
}

function renderAuth() {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter initialEntries={['/']}>
        <AuthProvider>
          <Routes>
            <Route path="/" element={<Harness />} />
            <Route path="/login" element={<p>login screen</p>} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AuthProvider', () => {
  beforeEach(() => {
    seedApi()
    setUnauthorizedHandler(null)
  })

  it('starts signed out when there is no stored token', async () => {
    renderAuth()
    expect(await screen.findByText('signed out')).toBeInTheDocument()
    expect(mocked.auth.me).not.toHaveBeenCalled()
  })

  it('restores the session from a stored token', async () => {
    setToken('stored-token')
    mocked.auth.me.mockResolvedValue(testUser)

    renderAuth()

    expect(await screen.findByText('signed in as Alex Morgan')).toBeInTheDocument()
    expect(screen.getByText('currency: USD')).toBeInTheDocument()
  })

  it('falls back to signed out when the stored token is rejected', async () => {
    setToken('stale-token')
    mocked.auth.me.mockRejectedValue(new ApiError(401, 'expired'))

    renderAuth()
    expect(await screen.findByText('signed out')).toBeInTheDocument()
  })

  it('stores the token returned by login', async () => {
    mocked.auth.login.mockResolvedValue({ token: 'fresh', user: testUser })
    renderAuth()
    await screen.findByText('signed out')

    await userEvent.click(screen.getByRole('button', { name: 'login' }))

    expect(await screen.findByText('signed in as Alex Morgan')).toBeInTheDocument()
    expect(getToken()).toBe('fresh')
    expect(mocked.auth.login).toHaveBeenCalledWith({
      email: 'a@b.c',
      password: 'pw',
    })
  })

  it('stores the token returned by register', async () => {
    mocked.auth.register.mockResolvedValue({ token: 'new', user: testUser })
    renderAuth()
    await screen.findByText('signed out')

    await userEvent.click(screen.getByRole('button', { name: 'register' }))

    expect(await screen.findByText('signed in as Alex Morgan')).toBeInTheDocument()
    expect(mocked.auth.register).toHaveBeenCalledWith({
      email: 'a@b.c',
      password: 'pw',
      name: 'Alex',
    })
  })

  it('clears the token and navigates to /login on logout', async () => {
    mocked.auth.login.mockResolvedValue({ token: 'fresh', user: testUser })
    renderAuth()
    await screen.findByText('signed out')
    await userEvent.click(screen.getByRole('button', { name: 'login' }))
    await screen.findByText('signed in as Alex Morgan')

    await userEvent.click(screen.getByRole('button', { name: 'logout' }))

    expect(await screen.findByText('login screen')).toBeInTheDocument()
    expect(getToken()).toBeNull()
  })

  it('registers a 401 handler that signs the user out', async () => {
    setToken('stored-token')
    mocked.auth.me.mockResolvedValue(testUser)
    renderAuth()
    await screen.findByText('signed in as Alex Morgan')

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('', { status: 401 })),
    )
    const { api } = await import('@/lib/api')
    await expect(api.get('/accounts')).rejects.toBeInstanceOf(ApiError)

    await waitFor(() =>
      expect(screen.getByText('login screen')).toBeInTheDocument(),
    )
  })
})
