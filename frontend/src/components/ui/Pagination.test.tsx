import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Pagination, totalPages } from './Pagination'

describe('totalPages', () => {
  it('rounds up and never drops below one', () => {
    expect(totalPages(45, 20)).toBe(3)
    expect(totalPages(40, 20)).toBe(2)
    expect(totalPages(0, 20)).toBe(1)
    expect(totalPages(10, 0)).toBe(1)
  })
})

describe('Pagination', () => {
  it('describes the visible slice', () => {
    render(
      <Pagination page={2} pageSize={20} total={45} onPageChange={vi.fn()} />,
    )
    expect(screen.getByText(/Page 2 of 3/)).toBeInTheDocument()
    expect(screen.getByText('21')).toBeInTheDocument()
    expect(screen.getByText('40')).toBeInTheDocument()
  })

  it('disables previous on the first page and next on the last', () => {
    const { rerender } = render(
      <Pagination page={1} pageSize={20} total={45} onPageChange={vi.fn()} />,
    )
    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled()

    rerender(
      <Pagination page={3} pageSize={20} total={45} onPageChange={vi.fn()} />,
    )
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled()
  })

  it('emits the new page number', async () => {
    const onPageChange = vi.fn()
    render(
      <Pagination page={2} pageSize={20} total={45} onPageChange={onPageChange} />,
    )

    await userEvent.click(screen.getByRole('button', { name: /next/i }))
    await userEvent.click(screen.getByRole('button', { name: /previous/i }))

    expect(onPageChange).toHaveBeenNthCalledWith(1, 3)
    expect(onPageChange).toHaveBeenNthCalledWith(2, 1)
  })

  it('handles an empty result set', () => {
    render(
      <Pagination page={1} pageSize={20} total={0} onPageChange={vi.fn()} />,
    )
    expect(screen.getAllByText('0')).toHaveLength(3)
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled()
  })
})
