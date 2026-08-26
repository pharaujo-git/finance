import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { auth as authApi } from '@/api/endpoints'
import { setUnauthorizedHandler } from '@/lib/api'
import { clearToken, getToken, setToken } from '@/lib/token'
import type { User } from '@/types'
import { AuthContext } from './auth-context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [user, setUserState] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(() => getToken() !== null)

  const logout = useCallback(() => {
    clearToken()
    setUserState(null)
    queryClient.clear()
    navigate('/login', { replace: true })
  }, [navigate, queryClient])

  // Route 401s from anywhere in the app back to the login screen.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      setUserState(null)
      queryClient.clear()
      navigate('/login', { replace: true })
    })
    return () => setUnauthorizedHandler(null)
  }, [navigate, queryClient])

  // Restore the session from a stored token on first mount. When there is no
  // token `isLoading` already starts false, so there is nothing to do.
  useEffect(() => {
    if (getToken() === null) return
    let cancelled = false
    authApi
      .me()
      .then((me) => {
        if (!cancelled) setUserState(me)
      })
      .catch(() => {
        if (!cancelled) setUserState(null)
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const result = await authApi.login({ email, password })
    setToken(result.token)
    setUserState(result.user)
  }, [])

  const register = useCallback(
    async (email: string, password: string, name: string) => {
      const result = await authApi.register({ email, password, name })
      setToken(result.token)
      setUserState(result.user)
    },
    [],
  )

  const value = useMemo(
    () => ({
      user,
      isLoading,
      isAuthenticated: user !== null,
      login,
      register,
      logout,
      setUser: setUserState,
      currency: user?.currency ?? 'USD',
    }),
    [user, isLoading, login, register, logout],
  )

  return <AuthContext value={value}>{children}</AuthContext>
}
