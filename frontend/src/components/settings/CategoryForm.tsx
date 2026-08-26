import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input, Select } from '@/components/ui/Field'
import { Icon, ICON_PATHS, type IconName } from '@/components/ui/Icon'
import { Modal } from '@/components/ui/Modal'
import { cn } from '@/lib/cn'
import { requiredString } from '@/lib/validation'
import type { Category, CategoryInput } from '@/types'

export const CATEGORY_COLORS = [
  '#ef4444',
  '#f97316',
  '#f59e0b',
  '#10b981',
  '#14b8a6',
  '#0ea5e9',
  '#6366f1',
  '#8b5cf6',
  '#ec4899',
  '#64748b',
]

export const CATEGORY_ICONS: IconName[] = [
  'cash',
  'transactions',
  'accounts',
  'budgets',
  'goals',
  'reports',
  'investment',
  'savings',
  'creditCard',
  'repeat',
]

export const categorySchema = z.object({
  name: requiredString('Name is required').max(40, 'Name is too long'),
  type: z.enum(['income', 'expense']),
  icon: requiredString('Pick an icon'),
  color: requiredString('Pick a colour'),
})

export type CategoryFormValues = z.infer<typeof categorySchema>

export function toCategoryInput(values: CategoryFormValues): CategoryInput {
  return {
    name: values.name.trim(),
    type: values.type,
    icon: values.icon,
    color: values.color,
  }
}

function isKnownIcon(value: string): value is IconName {
  return value in ICON_PATHS
}

export function CategoryForm({
  open,
  category,
  onClose,
  onSubmit,
}: {
  open: boolean
  category: Category | null
  onClose: () => void
  onSubmit: (input: CategoryInput) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CategoryFormValues>({
    resolver: zodResolver(categorySchema),
    defaultValues: {
      name: category?.name ?? '',
      type: category?.type ?? 'expense',
      icon: category?.icon ?? CATEGORY_ICONS[0],
      color: category?.color ?? CATEGORY_COLORS[0],
    },
  })

  const color = watch('color')
  const icon = watch('icon')

  const submit = handleSubmit(async (values) => {
    await onSubmit(toCategoryInput(values))
  })

  return (
    <Modal
      open={open}
      title={category ? 'Edit category' : 'New category'}
      onClose={onClose}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="category-form" type="submit" loading={isSubmitting}>
            {category ? 'Save changes' : 'Add category'}
          </Button>
        </>
      }
    >
      <form
        id="category-form"
        onSubmit={submit}
        noValidate
        className="flex flex-col gap-4"
      >
        <Input
          label="Name"
          placeholder="Groceries"
          error={errors.name?.message}
          {...register('name')}
        />

        <Select label="Type" error={errors.type?.message} {...register('type')}>
          <option value="expense">Expense</option>
          <option value="income">Income</option>
        </Select>

        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
            Colour
          </span>
          <div className="flex flex-wrap gap-2">
            {CATEGORY_COLORS.map((option) => (
              <button
                key={option}
                type="button"
                aria-label={`Use colour ${option}`}
                aria-pressed={color === option}
                onClick={() => setValue('color', option)}
                style={{ backgroundColor: option }}
                className={cn(
                  'size-7 rounded-full',
                  color === option &&
                    'ring-2 ring-slate-900 ring-offset-2 dark:ring-slate-100 dark:ring-offset-slate-900',
                )}
              />
            ))}
          </div>
          <input type="hidden" {...register('color')} />
        </div>

        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
            Icon
          </span>
          <div className="flex flex-wrap gap-2">
            {CATEGORY_ICONS.map((option) => (
              <button
                key={option}
                type="button"
                aria-label={`Use icon ${option}`}
                aria-pressed={icon === option}
                onClick={() => setValue('icon', option)}
                className={cn(
                  'rounded-lg border p-2 transition',
                  icon === option
                    ? 'border-brand-500 bg-brand-50 text-brand-700 dark:bg-brand-900/40 dark:text-brand-200'
                    : 'border-slate-200 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800',
                )}
              >
                <Icon name={option} className="size-4" />
              </button>
            ))}
          </div>
          <input type="hidden" {...register('icon')} />
          {errors.icon ? (
            <p role="alert" className="text-xs text-rose-600">
              {errors.icon.message}
            </p>
          ) : null}
        </div>
      </form>
    </Modal>
  )
}

/** Falls back to a neutral glyph when the API returns an unknown icon name. */
export function CategoryIcon({
  name,
  className,
}: {
  name: string
  className?: string
}) {
  return (
    <Icon name={isKnownIcon(name) ? name : 'cash'} className={className} />
  )
}
