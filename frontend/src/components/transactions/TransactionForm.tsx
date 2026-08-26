import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input, Select, Textarea } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import type { Lookups } from '@/hooks/useLookups'
import { todayInput } from '@/lib/format'
import {
  formatTags,
  moneyString,
  parseTags,
  requiredString,
  toMoneyInput,
  toNumber,
} from '@/lib/validation'
import type { Transaction, TransactionInput } from '@/types'

export const transactionSchema = z
  .object({
    type: z.enum(['income', 'expense', 'transfer']),
    accountId: requiredString('Account is required'),
    transferAccountId: z.string(),
    categoryId: z.string(),
    amount: moneyString,
    date: requiredString('Date is required'),
    description: requiredString('Description is required').max(
      140,
      'Keep the description under 140 characters',
    ),
    notes: z.string().max(1000, 'Notes are too long'),
    tags: z.string(),
  })
  .superRefine((values, ctx) => {
    if (values.type === 'transfer') {
      if (!values.transferAccountId) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['transferAccountId'],
          message: 'Choose the account the money moves to',
        })
      } else if (values.transferAccountId === values.accountId) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['transferAccountId'],
          message: 'Pick a different destination account',
        })
      }
    } else if (!values.categoryId) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['categoryId'],
        message: 'Category is required',
      })
    }
  })

export type TransactionFormValues = z.infer<typeof transactionSchema>

export function toTransactionInput(
  values: TransactionFormValues,
): TransactionInput {
  const isTransfer = values.type === 'transfer'
  return {
    accountId: values.accountId,
    categoryId: isTransfer ? null : values.categoryId || null,
    type: values.type,
    amount: toNumber(values.amount),
    date: values.date,
    description: values.description.trim(),
    notes: values.notes.trim() || null,
    tags: parseTags(values.tags),
    transferAccountId: isTransfer ? values.transferAccountId : null,
  }
}

function defaultsFrom(
  transaction: Transaction | null,
  fallbackAccountId: string,
): TransactionFormValues {
  if (!transaction) {
    return {
      type: 'expense',
      accountId: fallbackAccountId,
      transferAccountId: '',
      categoryId: '',
      amount: '',
      date: todayInput(),
      description: '',
      notes: '',
      tags: '',
    }
  }
  return {
    type: transaction.type,
    accountId: transaction.accountId,
    transferAccountId: transaction.transferAccountId ?? '',
    categoryId: transaction.categoryId ?? '',
    amount: toMoneyInput(transaction.amount),
    date: transaction.date.slice(0, 10),
    description: transaction.description ?? '',
    notes: transaction.notes ?? '',
    tags: formatTags(transaction.tags),
  }
}

export interface TransactionFormProps {
  open: boolean
  transaction: Transaction | null
  lookups: Lookups
  onClose: () => void
  onSubmit: (input: TransactionInput) => Promise<void>
}

export function TransactionForm({
  open,
  transaction,
  lookups,
  onClose,
  onSubmit,
}: TransactionFormProps) {
  const fallbackAccountId = lookups.activeAccounts[0]?.id ?? ''

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<TransactionFormValues>({
    resolver: zodResolver(transactionSchema),
    defaultValues: defaultsFrom(transaction, fallbackAccountId),
  })

  const type = watch('type')
  const isTransfer = type === 'transfer'
  const categories = lookups.categories.filter(
    (category) => category.type === type,
  )

  const submit = handleSubmit(async (values) => {
    await onSubmit(toTransactionInput(values))
  })

  return (
    <Modal
      open={open}
      title={transaction ? 'Edit transaction' : 'New transaction'}
      description="Amounts are always positive — the type decides the direction."
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="transaction-form" type="submit" loading={isSubmitting}>
            {transaction ? 'Save changes' : 'Add transaction'}
          </Button>
        </>
      }
    >
      <form
        id="transaction-form"
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
          label={isTransfer ? 'From account' : 'Account'}
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

        {isTransfer ? (
          <Select
            label="To account"
            error={errors.transferAccountId?.message}
            {...register('transferAccountId')}
          >
            <option value="">Select an account</option>
            {lookups.activeAccounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.name}
              </option>
            ))}
          </Select>
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
          label="Date"
          type="date"
          error={errors.date?.message}
          {...register('date')}
        />

        <Input
          label="Tags"
          placeholder="groceries, weekly"
          hint="Separate with commas"
          error={errors.tags?.message}
          {...register('tags')}
        />

        <Input
          label="Description"
          placeholder="Supermarket run"
          wrapperClassName="sm:col-span-2"
          error={errors.description?.message}
          {...register('description')}
        />

        <Textarea
          label="Notes"
          rows={3}
          placeholder="Optional details"
          wrapperClassName="sm:col-span-2"
          error={errors.notes?.message}
          {...register('notes')}
        />
      </form>
    </Modal>
  )
}
