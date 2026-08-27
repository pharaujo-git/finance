/**
 * Routers: HTTP only. Each handler binds the request, calls one service and
 * renders the result -- every rule, message and status code lives below.
 */

import express, { type Application, type Request, type Response, type Router } from 'express'
import multer from 'multer'
import { send } from '../../app.js'
import { FieldErrors, unauthorized, validationError } from '../../core/errors.js'
import { InvalidTokenError } from '../../core/security.js'
import { FIELD, LIMITS, intRange, maxLength } from '../../domain/validation.js'
import {
  AccountRepository,
  BudgetRepository,
  CategoryRepository,
  GoalRepository,
  RecurringRepository,
  TransactionRepository,
  UserRepository,
} from '../../repositories/index.js'
import {
  AccountService,
  AuthService,
  BudgetService,
  CategoryService,
  GoalService,
  TransactionService,
} from '../../services/index.js'
import {
  DEFAULT_CASHFLOW_MONTHS,
  DEFAULT_NET_WORTH_MONTHS,
  AnalyticsService,
} from '../../services/analytics.js'
import {
  CsvService,
  MAX_UPLOAD_BYTES,
  MISSING_FILE_MESSAGE,
  OVERSIZE_MESSAGE,
  exportFileName,
} from '../../services/csv.js'
import { RecurringService } from '../../services/recurring.js'
import { QueryReader } from '../query.js'
import {
  parseAccount,
  parseCategory,
  parseContribute,
  parseCreateBudget,
  parseGoal,
  parseLogin,
  parseRecurring,
  parseRegister,
  parseTransaction,
  parseUpdateBudget,
  parseUpdateProfile,
} from '../schemas.js'

const DEFAULT_PAGE = 1
const DEFAULT_PAGE_SIZE = 20
const MAX_PAGE_SIZE = 200
const MAX_INT32 = 2_147_483_647

const MISSING_TOKEN_DETAIL = 'Authentication is required.'
const BAD_TOKEN_DETAIL = 'The access token is invalid or has expired.'
const MISSING_CALLER_DETAIL = 'The access token does not identify a user.'

/** Wraps an async handler so a rejection reaches the error middleware. */
function handle(
  fn: (request: Request, response: Response) => Promise<void>,
): (request: Request, response: Response, next: (error?: unknown) => void) => void {
  return (request, response, next) => {
    fn(request, response).catch(next)
  }
}

/** Validates the bearer token and puts the caller's id on the request. */
function requireAuth(request: Request, _response: Response, next: (error?: unknown) => void): void {
  const header = request.headers.authorization ?? ''
  if (!header.startsWith('Bearer ')) {
    next(unauthorized(MISSING_TOKEN_DETAIL))
    return
  }

  const raw = header.slice('Bearer '.length).trim()
  if (!raw) {
    next(unauthorized(MISSING_TOKEN_DETAIL))
    return
  }

  try {
    request.userId = request.context.tokens.validate(raw).userId
    next()
  } catch (error) {
    next(error instanceof InvalidTokenError ? unauthorized(BAD_TOKEN_DETAIL) : error)
  }
}

function caller(request: Request): string {
  const userId = request.userId
  if (!userId) throw unauthorized(MISSING_CALLER_DETAIL)
  return userId
}

function pathId(request: Request, name = 'id'): string {
  // Express types a param as string | string[]; these routes declare it once.
  const value: unknown = request.params[name]
  return typeof value === 'string' ? value : ''
}

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/**
 * The equivalent of the .NET routes' {id:guid} constraint: a segment that is
 * not a uuid matches no route at all, so it answers a bare 404 with no body
 * rather than reaching the database and failing there.
 */
function withIdGuard(router: Router): Router {
  router.param('id', (_request, response, next, value: string) => {
    if (!UUID_PATTERN.test(value)) {
      response.status(404).end()
      return
    }
    next()
  })
  return router
}

// A single in-memory upload; the CSV is parsed and discarded within the request.
const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: MAX_UPLOAD_BYTES } })

export function registerRoutes(app: Application): void {
  const api = express.Router()

  api.use('/auth', authRouter())
  api.use(requireAuth)
  api.use('/accounts', accountsRouter())
  api.use('/categories', categoriesRouter())
  api.use('/transactions', transactionsRouter())
  api.use('/recurring', recurringRouter())
  api.use('/budgets', budgetsRouter())
  api.use('/goals', goalsRouter())
  api.use(analyticsRouter())

  app.use('/api', api)
}

// --- auth -------------------------------------------------------------------

function authService(request: Request): AuthService {
  return new AuthService(new UserRepository(request.context.pool), request.context.tokens)
}

function authRouter(): Router {
  const router = express.Router()

  // Register and login are the only anonymous routes in the API.
  router.post(
    '/register',
    handle(async (request, response) => {
      const body = parseRegister(request.body)
      send(response, 200, await authService(request).register(body.email, body.password, body.name))
    }),
  )

  router.post(
    '/login',
    handle(async (request, response) => {
      const body = parseLogin(request.body)
      send(response, 200, await authService(request).login(body.email, body.password))
    }),
  )

  router.get(
    '/me',
    requireAuth,
    handle(async (request, response) => {
      send(response, 200, await authService(request).profile(caller(request)))
    }),
  )

  router.put(
    '/me',
    requireAuth,
    handle(async (request, response) => {
      const body = parseUpdateProfile(request.body)
      send(
        response,
        200,
        await authService(request).updateProfile(caller(request), body.name, body.currency),
      )
    }),
  )

  return router
}

// --- accounts ---------------------------------------------------------------

function accountService(request: Request): AccountService {
  return new AccountService(
    new AccountRepository(request.context.pool),
    new TransactionRepository(request.context.pool),
  )
}

function accountsRouter(): Router {
  const router = withIdGuard(express.Router())

  router.get(
    '/',
    handle(async (request, response) => {
      send(response, 200, await accountService(request).listAll(caller(request)))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      send(response, 200, await accountService(request).create(caller(request), parseAccount(request.body)))
    }),
  )

  router.get(
    '/:id',
    handle(async (request, response) => {
      send(response, 200, await accountService(request).get(caller(request), pathId(request)))
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      send(
        response,
        200,
        await accountService(request).update(caller(request), pathId(request), parseAccount(request.body)),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await accountService(request).archive(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  return router
}

// --- categories -------------------------------------------------------------

function categoryService(request: Request): CategoryService {
  return new CategoryService(new CategoryRepository(request.context.pool))
}

function categoriesRouter(): Router {
  const router = withIdGuard(express.Router())

  // There is deliberately no GET by id here.
  router.get(
    '/',
    handle(async (request, response) => {
      send(response, 200, await categoryService(request).listAll(caller(request)))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      send(response, 200, await categoryService(request).create(caller(request), parseCategory(request.body)))
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      send(
        response,
        200,
        await categoryService(request).update(caller(request), pathId(request), parseCategory(request.body)),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await categoryService(request).remove(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  return router
}

// --- transactions -----------------------------------------------------------

function transactionService(request: Request): TransactionService {
  return new TransactionService(
    new TransactionRepository(request.context.pool),
    new AccountRepository(request.context.pool),
    categoryService(request),
  )
}

function csvService(request: Request): CsvService {
  return new CsvService(
    new TransactionRepository(request.context.pool),
    new AccountRepository(request.context.pool),
    new CategoryRepository(request.context.pool),
  )
}

function transactionsRouter(): Router {
  const router = withIdGuard(express.Router())

  router.get(
    '/',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      const query = {
        page: reader.numberOr('page', 'Page', DEFAULT_PAGE),
        pageSize: reader.numberOr('pageSize', 'PageSize', DEFAULT_PAGE_SIZE),
        accountId: reader.identifier('accountId', 'AccountId'),
        categoryId: reader.identifier('categoryId', 'CategoryId'),
        type: reader.enum('type', 'Type', 'TransactionType'),
        dateFrom: reader.moment('from', 'From'),
        dateTo: reader.moment('to', 'To'),
        search: reader.text('search') ?? '',
      }
      reader.done()
      validatePaging(query.page, query.pageSize, query.search)

      send(response, 200, await transactionService(request).search(caller(request), query))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      send(
        response,
        200,
        await transactionService(request).create(caller(request), parseTransaction(request.body)),
      )
    }),
  )

  // Declared before /:id so the literal segments are not swallowed.
  router.get(
    '/export',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      // Lowercase field keys here, unlike the search endpoint above -- that
      // asymmetry comes from the .NET action signatures.
      const dateFrom = reader.moment('from', 'from')
      const dateTo = reader.moment('to', 'to')
      reader.done()

      const body = await csvService(request).export(caller(request), dateFrom, dateTo)
      const name = exportFileName(new Date())
      response
        .status(200)
        .type('text/csv')
        .setHeader(
          'Content-Disposition',
          `attachment; filename=${name}; filename*=UTF-8''${name}`,
        )
      response.send(body)
    }),
  )

  router.post(
    '/import',
    upload.single('file'),
    handle(async (request, response) => {
      const file = request.file
      if (!file || file.size === 0) throw validationError(MISSING_FILE_MESSAGE)
      if (file.size > MAX_UPLOAD_BYTES) throw validationError(OVERSIZE_MESSAGE)

      send(response, 200, await csvService(request).import(caller(request), file.buffer))
    }),
  )

  router.get(
    '/:id',
    handle(async (request, response) => {
      send(response, 200, await transactionService(request).get(caller(request), pathId(request)))
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      send(
        response,
        200,
        await transactionService(request).update(
          caller(request),
          pathId(request),
          parseTransaction(request.body),
        ),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await transactionService(request).remove(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  return router
}

/** The paging bounds the service enforces, reported like any other field. */
function validatePaging(page: number, pageSize: number, search: string): void {
  const errs = new FieldErrors()
  intRange(errs, FIELD.page, page, 1, MAX_INT32)
  intRange(errs, FIELD.pageSize, pageSize, 1, MAX_PAGE_SIZE)
  maxLength(errs, FIELD.search, search, LIMITS.search)
  errs.raiseIfAny()
}

// --- budgets ----------------------------------------------------------------

function budgetService(request: Request): BudgetService {
  return new BudgetService(
    new BudgetRepository(request.context.pool),
    new TransactionRepository(request.context.pool),
    categoryService(request),
  )
}

function budgetsRouter(): Router {
  const router = withIdGuard(express.Router())

  router.get(
    '/',
    handle(async (request, response) => {
      // An absent key means "this month"; an empty one is a failure.
      const month = new QueryReader(request).text('month')
      send(response, 200, await budgetService(request).listAll(caller(request), month))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      const body = parseCreateBudget(request.body)
      send(
        response,
        200,
        await budgetService(request).create(
          caller(request),
          body.categoryId,
          body.month,
          body.limit,
        ),
      )
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      const body = parseUpdateBudget(request.body)
      send(
        response,
        200,
        await budgetService(request).update(caller(request), pathId(request), body.limit),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await budgetService(request).remove(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  return router
}

// --- goals ------------------------------------------------------------------

function goalService(request: Request): GoalService {
  return new GoalService(new GoalRepository(request.context.pool))
}

function goalsRouter(): Router {
  const router = withIdGuard(express.Router())

  router.get(
    '/',
    handle(async (request, response) => {
      send(response, 200, await goalService(request).listAll(caller(request)))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      send(response, 200, await goalService(request).create(caller(request), parseGoal(request.body)))
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      send(
        response,
        200,
        await goalService(request).update(caller(request), pathId(request), parseGoal(request.body)),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await goalService(request).remove(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  router.post(
    '/:id/contribute',
    handle(async (request, response) => {
      const body = parseContribute(request.body)
      send(
        response,
        200,
        await goalService(request).contribute(caller(request), pathId(request), body.amount),
      )
    }),
  )

  return router
}

// --- recurring --------------------------------------------------------------

function recurringService(request: Request): RecurringService {
  return new RecurringService(
    new RecurringRepository(request.context.pool),
    new TransactionRepository(request.context.pool),
    new AccountRepository(request.context.pool),
    categoryService(request),
  )
}

function recurringRouter(): Router {
  const router = withIdGuard(express.Router())

  router.get(
    '/',
    handle(async (request, response) => {
      send(response, 200, await recurringService(request).listAll(caller(request)))
    }),
  )

  router.post(
    '/',
    handle(async (request, response) => {
      send(
        response,
        200,
        await recurringService(request).create(caller(request), parseRecurring(request.body)),
      )
    }),
  )

  router.put(
    '/:id',
    handle(async (request, response) => {
      send(
        response,
        200,
        await recurringService(request).update(
          caller(request),
          pathId(request),
          parseRecurring(request.body),
        ),
      )
    }),
  )

  router.delete(
    '/:id',
    handle(async (request, response) => {
      await recurringService(request).remove(caller(request), pathId(request))
      response.status(204).end()
    }),
  )

  return router
}

// --- analytics --------------------------------------------------------------

function analyticsService(request: Request): AnalyticsService {
  return new AnalyticsService(
    new TransactionRepository(request.context.pool),
    new AccountRepository(request.context.pool),
    categoryService(request),
  )
}

/**
 * The query field keys here are lowercase (months, year, from, to), unlike the
 * transaction search's PascalCase ones. That difference is inherited from the
 * .NET action signatures and the parity tests assert it.
 */
function analyticsRouter(): Router {
  const router = express.Router()

  router.get(
    '/dashboard/summary',
    handle(async (request, response) => {
      send(response, 200, await analyticsService(request).summary(caller(request), new Date()))
    }),
  )

  router.get(
    '/dashboard/networth',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      const months = reader.numberOr('months', 'months', DEFAULT_NET_WORTH_MONTHS)
      reader.done()
      send(
        response,
        200,
        await analyticsService(request).netWorth(caller(request), new Date(), months),
      )
    }),
  )

  router.get(
    '/dashboard/cashflow',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      const months = reader.numberOr('months', 'months', DEFAULT_CASHFLOW_MONTHS)
      reader.done()
      send(
        response,
        200,
        await analyticsService(request).cashflow(caller(request), new Date(), months),
      )
    }),
  )

  router.get(
    '/dashboard/spending',
    handle(async (request, response) => {
      const month = new QueryReader(request).text('month')
      send(
        response,
        200,
        await analyticsService(request).spending(caller(request), new Date(), month),
      )
    }),
  )

  router.get(
    '/reports/monthly',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      const year = reader.numberOr('year', 'year', new Date().getUTCFullYear())
      reader.done()
      send(response, 200, await analyticsService(request).monthlyReport(caller(request), year))
    }),
  )

  router.get(
    '/reports/categories',
    handle(async (request, response) => {
      const reader = new QueryReader(request)
      const dateFrom = reader.moment('from', 'from')
      const dateTo = reader.moment('to', 'to')
      reader.done()
      send(
        response,
        200,
        await analyticsService(request).categoryReport(caller(request), dateFrom, dateTo),
      )
    }),
  )

  return router
}
