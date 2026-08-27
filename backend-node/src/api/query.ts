/**
 * Query-string binding that mirrors MVC's model binder.
 *
 * Conversion failures are collected rather than raised one at a time, so a
 * request with three bad values reports all three. An *absent* key is not the
 * same as an empty one: "?month=" is a failure, not "this month".
 */

import type { Request } from 'express'
import { FieldErrors } from '../core/errors.js'
import { type EnumKind, type EnumValue, parseEnum } from '../domain/enums.js'
import { type Instant, parseWireDate } from '../domain/instant.js'
import { invalidValueMessage } from '../domain/validation.js'

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export class QueryReader {
  private readonly params: Record<string, unknown>
  private readonly errs = new FieldErrors()

  constructor(request: Request) {
    this.params = request.query
  }

  /** null when the caller omitted the key entirely. */
  text(key: string): string | null {
    const value = this.params[key]
    if (value === undefined) return null
    // A repeated key arrives as an array; the binder reads the first one.
    if (Array.isArray(value)) return typeof value[0] === 'string' ? value[0] : ''
    return typeof value === 'string' ? value : ''
  }

  number(key: string, field: string): number | null {
    const raw = this.text(key)
    if (raw === null) return null

    // Deliberately strict: MVC's binder rejects "12abc" rather than reading 12.
    if (!/^[+-]?\d+$/.test(raw.trim())) {
      this.errs.add(field, invalidValueMessage(raw, field))
      return null
    }
    return Number(raw)
  }

  numberOr(key: string, field: string, fallback: number): number {
    const parsed = this.number(key, field)
    return parsed ?? fallback
  }

  identifier(key: string, field: string): string | null {
    const raw = this.text(key)
    if (raw === null) return null
    if (!UUID_PATTERN.test(raw)) {
      this.errs.add(field, invalidValueMessage(raw, field))
      return null
    }
    return raw
  }

  moment(key: string, field: string): Instant | null {
    const raw = this.text(key)
    if (raw === null) return null

    const parsed = parseWireDate(raw)
    if (parsed === null) this.errs.add(field, invalidValueMessage(raw, field))
    return parsed
  }

  enum(key: string, field: string, kind: EnumKind): EnumValue | null {
    const raw = this.text(key)
    if (raw === null) return null

    const parsed = parseEnum(kind, raw)
    if (parsed === null) this.errs.add(field, invalidValueMessage(raw, field))
    return parsed
  }

  /** Throws the accumulated conversion failures, if any. */
  done(): void {
    this.errs.raiseIfAny()
  }
}
