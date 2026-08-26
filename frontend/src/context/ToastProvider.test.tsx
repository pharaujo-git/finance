import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useToast } from './toast-context'
import { ToastProvider, TOAST_TIMEOUT_MS } from './ToastProvider'

function Harness() {
  const { push } = useToast()
  return (
    <>
      <button type="button" onClick={() => push('Saved')}>
        success
      </button>
      <button type="button" onClick={() => push('Nope', 'error')}>
        error
      </button>
    </>
  )
}

function ThrowsOutsideProvider() {
  useToast()
  return null
}

describe('ToastProvider', () => {
  it('renders nothing until a toast is pushed', () => {
    render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows a pushed toast and dismisses it on click', async () => {
    render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )

    await userEvent.click(screen.getByRole('button', { name: 'success' }))
    expect(screen.getByText('Saved')).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Dismiss notification' }),
    )
    expect(screen.queryByText('Saved')).not.toBeInTheDocument()
  })

  it('stacks toasts of different variants', async () => {
    render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )

    await userEvent.click(screen.getByRole('button', { name: 'success' }))
    await userEvent.click(screen.getByRole('button', { name: 'error' }))

    expect(screen.getByText('Saved')).toBeInTheDocument()
    expect(screen.getByText('Nope')).toBeInTheDocument()
  })

  it('auto-dismisses after the timeout', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      render(
        <ToastProvider>
          <Harness />
        </ToastProvider>,
      )
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      await user.click(screen.getByRole('button', { name: 'success' }))
      expect(screen.getByText('Saved')).toBeInTheDocument()

      act(() => {
        vi.advanceTimersByTime(TOAST_TIMEOUT_MS + 10)
      })
      expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('throws when used outside the provider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<ThrowsOutsideProvider />)).toThrow(
      /useToast must be used inside ToastProvider/,
    )
    spy.mockRestore()
  })
})
