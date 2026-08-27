/** The API implementations the SPA can talk to. */
export type Backend = 'dotnet' | 'go' | 'python' | 'node'

export const BACKEND_KEY = 'ft.backend'

const DEFAULT_BACKEND: Backend = 'dotnet'

export const BACKEND_LABELS: Record<Backend, string> = {
  dotnet: '.NET',
  go: 'Go',
  python: 'Python',
  node: 'Node',
}

/**
 * Per-tab storage. The choice lives here so two tabs can drive the two
 * backends side by side: localStorage is shared by every tab on the origin, so
 * keeping it there meant the last tab to pick a backend silently repointed all
 * the others.
 */
function tabStorage(): Storage | null {
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

/** Shared by every tab, and only read to seed a tab that has no choice yet. */
function deviceStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function isBackend(value: unknown): value is Backend {
  return (
    value === 'dotnet' || value === 'go' || value === 'python' || value === 'node'
  )
}

/**
 * The backend this tab talks to. A tab that has not picked one inherits the
 * last choice made on the device, so opening a new tab is not a surprise.
 */
export function getBackend(): Backend {
  const picked = tabStorage()?.getItem(BACKEND_KEY)
  if (isBackend(picked)) return picked

  const remembered = deviceStorage()?.getItem(BACKEND_KEY)
  return isBackend(remembered) ? remembered : DEFAULT_BACKEND
}

export function setBackend(backend: Backend): void {
  // The tab's own choice, plus the seed for tabs opened later.
  tabStorage()?.setItem(BACKEND_KEY, backend)
  deviceStorage()?.setItem(BACKEND_KEY, backend)
}

/** Read an env origin, trimming trailing slashes and treating '' as unset. */
function envBase(name: string): string | undefined {
  const value = import.meta.env[name] as string | undefined
  return value?.replace(/\/+$/, '') || undefined
}

/** Origin of the active backend — no trailing slash, no `/api` suffix. */
export function apiBase(): string {
  const backend = getBackend()
  if (backend === 'go') {
    return envBase('VITE_API_URL_GO') ?? 'http://localhost:8081'
  }
  if (backend === 'python') {
    return envBase('VITE_API_URL_PYTHON') ?? 'http://localhost:8082'
  }
  if (backend === 'node') {
    return envBase('VITE_API_URL_NODE') ?? 'http://localhost:8083'
  }
  return (
    envBase('VITE_API_URL_DOTNET') ??
    envBase('VITE_API_URL') ??
    'http://localhost:5000'
  )
}
