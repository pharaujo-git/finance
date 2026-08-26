import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input, Select } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import { formatMonth } from '@/lib/format'
import { moneyString, requiredString, toNumber } from '@/lib/validation'
import type { Budget, Category } from '@/types'

export const budgetSchema = z.object({
  categoryId: requiredString('Category is required'),
  limit: moneyString,
})

export type BudgetFormValues = z.infer<typeof budgetSchema>

export function BudgetForm({
  open,
  budget,
  month,
  categories,
  onClose,
  onSubmit,
}: {
  open: boolean
  budget: Budget | null
  month: string
  categories: Category[]
  onClose: () => void
  onSubmit: (values: { categoryId: string; limit: number }) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<BudgetFormValues>({
    resolver: zodResolver(budgetSchema),
    defaultValues: {
      categoryId: budget?.categoryId ?? '',
      limit: budget ? String(budget.limit) : '',
    },
  })

  const submit = handleSubmit(async (values) => {
    await onSubmit({
      categoryId: values.categoryId,
      limit: toNumber(values.limit),
    })
  })

  return (
    <Modal
      open={open}
      title={budget ? 'Edit budget' : 'New budget'}
      description={`For ${formatMonth(month)}`}
      onClose={onClose}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="budget-form" type="submit" loading={isSubmitting}>
            {budget ? 'Save changes' : 'Add budget'}
          </Button>
        </>
      }
    >
      <form
        id="budget-form"
        onSubmit={submit}
        noValidate
        className="flex flex-col gap-4"
      >
        {budget ? (
          <>
            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
                Category
              </span>
              <p className="text-sm text-slate-900 dark:text-slate-100">
                {categories.find((item) => item.id === budget.categoryId)
                  ?.name ?? 'Unknown category'}
              </p>
            </div>
            <input type="hidden" {...register('categoryId')} />
          </>
        ) : (
          <Select
            label="Category"
            error={errors.categoryId?.message}
            {...register('categoryId')}
          >
            <option value="">Select a category</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>
        )}
        <Input
          label="Monthly limit"
          type="number"
          step="0.01"
          min="0"
          inputMode="decimal"
          placeholder="0.00"
          error={errors.limit?.message}
          {...register('limit')}
        />
      </form>
    </Modal>
  )
}
