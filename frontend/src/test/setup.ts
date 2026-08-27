import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// jsdom ships neither of these and Recharts / the theme provider need them.
globalThis.ResizeObserver ??= ResizeObserverStub as never

if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as never
}

beforeEach(() => {
  window.localStorage.clear()
  // The backend choice is per-tab, so this has to be reset between tests too.
  window.sessionStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
