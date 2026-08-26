import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { AuthContext } from '@/context/auth-context'
import { makeAuth } from '@/test/utils'
import { ProtectedRoute, PublicOnlyRoute } from './ProtectedRoute'

function renderAt(
  route: string,
  auth: Parameters<typeof makeAuth>[0],
) {
  return render(
    <AuthContext value={makeAuth(auth)}>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route path="/" element={<p>Dashboard</p>} />
          </Route>
          <Route element={<PublicOnlyRoute />}>
            <Route path="/login" element={<p>Login screen</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </AuthContext>,
  )
}

describe('ProtectedRoute', () => {
  it('renders the route when the user is signed in', () => {
    renderAt('/', { isAuthenticated: true })
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('redirects to /login when the user is signed out', () => {
    renderAt('/', { isAuthenticated: false, user: null })
    expect(screen.getByText('Login screen')).toBeInTheDocument()
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
  })

  it('waits while the session is still being checked', () => {
    renderAt('/', { isAuthenticated: false, user: null, isLoading: true })
    expect(
      screen.getByRole('status', { name: /checking your session/i }),
    ).toBeInTheDocument()
    expect(screen.queryByText('Login screen')).not.toBeInTheDocument()
  })
})

describe('PublicOnlyRoute', () => {
  it('sends signed-in users to the dashboard', () => {
    renderAt('/login', { isAuthenticated: true })
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('shows the auth screen to signed-out users', () => {
    renderAt('/login', { isAuthenticated: false, user: null })
    expect(screen.getByText('Login screen')).toBeInTheDocument()
  })

  it('waits while the session is still being checked', () => {
    renderAt('/login', { isAuthenticated: false, user: null, isLoading: true })
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
