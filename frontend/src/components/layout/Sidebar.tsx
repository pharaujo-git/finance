import { NavLink } from 'react-router-dom'
import { Icon } from '@/components/ui/Icon'
import { cn } from '@/lib/cn'
import { NAV_ITEMS } from './nav'

export function Sidebar({
  open,
  onNavigate,
}: {
  open: boolean
  onNavigate: () => void
}) {
  return (
    <>
      {open ? (
        <button
          type="button"
          aria-label="Close navigation"
          onClick={onNavigate}
          className="fixed inset-0 z-30 cursor-default bg-slate-900/50 lg:hidden"
        />
      ) : null}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-200 bg-white transition-transform duration-200 lg:translate-x-0 dark:border-slate-800 dark:bg-slate-900',
          open ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-16 items-center gap-2 border-b border-slate-200 px-5 dark:border-slate-800">
          <span className="grid size-8 place-items-center rounded-lg bg-brand-600 text-white">
            <Icon name="wallet" className="size-4" />
          </span>
          <span className="text-sm font-semibold tracking-tight text-slate-900 dark:text-slate-50">
            Finance Tracker
          </span>
        </div>
        <nav aria-label="Main" className="flex-1 space-y-1 overflow-y-auto p-3">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              onClick={onNavigate}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition',
                  isActive
                    ? 'bg-brand-50 text-brand-700 dark:bg-brand-900/40 dark:text-brand-200'
                    : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200',
                )
              }
            >
              <Icon name={item.icon} className="size-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
    </>
  )
}
