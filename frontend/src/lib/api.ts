import { clearToken, getToken } from './token'

export const API_BASE =
  (import.meta.env.VITE_API_URL as string | undefined)?.replace(/\/+$/, '') ||
  'http://localhost:5000'

/** Requests slower than this are assumed to be hitting a cold-starting server. */
export const COLD_START_MS = 2500

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

type Listener = (pending: boolean) => void

const coldStartListeners = new Set<Listener>()
let inflight = 0
let coldTimer: ReturnType<typeof setTimeout> | null = null

function emitColdStart(pending: boolean): void {
  for (const listener of coldStartListeners) listener(pending)
}

/** Subscribe to the "server is waking up" signal. Returns an unsubscribe fn. */
export function onColdStart(listener: Listener): () => void {
  coldStartListeners.add(listener)
  return () => {
    coldStartListeners.delete(listener)
  }
}

function trackStart(): void {
  inflight += 1
  if (coldTimer === null) {
    coldTimer = setTimeout(() => emitColdStart(true), COLD_START_MS)
  }
}

function trackEnd(): void {
  inflight = Math.max(0, inflight - 1)
  if (inflight === 0) {
    if (coldTimer !== null) {
      clearTimeout(coldTimer)
      coldTimer = null
    }
    emitColdStart(false)
  }
}

/** Called when a request comes back 401 so the app can bounce to /login. */
let unauthorizedHandler: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
}

function handleUnauthorized(): void {
  clearToken()
  if (unauthorizedHandler) {
    unauthorizedHandler()
  } else if (typeof window !== 'undefined') {
    window.location.assign('/login')
  }
}

async function readError(response: Response): Promise<string> {
  try {
    const text = await response.text()
    if (!text) return response.statusText || 'Request failed'
    try {
      const parsed: unknown = JSON.parse(text)
      if (parsed && typeof parsed === 'object') {
        const record = parsed as Record<string, unknown>
        const message = record.message ?? record.title ?? record.error
        if (typeof message === 'string' && message.length > 0) return message
      }
      return text
    } catch {
      return text
    }
  } catch {
    return response.statusText || 'Request failed'
  }
}

export interface RequestOptions {
  method?: string
  body?: unknown
  /** Send a raw body (e.g. FormData) instead of JSON. */
  formData?: FormData
  signal?: AbortSignal
}

async function send(path: string, options: RequestOptions): Promise<Response> {
  const headers = new Headers()
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let body: BodyInit | undefined
  if (options.formData) {
    body = options.formData
  } else if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(options.body)
  }

  trackStart()
  try {
    const response = await fetch(`${API_BASE}/api${path}`, {
      method: options.method ?? 'GET',
      headers,
      body,
      signal: options.signal,
    })

    if (response.status === 401) {
      handleUnauthorized()
      throw new ApiError(401, 'Your session expired. Please sign in again.')
    }
    if (!response.ok) {
      throw new ApiError(response.status, await readError(response))
    }
    return response
  } finally {
    trackEnd()
  }
}

/** JSON request. Returns `undefined` for 204 responses. */
export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const response = await send(path, options)
  if (response.status === 204) return undefined as T
  const text = await response.text()
  return (text ? JSON.parse(text) : undefined) as T
}

export async function apiBlob(path: string): Promise<Blob> {
  const response = await send(path, {})
  return response.blob()
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) =>
    apiRequest<T>(path, { signal }),
  post: <T>(path: string, body?: unknown) =>
    apiRequest<T>(path, { method: 'POST', body }),
  put: <T>(path: string, body?: unknown) =>
    apiRequest<T>(path, { method: 'PUT', body }),
  del: <T>(path: string) => apiRequest<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, formData: FormData) =>
    apiRequest<T>(path, { method: 'POST', formData }),
  blob: apiBlob,
}

/** Build a `?a=b` query string, dropping empty values. */
export function toQuery(params: Record<string, unknown>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}
