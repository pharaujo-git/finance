import {
  useCreateRecurring,
  useDeleteRecurring,
  useRecurring,
  useUpdateRecurring,
} from '@/api/queries'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Icon } from '@/components/ui/Icon'
import { SkeletonRows } from '@/components/ui/Skeleton'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import type { Lookups } from '@/hooks/useLookups'
import { useToastAction } from '@/hooks/useToastAction'
import { formatCurrency, formatDate } from '@/lib/format'
import type { RecurringInput, RecurringRule } from '@/types'
import { RecurringForm, toRecurringInput } from './RecurringForm'

export function RecurringSection({
  lookups,
  currency,
}: {
  lookups: Lookups
  currency: string
}) {
  const query = useRecurring()
  const editor = useEditor<RecurringRule>()
  const confirm = useConfirm<RecurringRule>()
  const run = useToastAction()

  const create = useCreateRecurring()
  const update = useUpdateRecurring()
  const remove = useDeleteRecurring()

  const rules = query.data ?? []

  const save = async (input: RecurringInput) => {
    const result = await run(
      () =>
        editor.editing
          ? update.mutateAsync({ id: editor.editing.id, body: input })
          : create.mutateAsync(input),
      editor.editing ? 'Recurring rule updated' : 'Recurring rule added',
    )
    if (result) editor.close()
  }

  const toggleActive = (rule: RecurringRule) => {
    const body = toRecurringInput({
      type: rule.type,
      accountId: rule.accountId,
      categoryId: rule.categoryId ?? '',
      amount: String(rule.amount),
      description: rule.description,
      frequency: rule.frequency,
      startDate: rule.startDate.slice(0, 10),
      endDate: rule.endDate ? rule.endDate.slice(0, 10) : '',
      isActive: !rule.isActive,
    })
    return run(
      () => update.mutateAsync({ id: rule.id, body }),
      rule.isActive ? 'Rule paused' : 'Rule resumed',
    )
  }

  const confirmDelete = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => remove.mutateAsync(target.id), 'Recurring rule deleted')
    confirm.cancel()
  }

  return (
    <Card>
      <CardHeader
        title="Recurring rules"
        subtitle="Automatic transactions on a schedule"
        action={
          <Button
            size="sm"
            onClick={editor.openCreate}
            icon={<Icon name="plus" className="size-4" />}
          >
            New rule
          </Button>
        }
      />

      <DataState
        isLoading={query.isLoading}
        isError={query.isError}
        error={query.error}
        isEmpty={rules.length === 0}
        onRetry={() => void query.refetch()}
        skeleton={<SkeletonRows rows={3} className="p-4" />}
        empty={{
          icon: 'repeat',
          title: 'No recurring rules yet',
          description:
            'Add rent, salary or a subscription and it will be created for you each period.',
          action: (
            <Button size="sm" onClick={editor.openCreate}>
              Add your first rule
            </Button>
          ),
        }}
      >
        <ul className="divide-y divide-slate-100 dark:divide-slate-800">
          {rules.map((rule) => (
            <li
              key={rule.id}
              className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
            >
              <div className="min-w-0">
                <p className="flex items-center gap-2 text-sm font-medium text-slate-900 dark:text-slate-100">
                  {rule.description}
                  <Badge tone={rule.isActive ? 'success' : 'neutral'}>
                    {rule.isActive ? 'active' : 'paused'}
                  </Badge>
                </p>
                <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                  {rule.frequency} · {lookups.accountName(rule.accountId)} ·{' '}
                  {lookups.categoryName(rule.categoryId)} · next{' '}
                  {formatDate(rule.nextRunDate)}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                  {formatCurrency(rule.amount, currency)}
                </span>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => void toggleActive(rule)}
                >
                  {rule.isActive ? 'Pause' : 'Resume'}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Edit ${rule.description}`}
                  onClick={() => editor.openEdit(rule)}
                >
                  <Icon name="edit" className="size-4" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Delete ${rule.description}`}
                  onClick={() => confirm.request(rule)}
                  className="text-rose-600 dark:text-rose-400"
                >
                  <Icon name="trash" className="size-4" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </DataState>

      {editor.isOpen ? (
        <RecurringForm
          open
          rule={editor.editing}
          lookups={lookups}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Delete recurring rule"
        message={`Delete “${confirm.target?.description ?? ''}”? Transactions already created stay untouched.`}
        loading={remove.isPending}
        onConfirm={() => void confirmDelete()}
        onCancel={confirm.cancel}
      />
    </Card>
  )
}
