import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Field'
import { useAuth } from '@/context/auth-context'
import { errorMessage } from '@/hooks/useToastAction'
import { AuthShell, FormError } from './AuthShell'

const registerSchema = z.object({
  name: z.string().min(1, 'Name is required').max(80, 'Name is too long'),
  email: z.string().min(1, 'Email is required').email('Enter a valid email'),
  password: z.string().min(8, 'Use at least 8 characters'),
})

type RegisterValues = z.infer<typeof registerSchema>

export function RegisterPage() {
  const { register: signUp } = useAuth()
  const navigate = useNavigate()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: '', email: '', password: '' },
  })

  const onSubmit = handleSubmit(async (values) => {
    setSubmitError(null)
    try {
      await signUp(values.email, values.password, values.name)
      navigate('/', { replace: true })
    } catch (error) {
      setSubmitError(errorMessage(error))
    }
  })

  return (
    <AuthShell
      title="Create your account"
      subtitle="Track accounts, budgets and goals in one place."
      footerPrompt="Already have an account?"
      footerLinkLabel="Sign in"
      footerLinkTo="/login"
    >
      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
        <FormError message={submitError} />
        <Input
          label="Name"
          autoComplete="name"
          placeholder="Alex Morgan"
          error={errors.name?.message}
          {...register('name')}
        />
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
          autoComplete="new-password"
          placeholder="At least 8 characters"
          error={errors.password?.message}
          {...register('password')}
        />
        <Button type="submit" loading={isSubmitting} className="mt-1 w-full">
          Create account
        </Button>
      </form>
    </AuthShell>
  )
}
