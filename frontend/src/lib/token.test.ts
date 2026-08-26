import { describe, expect, it } from 'vitest'
import { clearToken, getToken, setToken } from './token'

describe('token storage', () => {
  it('round-trips a token', () => {
    expect(getToken()).toBeNull()
    setToken('abc')
    expect(getToken()).toBe('abc')
    clearToken()
    expect(getToken()).toBeNull()
  })
})
