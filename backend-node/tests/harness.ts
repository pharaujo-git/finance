/**
 * Integration harness: a throwaway Postgres schema per test.
 *
 * Mirrors backend-go/internal/pgtest and backend-py/tests/conftest.py. The
 * schema is built from the real baseline migration in db/migrations, so the
 * tests run against the shape that actually ships. Without TEST_DATABASE_URL
 * every test that needs Postgres skips, and `vitest` stays green with no
 * Docker.
 */

import { randomUUID } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import type { Application } from 'express'
import { createApp } from '../src/express-app.js'
import type { Settings } from '../src/core/config.js'
import { LOCAL_DEVELOPMENT_SECRET } from '../src/core/config.js'
import { createPool, type DbPool } from '../src/db.js'

/** Deliberately not DATABASE_URL: a developer's real database is expensive to hit. */
const DATABASE_URL_VARIABLE = 'TEST_DATABASE_URL'

const MIGRATIONS = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'db', 'migrations')

export const DEMO_PASSWORD = 'Passw0rd!123'

export function databaseUrl(): string | null {
  const url = (process.env[DATABASE_URL_VARIABLE] ?? '').trim()
  return url || null
}

/** The `-- migrate:up` half of a dbmate migration. */
function migrationUp(name: string): string {
  const text = readFileSync(join(MIGRATIONS, name), 'utf8')
  return text.split(/^--\s*migrate:down\s*$/m)[0]!.replace(/^--\s*migrate:up\s*$/m, '')
}

export interface Harness {
  app: Application
  pool: DbPool
  close: () => Promise<void>
}

/** Opens a pool pinned to a schema of its own and builds the app on it. */
export async function createHarness(): Promise<Harness> {
  const url = databaseUrl()
  if (url === null) throw new Error(`${DATABASE_URL_VARIABLE} is not set`)

  const schema = `vitest_${randomUUID().replace(/-/g, '').slice(0, 20)}`

  const admin = createPool(url)
  await admin.query(`CREATE SCHEMA "${schema}"`)
  await admin.end()

  const pool = createPool(url, { options: `-c search_path=${schema}` })
  for (const name of ['0001_baseline.sql', '0002_seed_default_categories.sql']) {
    await pool.query(migrationUp(name))
  }

  const settings: Settings = {
    databaseUrl: url,
    jwtSecret: LOCAL_DEVELOPMENT_SECRET,
    port: 8083,
    allowedOrigins: ['http://localhost:5173'],
  }

  return {
    app: createApp(settings, pool),
    pool,
    close: async () => {
      await pool.end()
      const cleanup = createPool(url)
      await cleanup.query(`DROP SCHEMA "${schema}" CASCADE`)
      await cleanup.end()
    },
  }
}
