import {
  useAccounts,
  useArchiveAccount,
  useCreateAccount,
  useUpdateAccount,
} from '@/api/queries'
import {
  AccountForm,
  toCreateInput,
  toUpdateInput,
  type AccountFormValues,
} from '@/components/accounts/AccountForm'
import { accountMeta } from '@/components/accounts/accountMeta'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Icon } from '@/components/ui/Icon'
import { PageHeader } from '@/components/ui/PageHeader'
import { SkeletonCards } from '@/components/ui/Skeleton'
import { useAuth } from '@/context/auth-context'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import { useToastAction } from '@/hooks/useToastAction'
import { cn } from '@/lib/cn'
import { formatCurrency, formatDate } from '@/lib/format'
import type { Account } from '@/types'

function AccountCard({
  account,
  onEdit,
  onArchive,
}: {
  account: Account
  onEdit: (account: Account) => void
  onArchive: (account: Account) => void
}) {
  const meta = accountMeta(account.type)
  return (
    <div
      className={cn(
        'card flex flex-col gap-4 p-5',
        account.isArchived && 'opacity-60',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <span className={cn('rounded-lg p-2.5', meta.tint)}>
            <Icon name={meta.icon} className="size-5" />
          </span>
          <div>
            <p className="text-sm font-semibold text-slate-900 dark:text-slate-100">
              {account.name}
            </p>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              {meta.label} · {account.currency}
            </p>
          </div>
        </div>
        {account.isArchived ? <Badge>archived</Badge> : null}
      </div>

      <p
        className={cn(
          'text-2xl font-semibold tracking-tight tabular-nums',
          account.balance < 0
            ? 'text-rose-600 dark:text-rose-400'
            : 'text-slate-900 dark:text-slate-50',
        )}
      >
        {formatCurrency(account.balance, account.currency)}
      </p>

      <div className="mt-auto flex items-center justify-between border-t border-slate-100 pt-3 dark:border-slate-800">
        <span className="text-xs text-slate-400 dark:text-slate-500">
          Opened {formatDate(account.createdAt)}
        </span>
        <div className="flex gap-1">
          <Button
            size="sm"
            variant="ghost"
            aria-label={`Edit ${account.name}`}
            onClick={() => onEdit(account)}
          >
            <Icon name="edit" className="size-4" />
          </Button>
          {account.isArchived ? null : (
            <Button
              size="sm"
              variant="ghost"
              aria-label={`Archive ${account.name}`}
              onClick={() => onArchive(account)}
            >
              <Icon name="archive" className="size-4" />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

export function AccountsPage() {
  const { currency } = useAuth()
  const query = useAccounts()
  const editor = useEditor<Account>()
  const confirm = useConfirm<Account>()
  const run = useToastAction()

  const create = useCreateAccount()
  const update = useUpdateAccount()
  const archive = useArchiveAccount()

  const accounts = query.data ?? []

  const save = async (values: AccountFormValues) => {
    const editing = editor.editing
    const result = await run(
      () =>
        editing
          ? update.mutateAsync({ id: editing.id, body: toUpdateInput(values) })
          : create.mutateAsync(toCreateInput(values)),
      editing ? 'Account updated' : 'Account added',
    )
    if (result) editor.close()
  }

  const confirmArchive = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => archive.mutateAsync(target.id), 'Account archived')
    confirm.cancel()
  }

  return (
    <>
      <PageHeader
        title="Accounts"
        description="Balances across every place you keep money."
        actions={
          <Button
            onClick={editor.openCreate}
            icon={<Icon name="plus" className="size-4" />}
          >
            Add account
          </Button>
        }
      />

      <DataState
        isLoading={query.isLoading}
        isError={query.isError}
        error={query.error}
        isEmpty={accounts.length === 0}
        onRetry={() => void query.refetch()}
        skeleton={<SkeletonCards count={3} />}
        empty={{
          icon: 'accounts',
          title: 'No accounts yet',
          description:
            'Add a checking account, a card or a wallet to start tracking.',
          action: (
            <Button size="sm" onClick={editor.openCreate}>
              Add your first account
            </Button>
          ),
        }}
      >
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {accounts.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              onEdit={editor.openEdit}
              onArchive={confirm.request}
            />
          ))}
        </div>
      </DataState>

      {editor.isOpen ? (
        <AccountForm
          open
          account={editor.editing}
          defaultCurrency={currency}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Archive account"
        message={`Archive “${confirm.target?.name ?? ''}”? Its history stays, but it is hidden from pickers.`}
        confirmLabel="Archive"
        loading={archive.isPending}
        onConfirm={() => void confirmArchive()}
        onCancel={confirm.cancel}
      />
    </>
  )
}
