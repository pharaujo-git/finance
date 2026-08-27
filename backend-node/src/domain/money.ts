/**
 * Money, tags and the wire formats for dates.
 *
 * JavaScript has no decimal type, and the usual libraries normalise trailing
 * zeros away -- `new Decimal('1250.00').toString()` is `'1250'`. This API has
 * to put `1250.00` on the wire, because the .NET, Go and Python backends do.
 * So money is a pair here: an integer count of the smallest unit, plus the
 * scale it is carrying. Arithmetic follows the same rule the other backends'
 * decimal libraries use -- an addition keeps the wider of the two scales.
 */

/** The bounds every [Range(typeof(decimal), ...)] in the DTOs uses. */
export const MONEY_MIN_POSITIVE = '0.01'
export const MONEY_MIN_ZERO = '0.00'
export const MONEY_MAX = '999999999999.99'

export const MONEY_SCALE = 2

/** The separator the TagsRaw column uses; it cannot occur inside a tag. */
const TAG_SEPARATOR = '\u001F'

const TEN = 10n

function pow10(exponent: number): bigint {
  return TEN ** BigInt(exponent)
}

export class Money {
  /** The value as an integer count of 10^-scale units. */
  readonly units: bigint
  /** How many decimal places this value is carrying. Never negative. */
  readonly scale: number

  private constructor(units: bigint, scale: number) {
    this.units = units
    this.scale = scale
  }

  static of(units: bigint, scale: number): Money {
    return new Money(units, scale)
  }

  static zero(): Money {
    return new Money(0n, 0)
  }

  /** Reads a decimal literal, keeping the scale it was written with. */
  static parse(text: string): Money | null {
    const trimmed = text.trim()
    // Deliberately strict: no exponents, no separators, no currency symbols.
    if (!/^[+-]?\d+(\.\d+)?$/.test(trimmed)) return null

    const negative = trimmed.startsWith('-')
    const digits = trimmed.replace(/^[+-]/, '')
    const [whole = '', fraction = ''] = digits.split('.')

    const units = BigInt(whole + fraction)
    return new Money(negative ? -units : units, fraction.length)
  }

  /** Reads whatever JSON produced: a number, or a quoted number. */
  static fromJson(value: unknown): Money | null {
    if (typeof value === 'number') {
      // Number->string is exact for every value the API accepts, and keeps the
      // scale the caller actually wrote (12.5 stays one place).
      return Number.isFinite(value) ? Money.parse(String(value)) : null
    }
    if (typeof value === 'string') return Money.parse(value)
    return null
  }

  private align(other: Money): [bigint, bigint, number] {
    const scale = Math.max(this.scale, other.scale)
    return [
      this.units * pow10(scale - this.scale),
      other.units * pow10(scale - other.scale),
      scale,
    ]
  }

  add(other: Money): Money {
    const [left, right, scale] = this.align(other)
    return new Money(left + right, scale)
  }

  subtract(other: Money): Money {
    const [left, right, scale] = this.align(other)
    return new Money(left - right, scale)
  }

  negate(): Money {
    return new Money(-this.units, this.scale)
  }

  compare(other: Money): number {
    const [left, right] = this.align(other)
    if (left < right) return -1
    return left > right ? 1 : 0
  }

  lessThan(other: Money): boolean {
    return this.compare(other) < 0
  }

  greaterThan(other: Money): boolean {
    return this.compare(other) > 0
  }

  isZero(): boolean {
    return this.units === 0n
  }

  /**
   * Rounds to two places, half away from zero -- and never lengthens the
   * scale, so 10 stays 10 rather than becoming 10.00.
   */
  round(): Money {
    if (this.scale <= MONEY_SCALE) return this
    const divisor = pow10(this.scale - MONEY_SCALE)
    const negative = this.units < 0n
    const magnitude = negative ? -this.units : this.units

    const quotient = magnitude / divisor
    const remainder = magnitude % divisor
    // Half away from zero: a remainder of exactly half rounds up.
    const rounded = remainder * 2n >= divisor ? quotient + 1n : quotient

    return new Money(negative ? -rounded : rounded, MONEY_SCALE)
  }

  /** Drops trailing fractional zeros. Used only on the savings rate. */
  trim(): Money {
    let { units, scale } = this
    while (scale > 0 && units % TEN === 0n) {
      units /= TEN
      scale -= 1
    }
    return new Money(units, scale)
  }

  /** Renders as a bare decimal literal, keeping its scale. */
  toString(): string {
    if (this.scale === 0) return this.units.toString()

    const negative = this.units < 0n
    const digits = (negative ? -this.units : this.units).toString().padStart(this.scale + 1, '0')
    const whole = digits.slice(0, digits.length - this.scale)
    const fraction = digits.slice(digits.length - this.scale)
    return `${negative ? '-' : ''}${whole}.${fraction}`
  }

  /** Exactly two places, whatever the scale. The CSV column wants this. */
  toFixed2(): string {
    return this.scale === MONEY_SCALE ? this.toString() : this.rescale(MONEY_SCALE).toString()
  }

  private rescale(scale: number): Money {
    if (scale === this.scale) return this
    if (scale > this.scale) return new Money(this.units * pow10(scale - this.scale), scale)
    return this.round()
  }
}

export const MONEY_ZERO = Money.zero()

/** Parses a bound like "0.01"; the constants above are always well-formed. */
export function bound(text: string): Money {
  const parsed = Money.parse(text)
  if (parsed === null) throw new Error(`money: ${text} is not a decimal`)
  return parsed
}

// --- tags -------------------------------------------------------------------

/** Trims, drops blanks, and packs into the storage column. */
export function joinTags(tags: readonly string[] | null | undefined): string {
  return (tags ?? [])
    .map((tag) => tag.trim())
    .filter((tag) => tag.length > 0)
    .join(TAG_SEPARATOR)
}

/** Unpacks the storage column. Always an array, never null. */
export function splitTags(raw: string | null | undefined): string[] {
  if (!raw) return []
  return raw.split(TAG_SEPARATOR).filter((tag) => tag.length > 0)
}

/** Whitespace-only collapses to null, as the other backends do for Notes. */
export function trimmedOrNull(value: string | null | undefined): string | null {
  const text = (value ?? '').trim()
  return text.length > 0 ? text : null
}
