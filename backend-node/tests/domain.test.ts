/** Money, dates, enums, rendering and the validation rules. */

import { describe, expect, it } from 'vitest'
import { renderJson } from '../src/api/rendering.js'
import { FieldErrors } from '../src/core/errors.js'
import { addMonths, addYears, monthKey, startOfMonth, trailingMonths, tryParseMonth } from '../src/domain/dates.js'
import { enumOf, parseEnum } from '../src/domain/enums.js'
import { Instant, parseWireDate } from '../src/domain/instant.js'
import { Money, joinTags, splitTags, trimmedOrNull } from '../src/domain/money.js'
import {
  FIELD,
  emailAddress,
  maxLength,
  moneyRange,
  required,
  requiredMembers,
} from '../src/domain/validation.js'

const money = (text: string): Money => Money.parse(text)!

describe('Money', () => {
  it('keeps the scale it was written with', () => {
    expect(money('1250.00').toString()).toBe('1250.00')
    expect(money('12.5').toString()).toBe('12.5')
    expect(money('10').toString()).toBe('10')
    expect(money('0').toString()).toBe('0')
    expect(money('-0.50').toString()).toBe('-0.50')
  })

  it('adds with the wider of the two scales', () => {
    expect(money('1.5').add(money('2.25')).toString()).toBe('3.75')
    expect(money('10').add(money('0.01')).toString()).toBe('10.01')
  })

  it('subtracts, including past zero', () => {
    expect(money('100.00').subtract(money('250.75')).toString()).toBe('-150.75')
  })

  it.each([
    ['1.005', '1.01'],
    ['-1.005', '-1.01'],
    ['2.344', '2.34'],
    ['2.345', '2.35'],
    // Never lengthens the scale.
    ['10', '10'],
    ['0.1', '0.1'],
  ])('rounds %s to %s, half away from zero', (input, expected) => {
    expect(money(input).round().toString()).toBe(expected)
  })

  it('renders exactly two places for the CSV column', () => {
    expect(money('12.5').toFixed2()).toBe('12.50')
    expect(money('3000').toFixed2()).toBe('3000.00')
  })

  it('drops trailing zeros when trimmed', () => {
    expect(money('0.7500').trim().toString()).toBe('0.75')
    expect(money('0.5000').trim().toString()).toBe('0.5')
    expect(money('100').trim().toString()).toBe('100')
  })

  it('compares across scales', () => {
    expect(money('1.50').compare(money('1.5'))).toBe(0)
    expect(money('1.5').lessThan(money('1.51'))).toBe(true)
    expect(money('2').greaterThan(money('1.99'))).toBe(true)
  })

  it('reads a JSON number without losing its scale', () => {
    expect(Money.fromJson(12.5)!.toString()).toBe('12.5')
    expect(Money.fromJson('1250.00')!.toString()).toBe('1250.00')
  })

  it.each(['abc', '', '1e5', '1,000', '--1'])('rejects %s', (input) => {
    expect(Money.parse(input)).toBeNull()
  })
})

describe('rendering', () => {
  it('writes money as a bare number with its scale', () => {
    expect(renderJson({ balance: money('1250.00') })).toBe('{"balance":1250.00}')
  })

  it('writes a defined enum as its camelCase name', () => {
    expect(renderJson({ type: enumOf('AccountType', 2) })).toBe('{"type":"creditCard"}')
  })

  it('writes an undefined ordinal as a bare number', () => {
    // The .NET converter round-trips a value naming no member.
    expect(renderJson({ type: enumOf('AccountType', 99) })).toBe('{"type":99}')
  })

  it('escapes text so it cannot forge a number', () => {
    const rendered = renderJson({ text: '1,"balance":999' })
    expect(JSON.parse(rendered)).toEqual({ text: '1,"balance":999' })
  })

  it('omits undefined members but keeps nulls', () => {
    expect(renderJson({ a: null, b: undefined, c: 1 })).toBe('{"a":null,"c":1}')
  })
})

describe('Instant', () => {
  it('keeps microseconds the Date cannot hold', () => {
    expect(Instant.fromPg('2026-08-26 22:27:26.51655+00').toString()).toBe(
      '2026-08-26T22:27:26.51655Z',
    )
  })

  it('omits the fraction when it is zero', () => {
    expect(Instant.fromPg('2026-08-26 22:27:26+00').toString()).toBe('2026-08-26T22:27:26Z')
  })

  it('applies a non-UTC offset', () => {
    expect(Instant.fromPg('2026-08-26 20:00:00+02').toString()).toBe('2026-08-26T18:00:00Z')
  })

  it.each([
    '2026-08-26',
    '2026-08-26T10:00',
    '2026-08-26T10:00:00',
    '2026-08-26T10:00:00Z',
    '2026-08-26T10:00:00+02:00',
  ])('parses the wire layout %s', (input) => {
    expect(parseWireDate(input)).not.toBeNull()
  })

  it('reads a naive value as UTC', () => {
    expect(parseWireDate('2026-08-26T10:00:00')!.toString()).toBe('2026-08-26T10:00:00Z')
  })

  it('rejects nonsense', () => {
    expect(parseWireDate('last tuesday')).toBeNull()
  })
})

describe('calendar arithmetic', () => {
  it('clamps rather than rolling over', () => {
    const jan31 = new Date(Date.UTC(2026, 0, 31))
    expect(addMonths(jan31, 1).toISOString()).toBe('2026-02-28T00:00:00.000Z')
    // Clamping is not remembered: Feb 28 + 1 month is Mar 28, not Mar 31.
    expect(addMonths(addMonths(jan31, 1), 1).toISOString()).toBe('2026-03-28T00:00:00.000Z')
  })

  it('walks backwards across a year boundary', () => {
    expect(addMonths(new Date(Date.UTC(2026, 0, 15)), -1).toISOString()).toBe(
      '2025-12-15T00:00:00.000Z',
    )
  })

  it('clamps a leap day in a common year', () => {
    expect(addYears(new Date(Date.UTC(2028, 1, 29)), 1).toISOString()).toBe(
      '2029-02-28T00:00:00.000Z',
    )
  })

  it('finds the start of a month and its key', () => {
    const moment = new Date(Date.UTC(2026, 7, 26, 13, 5))
    expect(startOfMonth(moment).toISOString()).toBe('2026-08-01T00:00:00.000Z')
    expect(monthKey(moment)).toBe('2026-08')
  })

  it('builds a window ending on the reference month', () => {
    const window = trailingMonths(new Date(Date.UTC(2026, 2, 15)), 3)
    expect(window.map(monthKey)).toEqual(['2026-01', '2026-02', '2026-03'])
  })

  it.each(['2026-13', 'nope', '', '2026', '2026-1'])('rejects the month %s', (input) => {
    expect(tryParseMonth(input)).toBeNull()
  })
})

describe('enums', () => {
  it.each(['creditCard', 'CREDITCARD', 'creditcard', ' creditCard '])(
    'parses the name %s in any casing',
    (input) => {
      expect(parseEnum('AccountType', input)!.ordinal).toBe(2)
    },
  )

  it('parses an ordinal as a number or digits', () => {
    expect(parseEnum('TransactionType', 2)!.ordinal).toBe(2)
    expect(parseEnum('TransactionType', '2')!.ordinal).toBe(2)
  })

  it('preserves an undefined ordinal', () => {
    const parsed = parseEnum('AccountType', 99)!
    expect(parsed.ordinal).toBe(99)
    expect(parsed.isDefined).toBe(false)
    expect(parsed.wireName).toBe('99')
  })

  it('rejects an unknown name', () => {
    expect(parseEnum('AccountType', 'rust')).toBeNull()
  })
})

describe('tags', () => {
  it('round-trips', () => {
    expect(splitTags(joinTags(['food', 'out']))).toEqual(['food', 'out'])
  })

  it('trims and drops blanks', () => {
    expect(splitTags(joinTags([' food ', '  ', '', 'out']))).toEqual(['food', 'out'])
  })

  it('returns an array, never null', () => {
    expect(splitTags('')).toEqual([])
    expect(splitTags(null)).toEqual([])
  })

  it('collapses whitespace-only notes to null', () => {
    expect(trimmedOrNull('   ')).toBeNull()
    expect(trimmedOrNull(' hi ')).toBe('hi')
  })
})

describe('validation messages', () => {
  it('puts the field name first for required', () => {
    const errs = new FieldErrors()
    required(errs, FIELD.email, '   ')
    expect(errs.errors).toEqual({ Email: ['The Email field is required.'] })
  })

  it('puts "The field" first for a length rule', () => {
    const errs = new FieldErrors()
    maxLength(errs, FIELD.name, 'n'.repeat(201), 200)
    expect(errs.errors.Name).toEqual([
      "The field Name must be a string or array type with a maximum length of '200'.",
    ])
  })

  it('quotes the money bounds verbatim', () => {
    const errs = new FieldErrors()
    moneyRange(errs, FIELD.amount, money('0'), '0.01')
    expect(errs.errors.Amount).toEqual([
      'The field Amount must be between 0.01 and 999999999999.99.',
    ])
  })

  it.each([
    ['a@b.c', true],
    ['bad', false],
    ['@b.c', false],
    ['a@', false],
    ['a@b@c', false],
  ])('checks the email shape of %s', (value, valid) => {
    const errs = new FieldErrors()
    emailAddress(errs, FIELD.email, value)
    expect(errs.isEmpty).toBe(valid)
  })

  it('short-circuits on a missing required member', () => {
    const errs = new FieldErrors()
    expect(requiredMembers(errs, ['accountId', 'type'])).toBe(true)
    expect(errs.errors.$).toEqual([
      'The JSON payload was missing required properties, including the following: accountId, type',
    ])
  })
})
