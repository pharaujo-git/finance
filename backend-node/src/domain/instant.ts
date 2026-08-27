/**
 * A moment in time that keeps the precision Postgres stores.
 *
 * A JavaScript Date holds milliseconds. The column is `timestamp with time
 * zone`, which holds microseconds, and the other three backends put all six
 * digits on the wire. Rendering through a Date would silently truncate
 * 22:27:26.516550Z to 22:27:26.516Z, so the original text is carried alongside
 * a Date used only for comparison and arithmetic.
 */

/** Matches what pg hands back for a timestamptz, e.g. "2026-08-26 22:27:26.51655+00". */
const PG_TIMESTAMP =
  /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(?:([+-]\d{2})(?::?(\d{2}))?|Z)?$/

export class Instant {
  /** Millisecond precision: enough for every window and ordering decision. */
  readonly date: Date
  /** Sub-millisecond digits the Date cannot hold, or '' when there are none. */
  private readonly extraDigits: string

  private constructor(date: Date, extraDigits: string) {
    this.date = date
    this.extraDigits = extraDigits
  }

  static fromDate(value: Date): Instant {
    return new Instant(value, '')
  }

  static now(): Instant {
    return new Instant(new Date(), '')
  }

  /** Reads the text form pg produces, keeping every fractional digit. */
  static fromPg(raw: string | Date): Instant {
    if (raw instanceof Date) return Instant.fromDate(raw)

    const match = PG_TIMESTAMP.exec(raw.trim())
    if (match === null) {
      const fallback = new Date(raw)
      return new Instant(Number.isNaN(fallback.getTime()) ? new Date(0) : fallback, '')
    }

    const [, year, month, day, hour, minute, second, fraction = '', offsetHour, offsetMinute] =
      match
    // Pad to microseconds so the split below is predictable.
    const digits = fraction.padEnd(6, '0')
    const millis = digits.slice(0, 3)
    const extra = digits.slice(3).replace(/0+$/, '')

    let epoch = Date.UTC(
      Number(year),
      Number(month) - 1,
      Number(day),
      Number(hour),
      Number(minute),
      Number(second),
      Number(millis),
    )
    if (offsetHour !== undefined) {
      const sign = offsetHour.startsWith('-') ? -1 : 1
      const hours = Math.abs(Number(offsetHour))
      const minutes = Number(offsetMinute ?? '0')
      epoch -= sign * (hours * 60 + minutes) * 60_000
    }

    return new Instant(new Date(epoch), extra)
  }

  /** Epoch milliseconds, so `<` and `>=` work directly on instants. */
  valueOf(): number {
    return this.date.getTime()
  }

  /** RFC3339 in UTC, with trailing zeros trimmed off the fraction. */
  toString(): string {
    const iso = this.date.toISOString() // ...THH:MM:SS.mmmZ
    const base = iso.slice(0, 19)
    const millis = iso.slice(20, 23)

    const fraction = `${millis}${this.extraDigits}`.replace(/0+$/, '')
    return fraction.length > 0 ? `${base}.${fraction}Z` : `${base}Z`
  }

  toJSON(): string {
    return this.toString()
  }

  /** The value handed to pg for a parameter. */
  toParameter(): Date {
    return this.date
  }
}

/**
 * Accepts the layouts the model binder does, assuming UTC when no zone is
 * given: RFC3339, second- and minute-precision local timestamps, and the bare
 * date the frontend's <input type="date"> posts.
 */
export function parseWireDate(raw: string): Instant | null {
  const text = (raw ?? '').trim()
  if (!text) return null

  let candidate: string | null = null
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) {
    candidate = `${text}T00:00:00Z`
  } else if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?$/.test(text)) {
    candidate = `${text}Z`
  } else if (
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/.test(text)
  ) {
    candidate = text
  }
  if (candidate === null) return null

  const parsed = new Date(candidate)
  return Number.isNaN(parsed.getTime()) ? null : Instant.fromDate(parsed)
}
