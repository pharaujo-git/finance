import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  api,
  ApiError,
  apiRequest,
  onColdStart,
  setUnauthorizedHandler,
  toQuery,
} from './api'
import { setBackend } from './backend'
import { clearToken, getToken, setToken } from './token'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockFetch(response: Response) {
  const spy = vi.fn().mockResolvedValue(response)
  vi.stubGlobal('fetch', spy)
  return spy
}

function lastInit(spy: ReturnType<typeof vi.fn>): RequestInit {
  return spy.mock.calls[0][1] as RequestInit
}

describe('toQuery', () => {
  it('builds a query string and drops empty values', () => {
    expect(toQuery({ page: 2, search: 'coffee' })).toBe('?page=2&search=coffee')
    expect(toQuery({ a: '', b: null, c: undefined })).toBe('')
    expect(toQuery({})).toBe('')
  })
})

describe('apiRequest', () => {
  beforeEach(() => {
    clearToken()
    setUnauthorizedHandler(null)
  })

  afterEach(() => {
    setUnauthorizedHandler(null)
    vi.unstubAllEnvs()
  })

  it('prefixes /api and parses JSON', async () => {
    const spy = mockFetch(jsonResponse({ ok: true }))
    const result = await apiRequest<{ ok: boolean }>('/accounts')

    expect(result).toEqual({ ok: true })
    expect(spy.mock.calls[0][0]).toMatch(/\/api\/accounts$/)
  })

  it('targets the origin of the selected backend', async () => {
    vi.stubEnv('VITE_API_URL_DOTNET', 'https://dotnet.example.com')
    vi.stubEnv('VITE_API_URL_GO', 'https://go.example.com')

    const dotnetSpy = mockFetch(jsonResponse([]))
    await api.get('/accounts')
    expect(dotnetSpy.mock.calls[0][0]).toBe(
      'https://dotnet.example.com/api/accounts',
    )

    setBackend('go')
    const goSpy = mockFetch(jsonResponse([]))
    await api.get('/accounts')
    expect(goSpy.mock.calls[0][0]).toBe('https://go.example.com/api/accounts')
  })

  it('omits the Authorization header when there is no token', async () => {
    const spy = mockFetch(jsonResponse([]))
    await api.get('/accounts')

    const headers = lastInit(spy).headers as Headers
    expect(headers.has('Authorization')).toBe(false)
  })

  it('adds a bearer token when one is stored', async () => {
    setToken('abc123')
    const spy = mockFetch(jsonResponse([]))
    await api.get('/accounts')

    const headers = lastInit(spy).headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer abc123')
  })

  it('serialises JSON bodies and sets the content type', async () => {
    const spy = mockFetch(jsonResponse({ id: '1' }))
    await api.post('/accounts', { name: 'Wallet' })

    const init = lastInit(spy)
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ name: 'Wallet' }))
    expect((init.headers as Headers).get('Content-Type')).toBe('application/json')
  })

  it('sends FormData without a JSON content type', async () => {
    const spy = mockFetch(jsonResponse({ imported: 2, skipped: 0 }))
    const form = new FormData()
    form.append('file', new File(['a,b'], 'tx.csv', { type: 'text/csv' }))

    await api.upload('/transactions/import', form)

    const init = lastInit(spy)
    expect(init.body).toBe(form)
    expect((init.headers as Headers).has('Content-Type')).toBe(false)
  })

  it('returns undefined for 204 responses', async () => {
    mockFetch(new Response(null, { status: 204 }))
    await expect(api.del('/accounts/1')).resolves.toBeUndefined()
  })

  it('clears the token and calls the handler on 401', async () => {
    setToken('expired')
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    mockFetch(new Response('', { status: 401 }))

    await expect(api.get('/accounts')).rejects.toBeInstanceOf(ApiError)
    expect(getToken()).toBeNull()
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('redirects to /login on 401 when no handler is registered', async () => {
    setToken('expired')
    const assign = vi.fn()
    vi.stubGlobal('location', { ...window.location, assign })
    mockFetch(new Response('', { status: 401 }))

    await expect(api.get('/accounts')).rejects.toThrow(/session expired/i)
    expect(assign).toHaveBeenCalledWith('/login')
  })

  it('surfaces the API message from an error body', async () => {
    mockFetch(jsonResponse({ message: 'Name already used' }, 400))
    await expect(api.post('/accounts', {})).rejects.toThrow('Name already used')
  })

  it('falls back to plain-text error bodies', async () => {
    mockFetch(new Response('boom', { status: 500 }))
    await expect(api.get('/accounts')).rejects.toThrow('boom')
  })

  it('exposes the HTTP status on the error', async () => {
    mockFetch(jsonResponse({ title: 'Not found' }, 404))
    await expect(api.get('/accounts/9')).rejects.toMatchObject({ status: 404 })
  })

  it('returns a blob for downloads', async () => {
    mockFetch(new Response('date,amount', { status: 200 }))
    const blob = await api.blob('/transactions/export')
    expect(await blob.text()).toBe('date,amount')
  })
})

describe('cold-start signal', () => {
  it('notifies listeners when requests settle and unsubscribes cleanly', async () => {
    const listener = vi.fn()
    const unsubscribe = onColdStart(listener)
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(jsonResponse([]))),
    )

    await api.get('/accounts')
    expect(listener).toHaveBeenCalledWith(false)

    listener.mockClear()
    unsubscribe()
    await api.get('/accounts')
    expect(listener).not.toHaveBeenCalled()
  })
})
