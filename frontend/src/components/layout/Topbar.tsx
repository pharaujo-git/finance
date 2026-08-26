import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icon'
import { useAuth } from '@/context/auth-context'
import { useTheme } from '@/context/theme-context'

export function Topbar({ onOpenNav }: { onOpenNav: () => void }) {
  const { user, logout } = useAuth()
  const { theme, toggleTheme } = useTheme()

  return (
    <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-3 border-b border-slate-200 bg-white/85 px-4 backdrop-blur sm:px-6 dark:border-slate-800 dark:bg-slate-900/85">
      <button
        type="button"
        onClick={onOpenNav}
        aria-label="Open navigation"
        className="rounded-lg p-2 text-slate-600 transition hover:bg-slate-100 lg:hidden dark:text-slate-300 dark:hover:bg-slate-800"
      >
        <Icon name="menu" className="size-5" />
      </button>

      <div className="ml-auto flex items-center gap-2">
        <button
          type="button"
          onClick={toggleTheme}
          aria-label={
            theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'
          }
          className="rounded-lg p-2 text-slate-600 transition hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <Icon name={theme === 'dark' ? 'sun' : 'moon'} className="size-5" />
        </button>

        <div className="hidden text-right sm:block">
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
            {user?.name ?? 'Account'}
          </p>
          <p className="text-xs text-slate-500 dark:text-slate-400">
            {user?.email}
          </p>
        </div>

        <Button
          variant="ghost"
          size="sm"
          onClick={logout}
          icon={<Icon name="logout" className="size-4" />}
        >
          <span className="hidden sm:inline">Log out</span>
        </Button>
      </div>
    </header>
  )
}
