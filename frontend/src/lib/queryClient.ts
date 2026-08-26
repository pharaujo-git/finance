import { QueryClient } from '@tanstack/react-query'
import { ApiError } from './api'

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // A 401 already redirects to /login; retrying it just delays that.
        retry: (failureCount, error) =>
          error instanceof ApiError && error.status < 500
            ? false
            : failureCount < 2,
      },
      mutations: { retry: false },
    },
  })
}
