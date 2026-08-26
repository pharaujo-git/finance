import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { ThemeProvider } from './context/ThemeProvider'
import { setToken } from './lib/token'
import { mocked, seedApi } from './test/apiMock'
import { createTestQueryClient, testUser } from './test/utils'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

function renderApp(route: string) {
  return render(
    <ThemeProvider>
      <QueryClientProvider client={createTestQueryClient()}>
        <MemoryRouter initialEntries={[route]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    </ThemeProvider>,
  )
}

describe('App', () => {
  beforeEach(() => seedApi())

  it('sends anonymous visitors to the login screen', async () => {
    renderApp('/')
    expect(await screen.findByText('Welcome back')).toBeInTheDocument()
  })

  it('serves the register route', async () => {
    renderApp('/register')
    expect(await screen.findByText('Create your account')).toBeInTheDocument()
  })

  it('renders the app shell for a signed-in visitor', async () => {
    setToken('valid')
    mocked.auth.me.mockResolvedValue(testUser)

    renderApp('/accounts')

    expect(
      await screen.findByRole('navigation', { name: 'Main' }),
    ).toBeInTheDocument()
    expect(
      await screen.findByRole('heading', { name: 'Accounts', level: 1 }),
    ).toBeInTheDocument()
    expect(screen.getByText('Alex Morgan')).toBeInTheDocument()
  })

  it('renders the not-found page for an unknown route', async () => {
    setToken('valid')
    mocked.auth.me.mockResolvedValue(testUser)

    renderApp('/nowhere')

    expect(await screen.findByText('Page not found')).toBeInTheDocument()
  })
})
