import { createContext, useContext } from 'react'
import type { User } from '@/types'

export interface AuthContextValue {
  user: User | null
  /** True until the initial `/auth/me` probe settles. */
  isLoading: boolean
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => void
  setUser: (user: User) => void
  /** Currency to format money with; falls back to USD before the user loads. */
  currency: string
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}

/** Convenience accessor used by every money-formatting call site. */
export function useCurrency(): string {
  return useAuth().currency
}
