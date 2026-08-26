import { describe, expect, it } from 'vitest'
import {
  formatTags,
  moneyString,
  nonNegativeMoneyString,
  parseTags,
  toMoneyInput,
  toNumber,
} from './validation'

describe('parseTags', () => {
  it('splits, trims and de-duplicates', () => {
    expect(parseTags('food, travel , food')).toEqual(['food', 'travel'])
  })

  it('returns an empty list for blank input', () => {
    expect(parseTags('')).toEqual([])
    expect(parseTags('  ,  ')).toEqual([])
  })
})

describe('formatTags', () => {
  it('joins tags and tolerates nullish input', () => {
    expect(formatTags(['a', 'b'])).toBe('a, b')
    expect(formatTags(null)).toBe('')
  })
})

describe('toNumber / toMoneyInput', () => {
  it('parses numeric strings and defaults to zero', () => {
    expect(toNumber('12.5')).toBe(12.5)
    expect(toNumber('abc')).toBe(0)
  })

  it('renders money inputs', () => {
    expect(toMoneyInput(12.5)).toBe('12.5')
    expect(toMoneyInput(null)).toBe('')
  })
})

describe('money schemas', () => {
  it('rejects empty, non-numeric and non-positive amounts', () => {
    expect(moneyString.safeParse('').success).toBe(false)
    expect(moneyString.safeParse('abc').success).toBe(false)
    expect(moneyString.safeParse('0').success).toBe(false)
    expect(moneyString.safeParse('-5').success).toBe(false)
  })

  it('accepts positive amounts', () => {
    expect(moneyString.safeParse('12.34').success).toBe(true)
  })

  it('allows zero for non-negative amounts but not negatives', () => {
    expect(nonNegativeMoneyString.safeParse('0').success).toBe(true)
    expect(nonNegativeMoneyString.safeParse('-1').success).toBe(false)
  })
})
