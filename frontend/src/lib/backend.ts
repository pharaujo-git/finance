/** The API implementations the SPA can talk to. */
export type Backend = 'dotnet' | 'go' | 'python' | 'node' | 'rails'

export const BACKEND_KEY = 'ft.backend'

// Node is the default because it is the implementation that is actually
// deployed: it runs on Vercel beside this app. The other four are containers
// and need a Docker host (see render.yaml), so a visitor who has not chosen
// would otherwise land on a backend that is not answering.
const DEFAULT_BACKEND: Backend = 'node'

export const BACKEND_LABELS: Record<Backend, string> = {
  dotnet: '.NET',
  go: 'Go',
  python: 'Python',
  node: 'Node',
  rails: 'Rails',
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
    value === 'dotnet' ||
    value === 'go' ||
    value === 'python' ||
    value === 'node' ||
    value === 'rails'
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

/**
 * The configured origin, or the local dev port it defaults to.
 *
 * The localhost default is a *development* convenience. In a deployed bundle
 * it is a trap: the origin is compiled in at build time, so a build made
 * before the variable existed points the browser at a port on the visitor's
 * own machine, and every call dies as "Failed to fetch" with nothing reaching
 * the server. A build made that way is rejected by assertApiOrigins in
 * vite.config.ts; this says so out loud if one ever gets past it.
 */
function originOf(name: string, devPort: string): string {
  const configured = envBase(name)
  if (configured) return configured

  if (import.meta.env.PROD) {
    const label = BACKEND_LABELS_BY_VAR[name] ?? name
    throw new Error(
      `${name} was not set when this app was built, so the ${label} backend ` +
        'has no address to call. Rebuild with it set.',
    )
  }
  return `http://localhost:${devPort}`
}

/** Only used to name the backend in the error above. */
const BACKEND_LABELS_BY_VAR: Record<string, string> = {
  VITE_API_URL_DOTNET: BACKEND_LABELS.dotnet,
  VITE_API_URL_GO: BACKEND_LABELS.go,
  VITE_API_URL_PYTHON: BACKEND_LABELS.python,
  VITE_API_URL_NODE: BACKEND_LABELS.node,
  VITE_API_URL_RAILS: BACKEND_LABELS.rails,
}

/** Origin of the active backend — no trailing slash, no `/api` suffix. */
export function apiBase(): string {
  const backend = getBackend()
  if (backend === 'go') {
    return originOf('VITE_API_URL_GO', '8081')
  }
  if (backend === 'python') {
    return originOf('VITE_API_URL_PYTHON', '8082')
  }
  if (backend === 'node') {
    return originOf('VITE_API_URL_NODE', '8083')
  }
  if (backend === 'rails') {
    return originOf('VITE_API_URL_RAILS', '8084')
  }
  // The legacy single-API variable still stands in for the .NET origin.
  return envBase('VITE_API_URL_DOTNET') ?? originOf('VITE_API_URL', '5000')
}
