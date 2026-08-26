import { useCallback } from 'react'
import { useToast } from '@/context/toast-context'

export function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  return 'Something went wrong. Please try again.'
}

/**
 * Runs a mutation and reports the outcome as a toast. Returns the result, or
 * `undefined` when the call failed, so callers can guard follow-up work.
 */
export function useToastAction() {
  const { push } = useToast()

  return useCallback(
    async <T>(
      run: () => Promise<T>,
      successMessage: string | ((result: T) => string),
    ) => {
      try {
        const result = await run()
        push(
          typeof successMessage === 'function'
            ? successMessage(result)
            : successMessage,
          'success',
        )
        return result
      } catch (error) {
        push(errorMessage(error), 'error')
        return undefined
      }
    },
    [push],
  )
}
