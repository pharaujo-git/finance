/**
 * CSV export and import.
 *
 * The reader is hand-written rather than pulled from a library: it has to match
 * the other backends byte for byte, including treating a bad row as *skipped*
 * rather than fatal, and ignoring bare carriage returns outside quotes.
 */

import { randomUUID } from 'node:crypto'
import { validationError } from '../core/errors.js'
import { TransactionType, parseEnum } from '../domain/enums.js'
import { Instant } from '../domain/instant.js'
import { MONEY_ZERO, Money, trimmedOrNull } from '../domain/money.js'
import type {
  AccountRepository,
  CategoryRepository,
  Transaction,
  TransactionRepository,
} from '../repositories/index.js'

export const CSV_HEADER = [
  'Date',
  'Type',
  'Amount',
  'Account',
  'Category',
  'Description',
  'Notes',
  'Tags',
]

/** The column separator for tags, distinct from the storage unit separator. */
const CSV_TAG_DELIMITER = ';'

export const EMPTY_CSV_MESSAGE = 'The uploaded file contains no rows.'
export const MISSING_FILE_MESSAGE = 'A non-empty CSV file is required.'
export const OVERSIZE_MESSAGE = 'The uploaded file is larger than 5 MB.'
export const MAX_UPLOAD_BYTES = 5 * 1024 * 1024

const CURRENCY_SYMBOLS = new Set(['¤', '$', '£', '€'])

export function exportFileName(now: Date): string {
  return `transactions-${isoDate(now)}.csv`
}

function isoDate(value: Date): string {
  return value.toISOString().slice(0, 10)
}

/** RFC 4180: quote only when needed, and double any internal quote. */
export function escapeCsvField(value: string): string {
  if (/[",\n\r]/.test(value)) return `"${value.replace(/"/g, '""')}"`
  return value
}

export function csvRow(fields: readonly string[]): string {
  // A bare \n, never \r\n -- that is what the other backends write.
  return `${fields.map(escapeCsvField).join(',')}\n`
}

/**
 * A minimal RFC 4180 reader. Outside quotes a carriage return is dropped
 * entirely, so CRLF files parse the same as LF ones, and a trailing newline
 * produces no empty final record.
 */
export function parseCsv(text: string): string[][] {
  const rows: string[][] = []
  let fields: string[] = []
  let buffer = ''
  let quoted = false
  let touched = false

  const commitRow = (): void => {
    if (!touched && buffer === '' && fields.length === 0) return
    fields.push(buffer)
    rows.push(fields)
    fields = []
    buffer = ''
    touched = false
  }

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index]!
    if (quoted) {
      if (char === '"') {
        if (text[index + 1] === '"') {
          buffer += '"'
          index += 1
          continue
        }
        quoted = false
      } else {
        // Quoted fields may span line breaks.
        buffer += char
      }
      continue
    }

    if (char === '"') {
      quoted = true
      touched = true
    } else if (char === ',') {
      fields.push(buffer)
      buffer = ''
      touched = true
    } else if (char === '\r') {
      // ignored outside quotes
    } else if (char === '\n') {
      commitRow()
    } else {
      buffer += char
      touched = true
    }
  }

  commitRow()
  return rows
}

/** Most specific first; a value with no zone is read as UTC. */
export function parseCsvDate(value: string): Instant | null {
  const text = value.trim()
  if (!text) return null

  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) return instantFrom(`${text}T00:00:00Z`)
  if (/^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}$/.test(text)) {
    return instantFrom(`${text.replace(' ', 'T')}Z`)
  }
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/.test(text)) {
    return instantFrom(text)
  }

  // M/D/YYYY, optionally with a clock and an AM/PM marker.
  const slash =
    /^(\d{1,2})\/(\d{1,2})\/(\d{4})(?:[ T](\d{1,2}):(\d{2}):(\d{2})(?:\s*(AM|PM))?)?$/i.exec(
      text,
    )
  if (slash) {
    const [, month, day, year, hour = '0', minute = '0', second = '0', meridiem] = slash
    const monthNumber = Number(month)
    const dayNumber = Number(day)

    // Date.UTC rolls an out-of-range field over into the next year rather than
    // failing, so 26/08/2026 would silently become February 2028. The other
    // backends reject it, because no layout of theirs matches a 26th month.
    if (monthNumber < 1 || monthNumber > 12) return null
    const lastDay = new Date(Date.UTC(Number(year), monthNumber, 0)).getUTCDate()
    if (dayNumber < 1 || dayNumber > lastDay) return null

    let hours = Number(hour)
    if (meridiem) {
      const upper = meridiem.toUpperCase()
      if (upper === 'PM' && hours < 12) hours += 12
      if (upper === 'AM' && hours === 12) hours = 0
    }
    if (hours > 23 || Number(minute) > 59 || Number(second) > 59) return null

    const epoch = Date.UTC(
      Number(year),
      monthNumber - 1,
      dayNumber,
      hours,
      Number(minute),
      Number(second),
    )
    return Number.isNaN(epoch) ? null : Instant.fromDate(new Date(epoch))
  }

  return null
}

function instantFrom(candidate: string): Instant | null {
  const parsed = new Date(candidate)
  return Number.isNaN(parsed.getTime()) ? null : Instant.fromDate(parsed)
}

/** Reads the shapes .NET's NumberStyles.Currency accepts. */
export function parseCurrencyAmount(value: string): Money | null {
  let text = value.trim()
  let negative = false

  if (text.startsWith('(') && text.endsWith(')')) {
    negative = true
    text = text.slice(1, -1).trim()
  }

  while (text.length > 0 && CURRENCY_SYMBOLS.has(text[0]!)) {
    text = text.slice(1)
  }
  text = text.replace(/,/g, '').trim()

  if (text.endsWith('-')) {
    negative = true
    text = text.slice(0, -1)
  }

  const amount = Money.parse(text.trim())
  if (amount === null) return null
  return negative ? amount.negate() : amount
}

function field(row: readonly string[], index: number): string {
  return (row[index] ?? '').trim()
}

function isHeaderRow(row: readonly string[]): boolean {
  return row.length > 0 && (row[0] ?? '').trim().toLowerCase() === 'date'
}

export class CsvService {
  constructor(
    private readonly transactions: TransactionRepository,
    private readonly accounts: AccountRepository,
    private readonly categories: CategoryRepository,
  ) {}

  async export(
    userId: string,
    dateFrom: Instant | null,
    dateTo: Instant | null,
  ): Promise<string> {
    const items = await this.transactions.listRange(userId, dateFrom, dateTo)
    const accountNames = new Map(
      (await this.accounts.listAll(userId)).map((account) => [account.id, account.name]),
    )
    const categoryNames = new Map(
      (await this.categories.listVisible(userId)).map((item) => [item.id, item.name]),
    )

    let out = csvRow(CSV_HEADER)
    for (const item of items) {
      out += csvRow([
        isoDate(item.date.date),
        item.type.wireName,
        // Always exactly two places here, unlike the JSON shape.
        item.amount.toFixed2(),
        accountNames.get(item.accountId) ?? '',
        item.categoryId ? (categoryNames.get(item.categoryId) ?? '') : '',
        item.description,
        item.notes ?? '',
        item.tags.join(CSV_TAG_DELIMITER),
      ])
    }
    return out
  }

  async import(userId: string, content: Buffer): Promise<{ imported: number; skipped: number }> {
    // Strip a leading UTF-8 BOM, which spreadsheets like to add.
    const text = content.toString('utf8').replace(/^\uFEFF/, '')
    let rows = parseCsv(text)
    if (rows.length === 0) throw validationError(EMPTY_CSV_MESSAGE)

    // Accounts: the last name wins. Categories: the first does. That asymmetry
    // is inherited from the .NET lookups and kept deliberately.
    const accountIds = new Map<string, string>()
    for (const account of await this.accounts.listAll(userId)) {
      accountIds.set(account.name.toUpperCase(), account.id)
    }
    const categoryIds = new Map<string, string>()
    for (const category of await this.categories.listVisible(userId)) {
      if (!categoryIds.has(category.name.toUpperCase())) {
        categoryIds.set(category.name.toUpperCase(), category.id)
      }
    }

    if (rows[0] && isHeaderRow(rows[0])) rows = rows.slice(1)

    const imported: Transaction[] = []
    let skipped = 0
    for (const row of rows) {
      const built = buildRow(userId, row, accountIds, categoryIds)
      if (built === null) skipped += 1
      else imported.push(built)
    }

    if (imported.length > 0) await this.transactions.addMany(imported)
    return { imported: imported.length, skipped }
  }
}

/** Returns null for any unusable row; the caller counts it as skipped. */
function buildRow(
  userId: string,
  row: readonly string[],
  accountIds: Map<string, string>,
  categoryIds: Map<string, string>,
): Transaction | null {
  if (row.length < 6) return null

  const date = parseCsvDate(field(row, 0))
  if (date === null) return null

  const type = parseEnum('TransactionType', field(row, 1))
  // Transfers carry no destination in this CSV shape, so they never import.
  if (type === null || type.equals(TransactionType.Transfer)) return null

  const amount = parseCurrencyAmount(field(row, 2))
  if (amount === null || !amount.greaterThan(MONEY_ZERO)) return null

  const accountId = accountIds.get(field(row, 3).toUpperCase())
  if (accountId === undefined) return null

  // A missing category is fine; the transaction is simply uncategorized.
  const categoryId = categoryIds.get(field(row, 4).toUpperCase()) ?? null
  const rawTags = field(row, 7)

  return {
    id: randomUUID(),
    userId,
    accountId,
    categoryId,
    type,
    amount: amount.round(),
    date,
    description: field(row, 5),
    notes: trimmedOrNull(field(row, 6)),
    tags: rawTags ? rawTags.split(CSV_TAG_DELIMITER) : [],
    transferAccountId: null,
  }
}
