import { cn } from '@/lib/cn'
import { Icon, type IconName } from './Icon'

export type StatTone = 'brand' | 'positive' | 'negative' | 'neutral'

const TONES: Record<StatTone, string> = {
  brand: 'bg-brand-50 text-brand-600 dark:bg-brand-900/40 dark:text-brand-300',
  positive:
    'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-300',
  negative: 'bg-rose-50 text-rose-600 dark:bg-rose-900/40 dark:text-rose-300',
  neutral:
    'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300',
}

export interface StatCardProps {
  label: string
  value: string
  hint?: string
  icon: IconName
  tone?: StatTone
}

export function StatCard({
  label,
  value,
  hint,
  icon,
  tone = 'brand',
}: StatCardProps) {
  return (
    <div className="card flex items-start gap-4 p-5">
      <span className={cn('rounded-lg p-2.5', TONES[tone])}>
        <Icon name={icon} className="size-5" />
      </span>
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500 dark:text-slate-400">
          {label}
        </p>
        <p className="mt-1 truncate text-xl font-semibold tracking-tight text-slate-900 dark:text-slate-50">
          {value}
        </p>
        {hint ? (
          <p className="mt-0.5 text-xs text-slate-400 dark:text-slate-500">
            {hint}
          </p>
        ) : null}
      </div>
    </div>
  )
}
