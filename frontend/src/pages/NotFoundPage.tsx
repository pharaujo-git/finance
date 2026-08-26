import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="card flex flex-col items-center gap-3 px-6 py-16 text-center">
      <p className="text-4xl font-semibold text-slate-300 dark:text-slate-700">
        404
      </p>
      <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
        Page not found
      </h1>
      <p className="max-w-sm text-sm text-slate-500 dark:text-slate-400">
        The page you were looking for does not exist or has moved.
      </p>
      <Link
        to="/"
        className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-brand-700"
      >
        Back to dashboard
      </Link>
    </div>
  )
}
