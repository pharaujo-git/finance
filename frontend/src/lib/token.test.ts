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

  it('keeps a copy on the tab and one for tabs opened later', () => {
    setToken('abc')
    expect(window.sessionStorage.getItem('ft.token')).toBe('abc')
    expect(window.localStorage.getItem('ft.token')).toBe('abc')
  })

  it('adopts the device token so a fresh tab stays signed in', () => {
    window.localStorage.setItem('ft.token', 'from-another-tab')
    expect(getToken()).toBe('from-another-tab')
    expect(window.sessionStorage.getItem('ft.token')).toBe('from-another-tab')
  })

  it('prefers the tab token over the device one', () => {
    window.localStorage.setItem('ft.token', 'device')
    window.sessionStorage.setItem('ft.token', 'tab')
    expect(getToken()).toBe('tab')
  })

  it('signing out does not strip a token another tab already holds', () => {
    // The other tab's copy lives in its own sessionStorage, which this one
    // cannot reach; clearing here must not be able to touch it.
    setToken('abc')
    clearToken()
    expect(window.localStorage.getItem('ft.token')).toBeNull()
    expect(window.sessionStorage.getItem('ft.token')).toBeNull()
  })
})
