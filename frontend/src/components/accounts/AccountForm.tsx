import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Checkbox, Input, Select } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import { CURRENCIES } from '@/lib/currencies'
import { requiredString, toNumber } from '@/lib/validation'
import type { Account, AccountInput, AccountUpdate } from '@/types'
import { ACCOUNT_TYPES, accountMeta } from './accountMeta'

export const accountSchema = z.object({
  name: requiredString('Name is required').max(60, 'Name is too long'),
  type: z.enum(['checking', 'savings', 'creditCard', 'cash', 'investment']),
  currency: requiredString('Currency is required'),
  initialBalance: z
    .string()
    .refine(
      (value) => value === '' || Number.isFinite(Number(value)),
      'Enter a valid amount',
    ),
  isArchived: z.boolean(),
})

export type AccountFormValues = z.infer<typeof accountSchema>

export function toCreateInput(values: AccountFormValues): AccountInput {
  return {
    name: values.name.trim(),
    type: values.type,
    initialBalance: toNumber(values.initialBalance || '0'),
    currency: values.currency,
  }
}

export function toUpdateInput(values: AccountFormValues): AccountUpdate {
  return {
    name: values.name.trim(),
    type: values.type,
    currency: values.currency,
    isArchived: values.isArchived,
  }
}

export function AccountForm({
  open,
  account,
  defaultCurrency,
  onClose,
  onSubmit,
}: {
  open: boolean
  account: Account | null
  defaultCurrency: string
  onClose: () => void
  onSubmit: (values: AccountFormValues) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<AccountFormValues>({
    resolver: zodResolver(accountSchema),
    defaultValues: account
      ? {
          name: account.name,
          type: account.type,
          currency: account.currency,
          initialBalance: String(account.balance),
          isArchived: account.isArchived,
        }
      : {
          name: '',
          type: 'checking',
          currency: defaultCurrency,
          initialBalance: '0',
          isArchived: false,
        },
  })

  return (
    <Modal
      open={open}
      title={account ? 'Edit account' : 'New account'}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="account-form" type="submit" loading={isSubmitting}>
            {account ? 'Save changes' : 'Add account'}
          </Button>
        </>
      }
    >
      <form
        id="account-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="grid gap-4 sm:grid-cols-2"
      >
        <Input
          label="Name"
          placeholder="Everyday checking"
          wrapperClassName="sm:col-span-2"
          error={errors.name?.message}
          {...register('name')}
        />

        <Select label="Type" error={errors.type?.message} {...register('type')}>
          {ACCOUNT_TYPES.map((type) => (
            <option key={type} value={type}>
              {accountMeta(type).label}
            </option>
          ))}
        </Select>

        <Select
          label="Currency"
          error={errors.currency?.message}
          {...register('currency')}
        >
          {CURRENCIES.map((code) => (
            <option key={code} value={code}>
              {code}
            </option>
          ))}
        </Select>

        {account ? (
          <Checkbox
            label="Archived"
            className="sm:col-span-2"
            {...register('isArchived')}
          />
        ) : (
          <Input
            label="Initial balance"
            type="number"
            step="0.01"
            inputMode="decimal"
            wrapperClassName="sm:col-span-2"
            hint="Where this account stands today."
            error={errors.initialBalance?.message}
            {...register('initialBalance')}
          />
        )}
      </form>
    </Modal>
  )
}
