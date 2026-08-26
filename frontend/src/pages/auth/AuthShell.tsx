import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ColdStartBanner } from '@/components/ui/ColdStartBanner'
import { Icon } from '@/components/ui/Icon'

export function AuthShell({
  title,
  subtitle,
  footerPrompt,
  footerLinkLabel,
  footerLinkTo,
  children,
}: {
  title: string
  subtitle: string
  footerPrompt: string
  footerLinkLabel: string
  footerLinkTo: string
  children: ReactNode
}) {
  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950">
      <ColdStartBanner />
      <div className="flex min-h-screen items-center justify-center px-4 py-10">
        <div className="w-full max-w-sm">
          <div className="mb-6 flex flex-col items-center gap-2 text-center">
            <span className="grid size-11 place-items-center rounded-xl bg-brand-600 text-white">
              <Icon name="wallet" className="size-5" />
            </span>
            <h1 className="text-xl font-semibold tracking-tight text-slate-900 dark:text-slate-50">
              {title}
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {subtitle}
            </p>
          </div>

          <div className="card p-6">{children}</div>

          <p className="mt-5 text-center text-sm text-slate-500 dark:text-slate-400">
            {footerPrompt}{' '}
            <Link
              to={footerLinkTo}
              className="font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400"
            >
              {footerLinkLabel}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}

export function FormError({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <p
      role="alert"
      className="rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:bg-rose-950 dark:text-rose-300"
    >
      {message}
    </p>
  )
}
