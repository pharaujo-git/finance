import { useMemo } from 'react'
import { useAccounts, useCategories } from '@/api/queries'
import type { Account, Category } from '@/types'

export interface Lookups {
  accounts: Account[]
  activeAccounts: Account[]
  categories: Category[]
  accountName: (id: string | null | undefined) => string
  category: (id: string | null | undefined) => Category | undefined
  categoryName: (id: string | null | undefined) => string
  isLoading: boolean
}

/** Account and category reference data, shared by most pages. */
export function useLookups(): Lookups {
  const accountsQuery = useAccounts()
  const categoriesQuery = useCategories()

  return useMemo(() => {
    const accounts = accountsQuery.data ?? []
    const categories = categoriesQuery.data ?? []
    const accountMap = new Map(accounts.map((item) => [item.id, item]))
    const categoryMap = new Map(categories.map((item) => [item.id, item]))

    return {
      accounts,
      activeAccounts: accounts.filter((item) => !item.isArchived),
      categories,
      accountName: (id) => (id ? (accountMap.get(id)?.name ?? '—') : '—'),
      category: (id) => (id ? categoryMap.get(id) : undefined),
      categoryName: (id) => (id ? (categoryMap.get(id)?.name ?? '—') : '—'),
      isLoading: accountsQuery.isLoading || categoriesQuery.isLoading,
    }
  }, [
    accountsQuery.data,
    accountsQuery.isLoading,
    categoriesQuery.data,
    categoriesQuery.isLoading,
  ])
}
