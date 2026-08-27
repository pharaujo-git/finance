/**
 * Postgres connection pool.
 *
 * The schema is owned by the backend-neutral dbmate migrations in
 * db/migrations, so nothing here issues DDL. Table and column names are EF
 * Core's quoted PascalCase, which is why every query quotes its identifiers.
 */

import pg from 'pg'
import { Instant } from './domain/instant.js'
import { Money } from './domain/money.js'

const { Pool, types } = pg

const TIMESTAMPTZ_OID = 1184
const TIMESTAMP_OID = 1114

/**
 * Timestamps arrive as text rather than a Date: a Date holds milliseconds and
 * the column holds microseconds, and truncating them would put a different
 * value on the wire than the other three backends emit.
 */
types.setTypeParser(TIMESTAMPTZ_OID, (value: string) => value)
types.setTypeParser(TIMESTAMP_OID, (value: string) => value)

export type DbPool = pg.Pool
export type DbClient = pg.PoolClient | pg.Pool

/** Opens the pool used for the lifetime of the process. */
export function createPool(databaseUrl: string, overrides: pg.PoolConfig = {}): DbPool {
  return new Pool({
    connectionString: databaseUrl,
    max: 10,
    // A query that hangs should fail the request, not the process.
    statement_timeout: 30_000,
    ...overrides,
  })
}

/** numeric comes back as text, which is exactly what Money wants. */
export function readMoney(value: unknown): Money {
  if (typeof value === 'number') return Money.parse(String(value)) ?? Money.zero()
  if (typeof value !== 'string') return Money.zero()
  return Money.parse(value) ?? Money.zero()
}

export function readInstant(value: unknown): Instant {
  return Instant.fromPg(value as string | Date)
}

export function readInstantOrNull(value: unknown): Instant | null {
  return value === null || value === undefined ? null : readInstant(value)
}
