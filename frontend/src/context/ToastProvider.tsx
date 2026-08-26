import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { ToastContext, type Toast, type ToastVariant } from './toast-context'
import { ToastViewport } from '@/components/ui/ToastViewport'

export const TOAST_TIMEOUT_MS = 4000

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const nextId = useRef(1)

  const dismiss = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id))
  }, [])

  const push = useCallback(
    (message: string, variant: ToastVariant = 'success') => {
      const id = nextId.current++
      setToasts((current) => [...current, { id, message, variant }])
      setTimeout(() => dismiss(id), TOAST_TIMEOUT_MS)
    },
    [dismiss],
  )

  const value = useMemo(
    () => ({ toasts, push, dismiss }),
    [toasts, push, dismiss],
  )

  return (
    <ToastContext value={value}>
      {children}
      <ToastViewport toasts={toasts} onDismiss={dismiss} />
    </ToastContext>
  )
}
