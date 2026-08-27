/** Process entry point: read the environment, open the pool, serve. */

import { createApp } from './app.js'
import { loadSettings } from './core/config.js'
import { createPool } from './db.js'

async function main(): Promise<void> {
  const settings = loadSettings()
  const pool = createPool(settings.databaseUrl)

  // Fail fast on a bad DATABASE_URL rather than serving errors per request.
  await pool.query('SELECT 1')

  const app = createApp(settings, pool)
  const server = app.listen(settings.port, () => {
    console.log(
      JSON.stringify({
        level: 'INFO',
        msg: 'listening',
        port: settings.port,
        allowedOrigins: settings.allowedOrigins,
      }),
    )
  })

  const shutdown = (signal: string): void => {
    console.log(JSON.stringify({ level: 'INFO', msg: 'shutting down', signal }))
    server.close(() => {
      void pool.end().then(() => process.exit(0))
    })
  }
  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
}

main().catch((error: unknown) => {
  console.error(
    JSON.stringify({
      level: 'ERROR',
      msg: 'startup failed',
      error: error instanceof Error ? error.message : String(error),
    }),
  )
  process.exit(1)
})
