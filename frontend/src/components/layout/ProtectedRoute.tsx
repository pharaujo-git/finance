import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { Spinner } from '@/components/ui/Spinner'
import { useAuth } from '@/context/auth-context'

export function FullPageSpinner({ label = 'Loading' }: { label?: string }) {
  return (
    <div
      role="status"
      aria-label={label}
      className="grid min-h-screen place-items-center bg-slate-50 dark:bg-slate-950"
    >
      <Spinner className="size-8 text-brand-600" />
    </div>
  )
}

/** Gate for every authenticated route; bounces to /login with a return path. */
export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) return <FullPageSpinner label="Checking your session" />
  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <Outlet />
}

/** Inverse gate: keeps signed-in users off the auth screens. */
export function PublicOnlyRoute() {
  const { isAuthenticated, isLoading } = useAuth()
  if (isLoading) return <FullPageSpinner label="Checking your session" />
  if (isAuthenticated) return <Navigate to="/" replace />
  return <Outlet />
}
