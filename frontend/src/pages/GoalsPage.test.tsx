import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mocked, seedApi } from '@/test/apiMock'
import { goal, renderWithProviders } from '@/test/utils'
import { GoalsPage } from './GoalsPage'

vi.mock('@/api/endpoints', async () => (await import('@/test/apiMock')).mocked)

describe('GoalsPage', () => {
  beforeEach(() => seedApi())

  it('shows an empty state with a call to action', async () => {
    renderWithProviders(<GoalsPage />)
    expect(await screen.findByText('No goals yet')).toBeInTheDocument()
  })

  it('renders goal progress', async () => {
    seedApi({ goals: [goal()] })
    renderWithProviders(<GoalsPage />)

    expect(await screen.findByText('Emergency fund')).toBeInTheDocument()
    expect(screen.getByText('$1,250.00')).toBeInTheDocument()
    expect(screen.getByText(/of \$5,000\.00/)).toBeInTheDocument()
    expect(
      screen.getByRole('img', { name: 'Emergency fund progress' }),
    ).toBeInTheDocument()
  })

  it('flags a goal that has been reached', async () => {
    seedApi({ goals: [goal({ currentAmount: 5000 })] })
    renderWithProviders(<GoalsPage />)

    expect(await screen.findByText('Goal reached')).toBeInTheDocument()
  })

  it('creates a goal', async () => {
    renderWithProviders(<GoalsPage />)
    await screen.findByText('No goals yet')

    await userEvent.click(screen.getByRole('button', { name: 'Add goal' }))
    await userEvent.type(screen.getByLabelText('Name'), 'New laptop')
    await userEvent.type(screen.getByLabelText('Target amount'), '1800')
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add goal' })[1],
    )

    await waitFor(() =>
      expect(mocked.goals.create).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'New laptop', targetAmount: 1800 }),
        expect.anything(),
      ),
    )
  })

  it('validates the goal form', async () => {
    renderWithProviders(<GoalsPage />)
    await screen.findByText('No goals yet')

    await userEvent.click(screen.getByRole('button', { name: 'Add goal' }))
    await userEvent.click(
      screen.getAllByRole('button', { name: 'Add goal' })[1],
    )

    expect(await screen.findByText('Name is required')).toBeInTheDocument()
    expect(mocked.goals.create).not.toHaveBeenCalled()
  })

  it('adds a contribution', async () => {
    seedApi({ goals: [goal()] })
    renderWithProviders(<GoalsPage />)
    await screen.findByText('Emergency fund')

    await userEvent.click(
      screen.getByRole('button', { name: /add contribution/i }),
    )
    const dialog = within(screen.getByRole('dialog'))
    await userEvent.type(dialog.getByLabelText('Amount'), '250')
    await userEvent.click(
      dialog.getByRole('button', { name: 'Add contribution' }),
    )

    await waitFor(() =>
      expect(mocked.goals.contribute).toHaveBeenCalledWith('g1', 250),
    )
    expect(await screen.findByText('Contribution added')).toBeInTheDocument()
  })

  it('deletes a goal after confirmation', async () => {
    seedApi({ goals: [goal()] })
    renderWithProviders(<GoalsPage />)
    await screen.findByText('Emergency fund')

    await userEvent.click(
      screen.getByRole('button', { name: /delete emergency fund/i }),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() =>
      expect(mocked.goals.remove).toHaveBeenCalledWith('g1', expect.anything()),
    )
  })
})
