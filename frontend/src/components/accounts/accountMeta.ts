import type { IconName } from '@/components/ui/Icon'
import type { AccountType } from '@/types'

export const ACCOUNT_TYPES: AccountType[] = [
  'checking',
  'savings',
  'creditCard',
  'cash',
  'investment',
]

const META: Record<AccountType, { label: string; icon: IconName; tint: string }> = {
  checking: {
    label: 'Checking',
    icon: 'checking',
    tint: 'bg-sky-50 text-sky-600 dark:bg-sky-900/40 dark:text-sky-300',
  },
  savings: {
    label: 'Savings',
    icon: 'savings',
    tint: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-300',
  },
  creditCard: {
    label: 'Credit card',
    icon: 'creditCard',
    tint: 'bg-violet-50 text-violet-600 dark:bg-violet-900/40 dark:text-violet-300',
  },
  cash: {
    label: 'Cash',
    icon: 'cash',
    tint: 'bg-amber-50 text-amber-600 dark:bg-amber-900/40 dark:text-amber-300',
  },
  investment: {
    label: 'Investment',
    icon: 'investment',
    tint: 'bg-indigo-50 text-indigo-600 dark:bg-indigo-900/40 dark:text-indigo-300',
  },
}

export function accountMeta(type: AccountType) {
  return META[type] ?? META.checking
}
