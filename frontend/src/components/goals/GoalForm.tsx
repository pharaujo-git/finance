import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import { moneyString, requiredString, toNumber } from '@/lib/validation'
import type { Goal, GoalInput } from '@/types'

export const GOAL_COLORS = [
  '#16a34a',
  '#0ea5e9',
  '#8b5cf6',
  '#f59e0b',
  '#ec4899',
  '#14b8a6',
]

export const goalSchema = z.object({
  name: requiredString('Name is required').max(60, 'Name is too long'),
  targetAmount: moneyString,
  targetDate: z.string(),
  color: requiredString('Pick a colour'),
})

export type GoalFormValues = z.infer<typeof goalSchema>

export function toGoalInput(values: GoalFormValues): GoalInput {
  return {
    name: values.name.trim(),
    targetAmount: toNumber(values.targetAmount),
    targetDate: values.targetDate || null,
    color: values.color,
  }
}

export function GoalForm({
  open,
  goal,
  onClose,
  onSubmit,
}: {
  open: boolean
  goal: Goal | null
  onClose: () => void
  onSubmit: (input: GoalInput) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<GoalFormValues>({
    resolver: zodResolver(goalSchema),
    defaultValues: {
      name: goal?.name ?? '',
      targetAmount: goal ? String(goal.targetAmount) : '',
      targetDate: goal?.targetDate ? goal.targetDate.slice(0, 10) : '',
      color: goal?.color ?? GOAL_COLORS[0],
    },
  })

  const color = watch('color')

  const submit = handleSubmit(async (values) => {
    await onSubmit(toGoalInput(values))
  })

  return (
    <Modal
      open={open}
      title={goal ? 'Edit goal' : 'New goal'}
      onClose={onClose}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="goal-form" type="submit" loading={isSubmitting}>
            {goal ? 'Save changes' : 'Add goal'}
          </Button>
        </>
      }
    >
      <form
        id="goal-form"
        onSubmit={submit}
        noValidate
        className="flex flex-col gap-4"
      >
        <Input
          label="Name"
          placeholder="Emergency fund"
          error={errors.name?.message}
          {...register('name')}
        />
        <Input
          label="Target amount"
          type="number"
          step="0.01"
          min="0"
          inputMode="decimal"
          placeholder="0.00"
          error={errors.targetAmount?.message}
          {...register('targetAmount')}
        />
        <Input
          label="Target date (optional)"
          type="date"
          error={errors.targetDate?.message}
          {...register('targetDate')}
        />

        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
            Colour
          </span>
          <div className="flex flex-wrap gap-2">
            {GOAL_COLORS.map((option) => (
              <button
                key={option}
                type="button"
                aria-label={`Use colour ${option}`}
                aria-pressed={color === option}
                onClick={() => setValue('color', option)}
                style={{ backgroundColor: option }}
                className={
                  color === option
                    ? 'size-7 rounded-full ring-2 ring-slate-900 ring-offset-2 dark:ring-slate-100 dark:ring-offset-slate-900'
                    : 'size-7 rounded-full'
                }
              />
            ))}
          </div>
          <input type="hidden" {...register('color')} />
        </div>
      </form>
    </Modal>
  )
}
