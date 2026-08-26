import { cn } from '@/lib/cn'

/** Compact `<input type="month">` used by the budgets and dashboard headers. */
export function MonthPicker({
  value,
  onChange,
  label = 'Month',
  className,
}: {
  value: string
  onChange: (month: string) => void
  label?: string
  className?: string
}) {
  return (
    <input
      type="month"
      aria-label={label}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={cn('field-control w-auto', className)}
    />
  )
}
