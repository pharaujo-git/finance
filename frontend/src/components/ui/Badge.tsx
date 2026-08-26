import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

export type BadgeTone = 'neutral' | 'success' | 'danger' | 'info' | 'warning'

const TONES: Record<BadgeTone, string> = {
  neutral:
    'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  success:
    'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  danger: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
  info: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
  warning:
    'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
}

export function Badge({
  tone = 'neutral',
  className,
  children,
}: {
  tone?: BadgeTone
  className?: string
  children: ReactNode
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap',
        TONES[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

export function TagChips({ tags }: { tags: string[] }) {
  if (tags.length === 0) return null
  return (
    <span className="flex flex-wrap gap-1">
      {tags.map((tag) => (
        <Badge key={tag} tone="info">
          {tag}
        </Badge>
      ))}
    </span>
  )
}

export function ColorDot({ color }: { color: string }) {
  return (
    <span
      aria-hidden="true"
      className="inline-block size-2.5 shrink-0 rounded-full"
      style={{ backgroundColor: color || '#94a3b8' }}
    />
  )
}
