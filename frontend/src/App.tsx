import { lazy, Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AppLayout } from '@/components/layout/AppLayout'
import {
  FullPageSpinner,
  ProtectedRoute,
  PublicOnlyRoute,
} from '@/components/layout/ProtectedRoute'
import { AuthProvider } from '@/context/AuthProvider'
import { ToastProvider } from '@/context/ToastProvider'
import { LoginPage } from '@/pages/auth/LoginPage'
import { RegisterPage } from '@/pages/auth/RegisterPage'
import { NotFoundPage } from '@/pages/NotFoundPage'

// The authenticated pages pull in the charting library, so they are split out
// of the entry bundle and loaded on demand.
const DashboardPage = lazy(() =>
  import('@/pages/DashboardPage').then((m) => ({ default: m.DashboardPage })),
)
const TransactionsPage = lazy(() =>
  import('@/pages/TransactionsPage').then((m) => ({
    default: m.TransactionsPage,
  })),
)
const AccountsPage = lazy(() =>
  import('@/pages/AccountsPage').then((m) => ({ default: m.AccountsPage })),
)
const BudgetsPage = lazy(() =>
  import('@/pages/BudgetsPage').then((m) => ({ default: m.BudgetsPage })),
)
const GoalsPage = lazy(() =>
  import('@/pages/GoalsPage').then((m) => ({ default: m.GoalsPage })),
)
const ReportsPage = lazy(() =>
  import('@/pages/ReportsPage').then((m) => ({ default: m.ReportsPage })),
)
const SettingsPage = lazy(() =>
  import('@/pages/SettingsPage').then((m) => ({ default: m.SettingsPage })),
)

export function AppRoutes() {
  return (
    <Suspense fallback={<FullPageSpinner label="Loading page" />}>
      <Routes>
        <Route element={<PublicOnlyRoute />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>

        <Route element={<ProtectedRoute />}>
          <Route element={<AppLayout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/transactions" element={<TransactionsPage />} />
            <Route path="/accounts" element={<AccountsPage />} />
            <Route path="/budgets" element={<BudgetsPage />} />
            <Route path="/goals" element={<GoalsPage />} />
            <Route path="/reports" element={<ReportsPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Route>
      </Routes>
    </Suspense>
  )
}

/** Providers that must sit inside the router (they use navigation). */
export function App() {
  return (
    <AuthProvider>
      <ToastProvider>
        <AppRoutes />
      </ToastProvider>
    </AuthProvider>
  )
}
