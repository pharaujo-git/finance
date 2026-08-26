import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Checkbox, Input, Select } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import type { Lookups } from '@/hooks/useLookups'
import { todayInput } from '@/lib/format'
import { moneyString, requiredString, toMoneyInput, toNumber } from '@/lib/validation'
import type { RecurringInput, RecurringRule } from '@/types'

export const recurringSchema = z
  .object({
    type: z.enum(['income', 'expense', 'transfer']),
    accountId: requiredString('Account is required'),
    categoryId: z.string(),
    amount: moneyString,
    description: requiredString('Description is required').max(140),
    frequency: z.enum(['daily', 'weekly', 'monthly', 'yearly']),
    startDate: requiredString('Start date is required'),
    endDate: z.string(),
    isActive: z.boolean(),
  })
  .superRefine((values, ctx) => {
    if (values.endDate && values.endDate < values.startDate) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['endDate'],
        message: 'End date must be after the start date',
      })
    }
  })

export type RecurringFormValues = z.infer<typeof recurringSchema>

export function toRecurringInput(values: RecurringFormValues): RecurringInput {
  return {
    accountId: values.accountId,
    categoryId: values.categoryId || null,
    type: values.type,
    amount: toNumber(values.amount),
    description: values.description.trim(),
    frequency: values.frequency,
    startDate: values.startDate,
    endDate: values.endDate || null,
    isActive: values.isActive,
  }
}

function defaultsFrom(
  rule: RecurringRule | null,
  fallbackAccountId: string,
): RecurringFormValues {
  if (!rule) {
    return {
      type: 'expense',
      accountId: fallbackAccountId,
      categoryId: '',
      amount: '',
      description: '',
      frequency: 'monthly',
      startDate: todayInput(),
      endDate: '',
      isActive: true,
    }
  }
  return {
    type: rule.type,
    accountId: rule.accountId,
    categoryId: rule.categoryId ?? '',
    amount: toMoneyInput(rule.amount),
    description: rule.description,
    frequency: rule.frequency,
    startDate: rule.startDate.slice(0, 10),
    endDate: rule.endDate ? rule.endDate.slice(0, 10) : '',
    isActive: rule.isActive,
  }
}

export function RecurringForm({
  open,
  rule,
  lookups,
  onClose,
  onSubmit,
}: {
  open: boolean
  rule: RecurringRule | null
  lookups: Lookups
  onClose: () => void
  onSubmit: (input: RecurringInput) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<RecurringFormValues>({
    resolver: zodResolver(recurringSchema),
    defaultValues: defaultsFrom(rule, lookups.activeAccounts[0]?.id ?? ''),
  })

  const type = watch('type')
  const categories = lookups.categories.filter(
    (category) => category.type === type,
  )

  const submit = handleSubmit(async (values) => {
    await onSubmit(toRecurringInput(values))
  })

  return (
    <Modal
      open={open}
      title={rule ? 'Edit recurring rule' : 'New recurring rule'}
      description="Rules generate transactions automatically on their schedule."
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="recurring-form" type="submit" loading={isSubmitting}>
            {rule ? 'Save changes' : 'Add rule'}
          </Button>
        </>
      }
    >
      <form
        id="recurring-form"
        onSubmit={submit}
        noValidate
        className="grid gap-4 sm:grid-cols-2"
      >
        <Select label="Type" error={errors.type?.message} {...register('type')}>
          <option value="expense">Expense</option>
          <option value="income">Income</option>
          <option value="transfer">Transfer</option>
        </Select>

        <Input
          label="Amount"
          type="number"
          step="0.01"
          min="0"
          inputMode="decimal"
          placeholder="0.00"
          error={errors.amount?.message}
          {...register('amount')}
        />

        <Select
          label="Account"
          error={errors.accountId?.message}
          {...register('accountId')}
        >
          <option value="">Select an account</option>
          {lookups.activeAccounts.map((account) => (
            <option key={account.id} value={account.id}>
              {account.name}
            </option>
          ))}
        </Select>

        <Select
          label="Category"
          error={errors.categoryId?.message}
          {...register('categoryId')}
        >
          <option value="">No category</option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>

        <Select
          label="Frequency"
          error={errors.frequency?.message}
          {...register('frequency')}
        >
          <option value="daily">Daily</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
          <option value="yearly">Yearly</option>
        </Select>

        <Input
          label="Description"
          placeholder="Rent"
          error={errors.description?.message}
          {...register('description')}
        />

        <Input
          label="Starts"
          type="date"
          error={errors.startDate?.message}
          {...register('startDate')}
        />

        <Input
          label="Ends (optional)"
          type="date"
          error={errors.endDate?.message}
          {...register('endDate')}
        />

        <Checkbox
          label="Active"
          className="sm:col-span-2"
          {...register('isActive')}
        />
      </form>
    </Modal>
  )
}
