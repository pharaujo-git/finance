import { cn } from '@/lib/cn'

export interface TabItem<T extends string> {
  id: T
  label: string
}

export function Tabs<T extends string>({
  items,
  active,
  onChange,
  label = 'Sections',
}: {
  items: TabItem<T>[]
  active: T
  onChange: (id: T) => void
  /** Accessible name for the tab list. */
  label?: string
}) {
  return (
    <div
      role="tablist"
      aria-label={label}
      className="inline-flex rounded-lg border border-slate-200 bg-white p-1 dark:border-slate-800 dark:bg-slate-900"
    >
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          role="tab"
          aria-selected={active === item.id}
          onClick={() => onChange(item.id)}
          className={cn(
            'rounded-md px-3 py-1.5 text-sm font-medium transition',
            active === item.id
              ? 'bg-brand-600 text-white'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800',
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}
