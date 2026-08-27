/**
 * The Express application: middleware, the unauthenticated probes, and the
 * /api group the routers hang off.
 *
 * The service document at GET / names this backend so an operator can tell
 * which of the four answered a request.
 */

import express, { type Application, type NextFunction, type Request, type Response } from 'express'
import type { Settings } from './core/config.js'
import {
  AppError,
  FieldErrors,
  PROBLEM_CONTENT_TYPE,
  VALIDATION_CONTENT_TYPE,
  problemBody,
  validationBody,
} from './core/errors.js'
import { TokenService } from './core/security.js'
import type { DbPool } from './db.js'
import { renderJson } from './api/rendering.js'
import { registerRoutes } from './api/routers/index.js'

/** Everything a handler needs, resolved once at startup. */
export interface AppContext {
  pool: DbPool
  tokens: TokenService
  settings: Settings
}

declare module 'express-serve-static-core' {
  interface Request {
    context: AppContext
    userId?: string
  }
}

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/** Writes a payload with the renderer that keeps money and enums exact. */
export function send(response: Response, status: number, payload: unknown): void {
  response.status(status).type(VALIDATION_CONTENT_TYPE).send(renderJson(payload))
}

export function createApp(settings: Settings, pool: DbPool): Application {
  const app = express()
  const context: AppContext = { pool, tokens: new TokenService(settings.jwtSecret), settings }

  // Express advertises itself by default; nothing downstream needs to know.
  app.disable('x-powered-by')
  app.set('trust proxy', true)

  app.use((request: Request, _response: Response, next: NextFunction) => {
    request.context = context
    next()
  })

  app.use(cors(settings.allowedOrigins))
  app.use(express.json({ limit: '1mb' }))

  app.get('/health', (_request, response) => {
    response.type('text/plain').send('ok')
  })

  app.get('/', (_request, response) => {
    send(response, 200, {
      service: 'FinanceTracker API (Node)',
      status: 'ok',
      docs: '/swagger',
    })
  })

  registerRoutes(app)

  app.use(notFoundHandler)
  app.use(errorHandler)
  return app
}

/**
 * Exact-match origin echo, mirroring the .NET policy: WithOrigins(list) with
 * any header and any method, and no credentials support. Every OPTIONS answers
 * 204, matched origin or not.
 */
function cors(allowed: readonly string[]) {
  const permitted = new Set(allowed)

  return (request: Request, response: Response, next: NextFunction): void => {
    const origin = request.headers.origin
    if (typeof origin === 'string' && permitted.has(origin)) {
      response.setHeader('Access-Control-Allow-Origin', origin)
      response.appendHeader('Vary', 'Origin')

      if (request.method === 'OPTIONS') {
        response.setHeader(
          'Access-Control-Allow-Methods',
          'GET, POST, PUT, PATCH, DELETE, OPTIONS',
        )
        const requested = request.headers['access-control-request-headers']
        response.setHeader(
          'Access-Control-Allow-Headers',
          typeof requested === 'string' && requested ? requested : '*',
        )
        response.setHeader('Access-Control-Max-Age', '86400')
      }
    }

    if (request.method === 'OPTIONS') {
      response.status(204).end()
      return
    }
    next()
  }
}

/**
 * A path that reached no route. A non-uuid {id} segment answers a bare 404
 * with no body: the .NET routes constrain it with {id:guid}, so such a request
 * matches no route at all.
 */
function notFoundHandler(request: Request, response: Response): void {
  const segments = request.path.split('/').filter(Boolean)
  const candidate = segments[2]
  if (candidate !== undefined && !['export', 'import'].includes(candidate)) {
    if (!UUID_PATTERN.test(candidate)) {
      response.status(404).end()
      return
    }
  }
  response.status(404).end()
}

/** Maps a domain error onto its status and body. */
function errorHandler(
  error: unknown,
  request: Request,
  response: Response,
  next: NextFunction,
): void {
  if (response.headersSent) {
    next(error)
    return
  }

  if (error instanceof FieldErrors) {
    response.status(400).type(VALIDATION_CONTENT_TYPE).send(renderJson(validationBody(error.errors)))
    return
  }

  if (error instanceof AppError) {
    if (error.kind === 'unauthorized') response.setHeader('WWW-Authenticate', 'Bearer')
    response
      .status(error.status)
      .type(PROBLEM_CONTENT_TYPE)
      .send(renderJson(problemBody(error.status, error.title, error.message, request.path)))
    return
  }

  // A malformed JSON body surfaces as a SyntaxError from the body parser.
  if (error instanceof SyntaxError && 'body' in error) {
    const errs = new FieldErrors()
    errs.add('$', error.message)
    response.status(400).type(VALIDATION_CONTENT_TYPE).send(renderJson(validationBody(errs.errors)))
    return
  }

  console.error('unhandled error', error)
  response
    .status(500)
    .type(PROBLEM_CONTENT_TYPE)
    .send(
      renderJson(
        problemBody(500, 'Internal Server Error', 'An unexpected error occurred.', request.path),
      ),
    )
}
