import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import { NAV_ITEMS } from './nav'
import { AppLayout } from './AppLayout'
import { renderWithProviders } from '@/test/utils'

function renderLayout(auth = {}) {
  return renderWithProviders(
    <Routes>
      <Route element={<AppLayout />}>
        <Route path="/" element={<p>Page body</p>} />
      </Route>
    </Routes>,
    { auth },
  )
}

describe('AppLayout', () => {
  it('renders every navigation destination', () => {
    renderLayout()
    for (const item of NAV_ITEMS) {
      expect(screen.getByRole('link', { name: item.label })).toHaveAttribute(
        'href',
        item.to,
      )
    }
  })

  it('shows the signed-in user in the topbar', () => {
    renderLayout()
    expect(screen.getByText('Alex Morgan')).toBeInTheDocument()
    expect(screen.getByText('alex@example.com')).toBeInTheDocument()
    expect(screen.getByText('Page body')).toBeInTheDocument()
  })

  it('logs out from the topbar', async () => {
    const logout = vi.fn()
    renderLayout({ logout })

    await userEvent.click(screen.getByRole('button', { name: /log out/i }))
    expect(logout).toHaveBeenCalledOnce()
  })

  it('toggles the theme from the topbar', async () => {
    renderLayout()

    await userEvent.click(
      screen.getByRole('button', { name: /switch to dark mode/i }),
    )
    await waitFor(() => expect(document.documentElement).toHaveClass('dark'))

    await userEvent.click(
      screen.getByRole('button', { name: /switch to light mode/i }),
    )
    await waitFor(() =>
      expect(document.documentElement).not.toHaveClass('dark'),
    )
  })

  it('opens and closes the mobile navigation', async () => {
    renderLayout()

    await userEvent.click(
      screen.getByRole('button', { name: 'Open navigation' }),
    )
    const overlay = screen.getByRole('button', { name: 'Close navigation' })
    expect(overlay).toBeInTheDocument()

    await userEvent.click(overlay)
    expect(
      screen.queryByRole('button', { name: 'Close navigation' }),
    ).not.toBeInTheDocument()
  })
})
