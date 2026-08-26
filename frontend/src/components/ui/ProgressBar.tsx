import { cn } from '@/lib/cn'
import { progressPercent } from '@/lib/format'

export interface ProgressBarProps {
  value: number
  max: number
  /** Colour the bar red once `value` exceeds `max`. */
  warnOnOverflow?: boolean
  color?: string
  label?: string
  className?: string
}

export function ProgressBar({
  value,
  max,
  warnOnOverflow = false,
  color,
  label,
  className,
}: ProgressBarProps) {
  const percent = progressPercent(value, max)
  const over = warnOnOverflow && max > 0 && value > max
  const style = over || !color ? undefined : { backgroundColor: color }

  return (
    <div
      role="progressbar"
      aria-valuenow={Math.round(percent)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
      className={cn(
        'h-2 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800',
        className,
      )}
    >
      <div
        data-testid="progress-fill"
        className={cn(
          'h-full rounded-full transition-[width] duration-500',
          over ? 'bg-rose-500' : !color && 'bg-brand-500',
        )}
        style={{ width: `${percent}%`, ...style }}
      />
    </div>
  )
}

export interface ProgressRingProps {
  value: number
  max: number
  color?: string
  size?: number
  label?: string
}

export function ProgressRing({
  value,
  max,
  color = 'var(--color-brand-500)',
  size = 88,
  label,
}: ProgressRingProps) {
  const percent = progressPercent(value, max)
  const radius = (size - 10) / 2
  const circumference = 2 * Math.PI * radius

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label ?? `${Math.round(percent)}% complete`}
      className="shrink-0"
    >
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        strokeWidth="8"
        className="stroke-slate-200 dark:stroke-slate-800"
      />
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        strokeWidth="8"
        strokeLinecap="round"
        stroke={color}
        strokeDasharray={circumference}
        strokeDashoffset={circumference * (1 - percent / 100)}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
      />
      <text
        x="50%"
        y="50%"
        dominantBaseline="central"
        textAnchor="middle"
        className="fill-slate-900 text-xs font-semibold dark:fill-slate-100"
      >
        {Math.round(percent)}%
      </text>
    </svg>
  )
}
