import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { budget, category } from '@/test/utils'
import { BudgetRow, budgetStatus } from './BudgetRow'

function renderRow(props: Partial<Parameters<typeof BudgetRow>[0]> = {}) {
  return render(
    <ul>
      <BudgetRow
        budget={budget()}
        category={category()}
        currency="USD"
        {...props}
      />
    </ul>,
  )
}

describe('budgetStatus', () => {
  it('reports on track well under the limit', () => {
    expect(budgetStatus(budget({ spent: 100, limit: 400 })).tone).toBe('success')
  })

  it('warns as spending approaches the limit', () => {
    const status = budgetStatus(budget({ spent: 350, limit: 400 }))
    expect(status.isNearLimit).toBe(true)
    expect(status.tone).toBe('warning')
  })

  it('flags spending above the limit', () => {
    const status = budgetStatus(budget({ spent: 500, limit: 400 }))
    expect(status.isOver).toBe(true)
    expect(status.label).toBe('over budget')
  })

  it('treats a zero limit as on track', () => {
    expect(budgetStatus(budget({ spent: 0, limit: 0 })).isOver).toBe(false)
  })
})

describe('BudgetRow', () => {
  it('shows the category, spend and remaining amount', () => {
    renderRow()
    expect(screen.getByText('Groceries')).toBeInTheDocument()
    expect(screen.getByText('$150.00')).toBeInTheDocument()
    expect(screen.getByText(/\$250\.00 left/)).toBeInTheDocument()
    expect(screen.getByText('on track')).toBeInTheDocument()
  })

  it('highlights an over-budget row in red', () => {
    renderRow({ budget: budget({ spent: 500, limit: 400, remaining: -100 }) })

    expect(screen.getByText('over budget')).toBeInTheDocument()
    expect(screen.getByText(/\$100\.00 over/)).toBeInTheDocument()
    expect(screen.getByTestId('progress-fill').className).toContain('bg-rose-500')
  })

  it('falls back when the category is unknown', () => {
    renderRow({ category: undefined })
    expect(screen.getByText('Unknown category')).toBeInTheDocument()
  })

  it('hides the actions when no handlers are supplied', () => {
    renderRow()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('calls the edit and delete handlers', async () => {
    const onEdit = vi.fn()
    const onDelete = vi.fn()
    renderRow({ onEdit, onDelete })

    await userEvent.click(
      screen.getByRole('button', { name: /edit budget for groceries/i }),
    )
    await userEvent.click(
      screen.getByRole('button', { name: /delete budget for groceries/i }),
    )

    expect(onEdit).toHaveBeenCalledOnce()
    expect(onDelete).toHaveBeenCalledOnce()
  })
})
