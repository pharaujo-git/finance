import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Field'
import { Tabs, type TabItem } from '@/components/ui/Tabs'
import { useAuth } from '@/context/auth-context'
import { errorMessage } from '@/hooks/useToastAction'
import {
  BACKEND_LABELS,
  getBackend,
  setBackend,
  type Backend,
} from '@/lib/backend'
import { AuthShell, FormError } from './AuthShell'

const loginSchema = z.object({
  email: z.string().min(1, 'Email is required').email('Enter a valid email'),
  password: z.string().min(1, 'Password is required'),
})

type LoginValues = z.infer<typeof loginSchema>

// Demo credentials surfaced on the login page in dev builds only —
// import.meta.env.DEV is false in production, so the block is stripped there.
const DEMO_CREDENTIALS = import.meta.env.DEV
  ? { email: 'smoke@test.dev', password: 'Passw0rd!123' }
  : null

const BACKEND_TABS: TabItem<Backend>[] = [
  { id: 'dotnet', label: BACKEND_LABELS.dotnet },
  { id: 'go', label: BACKEND_LABELS.go },
]

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [backend, setActiveBackend] = useState<Backend>(getBackend)

  const chooseBackend = (next: Backend) => {
    setBackend(next)
    setActiveBackend(next)
  }

  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: '', password: '' },
  })

  const onSubmit = handleSubmit(async (values) => {
    setSubmitError(null)
    try {
      await login(values.email, values.password)
      navigate('/', { replace: true })
    } catch (error) {
      setSubmitError(errorMessage(error))
    }
  })

  return (
    <AuthShell
      title="Welcome back"
      subtitle="Sign in to pick up where you left off."
      footerPrompt="New here?"
      footerLinkLabel="Create an account"
      footerLinkTo="/register"
    >
      <div className="mb-5 flex flex-col items-start gap-2">
        <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
          API backend
        </span>
        <Tabs
          items={BACKEND_TABS}
          active={backend}
          onChange={chooseBackend}
          label="API backend"
        />
      </div>

      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
        <FormError message={submitError} />
        <Input
          label="Email"
          type="email"
          autoComplete="email"
          placeholder="you@example.com"
          error={errors.email?.message}
          {...register('email')}
        />
        <Input
          label="Password"
          type="password"
          autoComplete="current-password"
          placeholder="••••••••"
          error={errors.password?.message}
          {...register('password')}
        />
        <Button type="submit" loading={isSubmitting} className="mt-1 w-full">
          Sign in
        </Button>
        {DEMO_CREDENTIALS && (
          <button
            type="button"
            onClick={() => {
              setValue('email', DEMO_CREDENTIALS.email)
              setValue('password', DEMO_CREDENTIALS.password)
            }}
            className="rounded-lg border border-dashed border-emerald-700/60 px-3 py-2 text-left text-xs text-slate-400 transition hover:border-emerald-500 hover:text-slate-200"
          >
            <span className="font-semibold text-emerald-500">Dev environment</span> — click to fill
            the demo account: {DEMO_CREDENTIALS.email} / {DEMO_CREDENTIALS.password}
          </button>
        )}
      </form>
    </AuthShell>
  )
}
