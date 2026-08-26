/** The API implementations the SPA can talk to. */
export type Backend = 'dotnet' | 'go'

export const BACKEND_KEY = 'ft.backend'

const DEFAULT_BACKEND: Backend = 'dotnet'

export const BACKEND_LABELS: Record<Backend, string> = {
  dotnet: '.NET',
  go: 'Go',
}

function safeStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function isBackend(value: unknown): value is Backend {
  return value === 'dotnet' || value === 'go'
}

/** The backend picked on this device. Falls back to .NET. */
export function getBackend(): Backend {
  const stored = safeStorage()?.getItem(BACKEND_KEY)
  return isBackend(stored) ? stored : DEFAULT_BACKEND
}

export function setBackend(backend: Backend): void {
  safeStorage()?.setItem(BACKEND_KEY, backend)
}

/** Read an env origin, trimming trailing slashes and treating '' as unset. */
function envBase(name: string): string | undefined {
  const value = import.meta.env[name] as string | undefined
  return value?.replace(/\/+$/, '') || undefined
}

/** Origin of the active backend — no trailing slash, no `/api` suffix. */
export function apiBase(): string {
  if (getBackend() === 'go') {
    return envBase('VITE_API_URL_GO') ?? 'http://localhost:8081'
  }
  return (
    envBase('VITE_API_URL_DOTNET') ??
    envBase('VITE_API_URL') ??
    'http://localhost:5000'
  )
}
