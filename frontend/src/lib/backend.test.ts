import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  apiBase,
  BACKEND_KEY,
  BACKEND_LABELS,
  getBackend,
  setBackend,
} from './backend'

afterEach(() => {
  vi.unstubAllEnvs()
})

describe('backend selection', () => {
  it('defaults to Node, the deployed backend', () => {
    expect(getBackend()).toBe('node')
  })

  it('round-trips the choice', () => {
    setBackend('go')
    expect(getBackend()).toBe('go')

    setBackend('dotnet')
    expect(getBackend()).toBe('dotnet')
  })

  it('keeps the choice on the tab so other tabs are not repointed', () => {
    setBackend('go')
    expect(window.sessionStorage.getItem(BACKEND_KEY)).toBe('go')
  })

  it('remembers the choice for tabs opened later', () => {
    setBackend('go')
    expect(window.localStorage.getItem(BACKEND_KEY)).toBe('go')
  })

  it('inherits the remembered choice when the tab has not picked one', () => {
    window.localStorage.setItem(BACKEND_KEY, 'go')
    expect(getBackend()).toBe('go')
  })

  it("prefers this tab's choice over the remembered one", () => {
    window.localStorage.setItem(BACKEND_KEY, 'go')
    window.sessionStorage.setItem(BACKEND_KEY, 'dotnet')
    expect(getBackend()).toBe('dotnet')
  })

  it('falls back to the default for an unknown stored value', () => {
    window.localStorage.setItem(BACKEND_KEY, 'rust')
    window.sessionStorage.setItem(BACKEND_KEY, 'rust')
    expect(getBackend()).toBe('node')
  })

  it('labels every backend', () => {
    expect(BACKEND_LABELS).toEqual({
      dotnet: '.NET',
      go: 'Go',
      python: 'Python',
      node: 'Node',
      rails: 'Rails',
    })
  })
})

describe('apiBase', () => {
  it('uses the built-in defaults when nothing is configured', () => {
    vi.stubEnv('VITE_API_URL', '')
    vi.stubEnv('VITE_API_URL_DOTNET', '')
    vi.stubEnv('VITE_API_URL_GO', '')

    vi.stubEnv('VITE_API_URL_PYTHON', '')
    vi.stubEnv('VITE_API_URL_NODE', '')
    vi.stubEnv('VITE_API_URL_RAILS', '')

    setBackend('dotnet')
    expect(apiBase()).toBe('http://localhost:5000')
    setBackend('go')
    expect(apiBase()).toBe('http://localhost:8081')
    setBackend('python')
    expect(apiBase()).toBe('http://localhost:8082')
    setBackend('node')
    expect(apiBase()).toBe('http://localhost:8083')
    setBackend('rails')
    expect(apiBase()).toBe('http://localhost:8084')
  })

  it('uses VITE_API_URL_NODE for the Node backend', () => {
    vi.stubEnv('VITE_API_URL_NODE', 'https://node.example.com')
    setBackend('node')
    expect(apiBase()).toBe('https://node.example.com')
  })

  it('uses VITE_API_URL_RAILS for the Rails backend', () => {
    vi.stubEnv('VITE_API_URL_RAILS', 'https://rails.example.com')
    setBackend('rails')
    expect(apiBase()).toBe('https://rails.example.com')
  })

  it('uses VITE_API_URL_PYTHON for the Python backend', () => {
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    vi.stubEnv('VITE_API_URL_PYTHON', 'https://python.example.com')
    setBackend('python')
    expect(apiBase()).toBe('https://python.example.com')
  })

  it('prefers VITE_API_URL_DOTNET over VITE_API_URL', () => {
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    vi.stubEnv('VITE_API_URL_DOTNET', 'https://dotnet.example.com')
    setBackend('dotnet')
    expect(apiBase()).toBe('https://dotnet.example.com')
  })

  it('falls back to VITE_API_URL for the .NET backend', () => {
    // Stubbed explicitly: a developer's .env.local sets this, and the fallback
    // is only reachable when it is unset.
    vi.stubEnv('VITE_API_URL_DOTNET', '')
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    setBackend('dotnet')
    expect(apiBase()).toBe('https://legacy.example.com')
  })

  it('ignores VITE_API_URL for the Go backend', () => {
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    vi.stubEnv('VITE_API_URL_GO', 'https://go.example.com')
    setBackend('go')
    expect(apiBase()).toBe('https://go.example.com')
  })

  it('strips trailing slashes', () => {
    vi.stubEnv('VITE_API_URL_DOTNET', 'https://dotnet.example.com///')
    setBackend('dotnet')
    expect(apiBase()).toBe('https://dotnet.example.com')

    vi.stubEnv('VITE_API_URL_GO', 'https://go.example.com/')
    setBackend('go')
    expect(apiBase()).toBe('https://go.example.com')
  })

  it('treats an empty env value as unset', () => {
    vi.stubEnv('VITE_API_URL_DOTNET', '')
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    setBackend('dotnet')
    expect(apiBase()).toBe('https://legacy.example.com')
  })

  it('refuses the localhost default in a production bundle', () => {
    // A deployed bundle built before the variable existed used to point the
    // visitor's browser at their own machine, which fails as "Failed to fetch"
    // and never reaches the server. Say which variable is missing instead.
    vi.stubEnv('PROD', true)
    vi.stubEnv('VITE_API_URL_PYTHON', '')
    setBackend('python')
    expect(() => apiBase()).toThrow(/VITE_API_URL_PYTHON.*Python/s)
  })

  it('still uses the localhost defaults outside a production bundle', () => {
    vi.stubEnv('PROD', false)
    vi.stubEnv('VITE_API_URL_PYTHON', '')
    setBackend('python')
    expect(apiBase()).toBe('http://localhost:8082')
  })
})
