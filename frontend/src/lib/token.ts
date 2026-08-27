const TOKEN_KEY = 'ft.token'

/**
 * Per-tab storage. The session lives here so signing out of one tab does not
 * sign out the others: a tab has to sign out to switch backends, and doing so
 * used to drop the shared token out from under every other tab.
 */
function tabStorage(): Storage | null {
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

/** Shared by every tab, so a new tab (or a restarted browser) stays signed in. */
function deviceStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

/**
 * This tab's token. A tab that has none adopts the device's, which is what
 * keeps a fresh tab signed in; from then on the tab holds its own copy.
 */
export function getToken(): string | null {
  const own = tabStorage()?.getItem(TOKEN_KEY)
  if (own) return own

  const shared = deviceStorage()?.getItem(TOKEN_KEY)
  if (shared) tabStorage()?.setItem(TOKEN_KEY, shared)
  return shared ?? null
}

export function setToken(token: string): void {
  tabStorage()?.setItem(TOKEN_KEY, token)
  deviceStorage()?.setItem(TOKEN_KEY, token)
}

/**
 * Signs this tab out. The device copy goes too, so a new tab does not revive
 * the session, but other open tabs keep the copy they already adopted.
 */
export function clearToken(): void {
  tabStorage()?.removeItem(TOKEN_KEY)
  deviceStorage()?.removeItem(TOKEN_KEY)
}
