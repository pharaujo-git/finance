import { cn } from '@/lib/cn'
import type { Toast, ToastVariant } from '@/context/toast-context'
import { Icon, type IconName } from './Icon'

const STYLES: Record<ToastVariant, { classes: string; icon: IconName }> = {
  success: {
    classes:
      'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-100',
    icon: 'check',
  },
  error: {
    classes:
      'border-rose-200 bg-rose-50 text-rose-900 dark:border-rose-800 dark:bg-rose-950 dark:text-rose-100',
    icon: 'alert',
  },
  info: {
    classes:
      'border-sky-200 bg-sky-50 text-sky-900 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-100',
    icon: 'info',
  },
}

export function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: Toast[]
  onDismiss: (id: number) => void
}) {
  if (toasts.length === 0) return null
  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4 sm:inset-x-auto sm:right-4 sm:items-end"
    >
      {toasts.map((toast) => {
        const style = STYLES[toast.variant]
        return (
          <div
            key={toast.id}
            className={cn(
              'pointer-events-auto flex w-full max-w-sm items-start gap-2 rounded-xl border px-3.5 py-2.5 text-sm shadow-lg',
              style.classes,
            )}
          >
            <Icon name={style.icon} className="mt-0.5 size-4" />
            <span className="flex-1">{toast.message}</span>
            <button
              type="button"
              aria-label="Dismiss notification"
              onClick={() => onDismiss(toast.id)}
              className="opacity-60 transition hover:opacity-100"
            >
              <Icon name="close" className="size-4" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
