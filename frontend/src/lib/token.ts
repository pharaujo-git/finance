const TOKEN_KEY = 'ft.token'

function safeStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function getToken(): string | null {
  return safeStorage()?.getItem(TOKEN_KEY) ?? null
}

export function setToken(token: string): void {
  safeStorage()?.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  safeStorage()?.removeItem(TOKEN_KEY)
}
