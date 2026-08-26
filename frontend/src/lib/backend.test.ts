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
  it('defaults to .NET', () => {
    expect(getBackend()).toBe('dotnet')
  })

  it('round-trips the choice through localStorage', () => {
    setBackend('go')
    expect(window.localStorage.getItem(BACKEND_KEY)).toBe('go')
    expect(getBackend()).toBe('go')

    setBackend('dotnet')
    expect(getBackend()).toBe('dotnet')
  })

  it('falls back to the default for an unknown stored value', () => {
    window.localStorage.setItem(BACKEND_KEY, 'rust')
    expect(getBackend()).toBe('dotnet')
  })

  it('labels both backends', () => {
    expect(BACKEND_LABELS).toEqual({ dotnet: '.NET', go: 'Go' })
  })
})

describe('apiBase', () => {
  it('uses the built-in defaults when nothing is configured', () => {
    vi.stubEnv('VITE_API_URL', '')
    vi.stubEnv('VITE_API_URL_DOTNET', '')
    vi.stubEnv('VITE_API_URL_GO', '')

    expect(apiBase()).toBe('http://localhost:5000')
    setBackend('go')
    expect(apiBase()).toBe('http://localhost:8081')
  })

  it('prefers VITE_API_URL_DOTNET over VITE_API_URL', () => {
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    vi.stubEnv('VITE_API_URL_DOTNET', 'https://dotnet.example.com')
    expect(apiBase()).toBe('https://dotnet.example.com')
  })

  it('falls back to VITE_API_URL for the .NET backend', () => {
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
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
    expect(apiBase()).toBe('https://dotnet.example.com')

    vi.stubEnv('VITE_API_URL_GO', 'https://go.example.com/')
    setBackend('go')
    expect(apiBase()).toBe('https://go.example.com')
  })

  it('treats an empty env value as unset', () => {
    vi.stubEnv('VITE_API_URL_DOTNET', '')
    vi.stubEnv('VITE_API_URL', 'https://legacy.example.com')
    expect(apiBase()).toBe('https://legacy.example.com')
  })
})
