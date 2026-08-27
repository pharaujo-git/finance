/** Balance, recurrence and CSV logic -- all pure, so no database is needed. */

import { describe, expect, it } from 'vitest'
import { Frequency, TransactionType, enumOf } from '../src/domain/enums.js'
import { Instant } from '../src/domain/instant.js'
import { Money } from '../src/domain/money.js'
import type { Account, RecurringRule, TransactionSlice } from '../src/repositories/index.js'
import { balanceOf, deltaFor, netWorthDelta } from '../src/services/balance.js'
import { compareGuid } from '../src/services/index.js'
import {
  escapeCsvField,
  parseCsv,
  parseCsvDate,
  parseCurrencyAmount,
} from '../src/services/csv.js'
import { MAX_OCCURRENCES_PER_PASS, advance, materialize } from '../src/services/recurring.js'

const CHECKING = '00000000-0000-0000-0000-000000000001'
const SAVINGS = '00000000-0000-0000-0000-000000000002'
const money = (text: string): Money => Money.parse(text)!
const at = (iso: string): Instant => Instant.fromDate(new Date(iso))

function slice(
  type: number,
  amount: string,
  options: { account?: string; transfer?: string | null } = {},
): TransactionSlice {
  return {
    accountId: options.account ?? CHECKING,
    transferAccountId: options.transfer ?? null,
    categoryId: null,
    type: enumOf('TransactionType', type),
    amount: money(amount),
    date: at('2026-08-01T00:00:00Z'),
  }
}

describe('balance', () => {
  it('credits the account for income', () => {
    expect(deltaFor(CHECKING, slice(TransactionType.Income, '100')).toString()).toBe('100')
  })

  it('debits the account for an expense', () => {
    expect(deltaFor(CHECKING, slice(TransactionType.Expense, '40')).toString()).toBe('-40')
  })

  it('debits the source of a transfer', () => {
    const item = slice(TransactionType.Transfer, '25', { transfer: SAVINGS })
    expect(deltaFor(CHECKING, item).toString()).toBe('-25')
  })

  it('credits the destination of a transfer', () => {
    const item = slice(TransactionType.Transfer, '25', { transfer: SAVINGS })
    expect(deltaFor(SAVINGS, item).toString()).toBe('25')
  })

  it('leaves an unrelated account untouched', () => {
    expect(deltaFor(SAVINGS, slice(TransactionType.Income, '100')).toString()).toBe('0')
  })

  it('treats a transfer as net-worth neutral', () => {
    const item = slice(TransactionType.Transfer, '25', { transfer: SAVINGS })
    expect(netWorthDelta(item).toString()).toBe('0')
  })

  it('follows income and expense for net worth', () => {
    expect(netWorthDelta(slice(TransactionType.Income, '100')).toString()).toBe('100')
    expect(netWorthDelta(slice(TransactionType.Expense, '40')).toString()).toBe('-40')
  })

  it('starts from the opening amount', () => {
    const account: Account = {
      id: CHECKING,
      userId: 'u',
      name: 'Checking',
      type: enumOf('AccountType', 0),
      initialBalance: money('1000.00'),
      currency: 'USD',
      isArchived: false,
      createdAt: at('2026-01-01T00:00:00Z'),
    }
    const slices = [
      slice(TransactionType.Income, '3000'),
      slice(TransactionType.Expense, '42.50'),
      slice(TransactionType.Transfer, '250.75', { transfer: SAVINGS }),
    ]
    expect(balanceOf(account, slices).toString()).toBe('3706.75')
  })
})

describe('compareGuid', () => {
  it('compares the first group as signed, not as bytes', () => {
    // 0x80... sorts before 0x7f... under .NET's Guid.CompareTo.
    const high = '80000000-0000-0000-0000-000000000000'
    const low = '7f000000-0000-0000-0000-000000000000'
    expect(compareGuid(high, low)).toBeLessThan(0)
  })

  it('falls through to the trailing bytes', () => {
    const a = '00000000-0000-0000-0000-000000000001'
    const b = '00000000-0000-0000-0000-000000000002'
    expect(compareGuid(a, b)).toBeLessThan(0)
    expect(compareGuid(a, a)).toBe(0)
  })
})

function rule(overrides: Partial<RecurringRule> = {}): RecurringRule {
  return {
    id: 'r',
    userId: 'u',
    accountId: CHECKING,
    categoryId: null,
    type: enumOf('TransactionType', TransactionType.Expense),
    amount: money('9.99'),
    description: 'Streaming',
    frequency: enumOf('Frequency', Frequency.Monthly),
    startDate: at('2026-01-01T00:00:00Z'),
    endDate: null,
    nextRunDate: at('2026-01-01T00:00:00Z'),
    isActive: true,
    ...overrides,
  }
}

describe('advance', () => {
  it.each([
    [Frequency.Daily, '2026-01-02T00:00:00Z'],
    [Frequency.Weekly, '2026-01-08T00:00:00Z'],
    [Frequency.Monthly, '2026-02-01T00:00:00Z'],
    [Frequency.Yearly, '2027-01-01T00:00:00Z'],
  ])('steps by frequency %i', (frequency, expected) => {
    const next = advance(new Date('2026-01-01T00:00:00Z'), enumOf('Frequency', frequency))
    expect(next.toISOString()).toBe(new Date(expected).toISOString())
  })

  it('clamps the day when stepping a month', () => {
    const next = advance(
      new Date('2026-01-31T00:00:00Z'),
      enumOf('Frequency', Frequency.Monthly),
    )
    expect(next.toISOString()).toBe('2026-02-28T00:00:00.000Z')
  })
})

describe('materialize', () => {
  it('creates one per period up to the cutoff', () => {
    const target = rule()
    const created = materialize(target, at('2026-03-15T00:00:00Z'))
    expect(created.map((item) => item.date.toString().slice(0, 10))).toEqual([
      '2026-01-01',
      '2026-02-01',
      '2026-03-01',
    ])
    expect(target.nextRunDate.toString()).toBe('2026-04-01T00:00:00Z')
  })

  it('creates nothing before the first run', () => {
    expect(materialize(rule({ nextRunDate: at('2026-06-01T00:00:00Z') }), at('2026-01-01T00:00:00Z')))
      .toEqual([])
  })

  it('tags what it creates', () => {
    const created = materialize(rule(), at('2026-01-02T00:00:00Z'))
    expect(created[0]!.tags).toEqual(['recurring'])
    expect(created[0]!.notes).toBeNull()
    expect(created[0]!.transferAccountId).toBeNull()
  })

  it('still runs an occurrence exactly on the end date', () => {
    const target = rule({ endDate: at('2026-02-01T00:00:00Z') })
    expect(materialize(target, at('2026-06-01T00:00:00Z'))).toHaveLength(2)
    expect(target.isActive).toBe(false)
  })

  it('retires a rule past its end', () => {
    const target = rule({ endDate: at('2025-12-01T00:00:00Z') })
    expect(materialize(target, at('2026-06-01T00:00:00Z'))).toEqual([])
    expect(target.isActive).toBe(false)
  })

  it('caps a long-dormant rule and leaves it running', () => {
    const target = rule({ frequency: enumOf('Frequency', Frequency.Daily) })
    expect(materialize(target, at('2030-01-01T00:00:00Z'))).toHaveLength(MAX_OCCURRENCES_PER_PASS)
    expect(target.isActive).toBe(true)
  })

  it('produces nothing for an inactive rule', () => {
    expect(materialize(rule({ isActive: false }), at('2030-01-01T00:00:00Z'))).toEqual([])
  })
})

describe('csv reader', () => {
  it('reads plain rows', () => {
    expect(parseCsv('a,b\n1,2\n')).toEqual([
      ['a', 'b'],
      ['1', '2'],
    ])
  })

  it('adds no empty row for a trailing newline', () => {
    expect(parseCsv('a,b\n1,2\n\n')).toHaveLength(2)
  })

  it('handles quoted commas and doubled quotes', () => {
    expect(parseCsv('"a,b","say ""hi"""\n')).toEqual([['a,b', 'say "hi"']])
  })

  it('lets a quoted field span lines', () => {
    expect(parseCsv('"line1\nline2",x\n')).toEqual([['line1\nline2', 'x']])
  })

  it('ignores carriage returns outside quotes', () => {
    expect(parseCsv('a,b\r\n1,2\r\n')).toEqual([
      ['a', 'b'],
      ['1', '2'],
    ])
  })

  it('reads empty input as no rows', () => {
    expect(parseCsv('')).toEqual([])
  })
})

describe('csv writer', () => {
  it.each([
    ['plain', 'plain'],
    ['has,comma', '"has,comma"'],
    ['has"quote', '"has""quote"'],
    ['has\nnewline', '"has\nnewline"'],
  ])('quotes %s only when needed', (value, expected) => {
    expect(escapeCsvField(value)).toBe(expected)
  })
})

describe('csv values', () => {
  it.each([
    ['12.50', '12.50'],
    ['$12.50', '12.50'],
    ['1,234.56', '1234.56'],
    ['(12.50)', '-12.50'],
    ['12.50-', '-12.50'],
    ['€9.99', '9.99'],
  ])('reads the currency shape %s', (value, expected) => {
    expect(parseCurrencyAmount(value)!.toString()).toBe(expected)
  })

  it('rejects nonsense', () => {
    expect(parseCurrencyAmount('abc')).toBeNull()
  })

  it.each([
    '2026-08-26',
    '2026-08-26T10:00:00',
    '2026-08-26 10:00:00',
    '08/26/2026',
    '8/26/2026',
    '8/26/2026 3:04:05 PM',
  ])('reads the import date layout %s', (value) => {
    expect(parseCsvDate(value)).not.toBeNull()
  })

  it('reads a PM marker as afternoon', () => {
    expect(parseCsvDate('8/26/2026 3:04:05 PM')!.toString()).toBe('2026-08-26T15:04:05Z')
  })

  it('rejects a day-first date', () => {
    expect(parseCsvDate('26/08/2026')).toBeNull()
  })
})
