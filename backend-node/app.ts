/**
 * Vercel entry point.
 *
 * Vercel's Node runtime wraps this module's default export, which must be a
 * function or a server -- an Express application is already a `(req, res)`
 * function, so the real app is handed over and no listener is created here.
 * Vercel owns the socket; `src/main.ts` still owns it for the container build,
 * and the app itself is identical in both.
 *
 * Three details are load-bearing:
 *
 *  - The runtime only accepts an entrypoint that imports `express` itself, so
 *    the real app is mounted behind a thin outer instance rather than being
 *    re-exported. `createApp` is untouched.
 *  - The file is named `app.ts` at the root because the runtime searches there
 *    before `src/`. `src/app.ts` was renamed to `src/express-app.ts` so it can
 *    no longer be mistaken for an entrypoint: its default export is a factory.
 *  - It imports from `src/` rather than the compiled `dist/`, because `dist/`
 *    is gitignored and therefore never uploaded.
 */
import express from 'express'
import { createApp } from './src/express-app.js'
import { loadSettings } from './src/core/config.js'
import { createPool } from './src/db.js'

const settings = loadSettings()

// One connection per instance. Each invocation serves a single request, and
// Neon's pooler is what actually fans out; the container keeps max: 10 because
// there one process really does serve concurrently.
const pool = createPool(settings.databaseUrl, { max: 1 })

const server = express()
server.use(createApp(settings, pool))

export default server
