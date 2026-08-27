/**
 * Domain errors and the two problem shapes the API renders.
 *
 * Both were captured from the .NET API and reproduced by the Go and Python
 * ones; the frontend's readError() parses them, so the field names, the status
 * titles and even the content types are fixed.
 */

/** RFC 9457, and what the .NET API writes for handled exceptions. */
export const PROBLEM_CONTENT_TYPE = 'application/problem+json'

/** MVC content-negotiates plain JSON, not problem+json, for validation 400s. */
export const VALIDATION_CONTENT_TYPE = 'application/json; charset=utf-8'

export const VALIDATION_PROBLEM_TYPE = 'https://tools.ietf.org/html/rfc9110#section-15.5.1'
export const VALIDATION_PROBLEM_TITLE = 'One or more validation errors occurred.'

export type ErrorKind = 'validation' | 'unauthorized' | 'notFound' | 'conflict'

/** Mirrors ApiExceptionHandler.Responses in the .NET API. */
const STATUS_TITLES: Record<ErrorKind, { status: number; title: string }> = {
  validation: { status: 400, title: 'Bad Request' },
  unauthorized: { status: 401, title: 'Unauthorized' },
  notFound: { status: 404, title: 'Not Found' },
  conflict: { status: 409, title: 'Conflict' },
}

/** A domain failure carrying the kind that decides its HTTP status. */
export class AppError extends Error {
  readonly kind: ErrorKind

  constructor(kind: ErrorKind, message: string) {
    super(message)
    this.name = 'AppError'
    this.kind = kind
  }

  get status(): number {
    return STATUS_TITLES[this.kind].status
  }

  get title(): string {
    return STATUS_TITLES[this.kind].title
  }
}

export function validationError(message: string): AppError {
  return new AppError('validation', message)
}

export function unauthorized(message: string): AppError {
  return new AppError('unauthorized', message)
}

/** Takes the entity name, not the sentence: "Account" -> "Account was not found.". */
export function notFound(entity: string): AppError {
  return new AppError('notFound', `${entity} was not found.`)
}

export function conflict(message: string): AppError {
  return new AppError('conflict', message)
}

/** Field-keyed errors, rendered as MVC's validation dictionary. */
export class FieldErrors extends Error {
  readonly errors: Record<string, string[]>

  constructor(errors: Record<string, string[]> = {}) {
    super(VALIDATION_PROBLEM_TITLE)
    this.name = 'FieldErrors'
    this.errors = errors
  }

  add(field: string, message: string): void {
    ;(this.errors[field] ??= []).push(message)
  }

  get isEmpty(): boolean {
    return Object.keys(this.errors).length === 0
  }

  /** Throws itself when anything was collected. */
  raiseIfAny(): void {
    if (!this.isEmpty) throw this
  }
}

export interface ProblemBody {
  title: string
  status: number
  detail: string
  instance?: string
}

/**
 * An RFC 9457 problem document. `type` is absent because the .NET handler never
 * sets it; the human-readable text lands in `detail` with `title` holding the
 * status phrase.
 */
export function problemBody(
  status: number,
  title: string,
  detail: string,
  instance?: string,
): ProblemBody {
  const body: ProblemBody = { title, status, detail }
  if (instance) body.instance = instance
  return body
}

export interface ValidationBody {
  type: string
  title: string
  status: number
  errors: Record<string, string[]>
}

function byCodeUnit(left: string, right: string): number {
  if (left < right) return -1
  return left > right ? 1 : 0
}

/**
 * The field-error dictionary MVC writes for a 400. Keys are sorted because Go
 * marshals a map with sorted keys, and the bodies are compared byte for byte.
 */
export function validationBody(errors: Record<string, string[]>): ValidationBody {
  const sorted: Record<string, string[]> = {}
  // Compared by code unit, not with localeCompare: Go sorts its map keys
  // byte-wise, and a locale-aware collation would order "$" against "Email"
  // differently, breaking the byte-for-byte match with the other backends.
  for (const key of Object.keys(errors).sort(byCodeUnit)) {
    sorted[key] = errors[key] ?? []
  }
  return {
    type: VALIDATION_PROBLEM_TYPE,
    title: VALIDATION_PROBLEM_TITLE,
    status: 400,
    errors: sorted,
  }
}
