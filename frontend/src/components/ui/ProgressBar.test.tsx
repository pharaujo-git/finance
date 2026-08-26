import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProgressBar, ProgressRing } from './ProgressBar'

describe('ProgressBar', () => {
  it('sizes the fill to the used percentage', () => {
    render(<ProgressBar value={50} max={200} label="Groceries" />)

    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '25',
    )
    expect(screen.getByTestId('progress-fill')).toHaveStyle({ width: '25%' })
  })

  it('caps the fill at 100% when over the limit', () => {
    render(<ProgressBar value={500} max={200} />)
    expect(screen.getByTestId('progress-fill')).toHaveStyle({ width: '100%' })
  })

  it('turns red only when overflow warnings are enabled', () => {
    const { rerender } = render(<ProgressBar value={500} max={200} />)
    expect(screen.getByTestId('progress-fill').className).not.toContain(
      'bg-rose-500',
    )

    rerender(<ProgressBar value={500} max={200} warnOnOverflow />)
    expect(screen.getByTestId('progress-fill').className).toContain('bg-rose-500')
  })

  it('uses the category colour when it is within budget', () => {
    render(<ProgressBar value={10} max={100} warnOnOverflow color="#ff0000" />)
    expect(screen.getByTestId('progress-fill')).toHaveStyle({
      backgroundColor: '#ff0000',
    })
  })
})

describe('ProgressRing', () => {
  it('labels the completed percentage', () => {
    render(<ProgressRing value={25} max={100} />)
    expect(screen.getByRole('img', { name: '25% complete' })).toBeInTheDocument()
    expect(screen.getByText('25%')).toBeInTheDocument()
  })

  it('accepts a custom label', () => {
    render(<ProgressRing value={1} max={2} label="Emergency fund progress" />)
    expect(
      screen.getByRole('img', { name: 'Emergency fund progress' }),
    ).toBeInTheDocument()
  })
})
