import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Field'
import { Modal } from '@/components/ui/Modal'
import { formatCurrency } from '@/lib/format'
import { moneyString, toNumber } from '@/lib/validation'
import type { Goal } from '@/types'

export const contributeSchema = z.object({ amount: moneyString })
export type ContributeValues = z.infer<typeof contributeSchema>

export function ContributeForm({
  goal,
  currency,
  onClose,
  onSubmit,
}: {
  goal: Goal
  currency: string
  onClose: () => void
  onSubmit: (amount: number) => Promise<void>
}) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ContributeValues>({
    resolver: zodResolver(contributeSchema),
    defaultValues: { amount: '' },
  })

  const submit = handleSubmit(async (values) => {
    await onSubmit(toNumber(values.amount))
  })

  const remaining = Math.max(0, goal.targetAmount - goal.currentAmount)

  return (
    <Modal
      open
      title={`Add to ${goal.name}`}
      description={`${formatCurrency(remaining, currency)} still to go.`}
      onClose={onClose}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button form="contribute-form" type="submit" loading={isSubmitting}>
            Add contribution
          </Button>
        </>
      }
    >
      <form id="contribute-form" onSubmit={submit} noValidate>
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
      </form>
    </Modal>
  )
}
