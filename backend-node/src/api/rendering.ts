/**
 * JSON rendering with exact control over how each scalar reaches the wire.
 *
 * `JSON.stringify` will not do. Money must arrive as a bare JSON number that
 * keeps the scale it carries (1250.00, not 1250 and not "1250.00"), which no
 * JavaScript number can represent -- 1250.00 and 1250 are the same value. So
 * the document is written directly, and a Money renders from its own digits.
 */

import { EnumValue } from '../domain/enums.js'
import { Instant } from '../domain/instant.js'
import { Money } from '../domain/money.js'

function write(value: unknown, out: string[]): void {
  if (value === null || value === undefined) {
    out.push('null')
    return
  }
  if (value instanceof Money) {
    out.push(value.toString())
    return
  }
  if (value instanceof EnumValue) {
    // An ordinal naming no member is written back as a bare number, exactly as
    // JsonStringEnumConverter does.
    out.push(value.isDefined ? JSON.stringify(value.wireName) : String(value.ordinal))
    return
  }
  if (value instanceof Instant) {
    out.push(JSON.stringify(value.toString()))
    return
  }
  if (value instanceof Date) {
    out.push(JSON.stringify(Instant.fromDate(value).toString()))
    return
  }

  switch (typeof value) {
    case 'boolean':
      out.push(value ? 'true' : 'false')
      return
    case 'number':
      out.push(Number.isFinite(value) ? String(value) : 'null')
      return
    case 'bigint':
      out.push(value.toString())
      return
    case 'string':
      out.push(JSON.stringify(value))
      return
    default:
      break
  }

  if (Array.isArray(value)) {
    out.push('[')
    value.forEach((item, index) => {
      if (index > 0) out.push(',')
      write(item, out)
    })
    out.push(']')
    return
  }

  out.push('{')
  let first = true
  for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
    if (item === undefined) continue
    if (!first) out.push(',')
    first = false
    out.push(JSON.stringify(key), ':')
    write(item, out)
  }
  out.push('}')
}

export function renderJson(payload: unknown): string {
  const out: string[] = []
  write(payload, out)
  return out.join('')
}
