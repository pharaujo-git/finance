import { useState } from 'react'
import {
  useContributeGoal,
  useCreateGoal,
  useDeleteGoal,
  useGoals,
  useUpdateGoal,
} from '@/api/queries'
import { ContributeForm } from '@/components/goals/ContributeForm'
import { GoalForm } from '@/components/goals/GoalForm'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Icon } from '@/components/ui/Icon'
import { PageHeader } from '@/components/ui/PageHeader'
import { ProgressRing } from '@/components/ui/ProgressBar'
import { SkeletonCards } from '@/components/ui/Skeleton'
import { useCurrency } from '@/context/auth-context'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import { useToastAction } from '@/hooks/useToastAction'
import { formatCurrency, formatDate } from '@/lib/format'
import type { Goal, GoalInput } from '@/types'

function GoalCard({
  goal,
  currency,
  onEdit,
  onDelete,
  onContribute,
}: {
  goal: Goal
  currency: string
  onEdit: (goal: Goal) => void
  onDelete: (goal: Goal) => void
  onContribute: (goal: Goal) => void
}) {
  const reached = goal.currentAmount >= goal.targetAmount

  return (
    <div className="card flex flex-col gap-4 p-5">
      <div className="flex items-start gap-4">
        <ProgressRing
          value={goal.currentAmount}
          max={goal.targetAmount}
          color={goal.color}
          label={`${goal.name} progress`}
        />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
            {goal.name}
          </p>
          <p className="mt-1 text-lg font-semibold tabular-nums text-slate-900 dark:text-slate-50">
            {formatCurrency(goal.currentAmount, currency)}
          </p>
          <p className="text-xs text-slate-500 dark:text-slate-400">
            of {formatCurrency(goal.targetAmount, currency)}
            {goal.targetDate ? ` by ${formatDate(goal.targetDate)}` : ''}
          </p>
          {reached ? (
            <p className="mt-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
              Goal reached
            </p>
          ) : null}
        </div>
      </div>

      <div className="mt-auto flex items-center justify-between border-t border-slate-100 pt-3 dark:border-slate-800">
        <Button
          size="sm"
          variant="subtle"
          onClick={() => onContribute(goal)}
          icon={<Icon name="plus" className="size-3.5" />}
        >
          Add contribution
        </Button>
        <div className="flex gap-1">
          <Button
            size="sm"
            variant="ghost"
            aria-label={`Edit ${goal.name}`}
            onClick={() => onEdit(goal)}
          >
            <Icon name="edit" className="size-4" />
          </Button>
          <Button
            size="sm"
            variant="ghost"
            aria-label={`Delete ${goal.name}`}
            onClick={() => onDelete(goal)}
            className="text-rose-600 dark:text-rose-400"
          >
            <Icon name="trash" className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}

export function GoalsPage() {
  const currency = useCurrency()
  const query = useGoals()
  const run = useToastAction()

  const editor = useEditor<Goal>()
  const confirm = useConfirm<Goal>()
  const [contributing, setContributing] = useState<Goal | null>(null)

  const create = useCreateGoal()
  const update = useUpdateGoal()
  const remove = useDeleteGoal()
  const contribute = useContributeGoal()

  const goals = query.data ?? []

  const save = async (input: GoalInput) => {
    const editing = editor.editing
    const result = await run(
      () =>
        editing
          ? update.mutateAsync({ id: editing.id, body: input })
          : create.mutateAsync(input),
      editing ? 'Goal updated' : 'Goal added',
    )
    if (result) editor.close()
  }

  const addContribution = async (amount: number) => {
    if (!contributing) return
    const result = await run(
      () => contribute.mutateAsync({ id: contributing.id, amount }),
      'Contribution added',
    )
    if (result) setContributing(null)
  }

  const confirmDelete = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => remove.mutateAsync(target.id), 'Goal deleted')
    confirm.cancel()
  }

  return (
    <>
      <PageHeader
        title="Goals"
        description="What you are saving towards, and how far along you are."
        actions={
          <Button
            onClick={editor.openCreate}
            icon={<Icon name="plus" className="size-4" />}
          >
            Add goal
          </Button>
        }
      />

      <DataState
        isLoading={query.isLoading}
        isError={query.isError}
        error={query.error}
        isEmpty={goals.length === 0}
        onRetry={() => void query.refetch()}
        skeleton={<SkeletonCards count={3} />}
        empty={{
          icon: 'goals',
          title: 'No goals yet',
          description:
            'Name something you are saving for and track every contribution.',
          action: (
            <Button size="sm" onClick={editor.openCreate}>
              Create your first goal
            </Button>
          ),
        }}
      >
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {goals.map((goal) => (
            <GoalCard
              key={goal.id}
              goal={goal}
              currency={currency}
              onEdit={editor.openEdit}
              onDelete={confirm.request}
              onContribute={setContributing}
            />
          ))}
        </div>
      </DataState>

      {editor.isOpen ? (
        <GoalForm
          open
          goal={editor.editing}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      {contributing ? (
        <ContributeForm
          goal={contributing}
          currency={currency}
          onClose={() => setContributing(null)}
          onSubmit={addContribution}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Delete goal"
        message={`Delete “${confirm.target?.name ?? ''}”? Contributions recorded against it are removed too.`}
        loading={remove.isPending}
        onConfirm={() => void confirmDelete()}
        onCancel={confirm.cancel}
      />
    </>
  )
}
