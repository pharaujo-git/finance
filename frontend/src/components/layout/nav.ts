import type { IconName } from '@/components/ui/Icon'

export interface NavItem {
  to: string
  label: string
  icon: IconName
}

export const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: 'dashboard' },
  { to: '/transactions', label: 'Transactions', icon: 'transactions' },
  { to: '/accounts', label: 'Accounts', icon: 'accounts' },
  { to: '/budgets', label: 'Budgets', icon: 'budgets' },
  { to: '/goals', label: 'Goals', icon: 'goals' },
  { to: '/reports', label: 'Reports', icon: 'reports' },
  { to: '/settings', label: 'Settings', icon: 'settings' },
]
