import { useCallback, useState } from 'react'
import { Outlet } from 'react-router-dom'
import { ColdStartBanner } from '@/components/ui/ColdStartBanner'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'

export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false)
  const closeNav = useCallback(() => setNavOpen(false), [])
  const openNav = useCallback(() => setNavOpen(true), [])

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950">
      <Sidebar open={navOpen} onNavigate={closeNav} />
      <div className="lg:pl-64">
        <Topbar onOpenNav={openNav} />
        <ColdStartBanner />
        <main className="mx-auto w-full max-w-7xl space-y-6 px-4 py-6 sm:px-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
