import { useCallback, useRef, useState } from 'react'
import { transactions as transactionsApi } from '@/api/endpoints'
import {
  useCreateTransaction,
  useDeleteTransaction,
  useImportTransactions,
  useTransactions,
  useUpdateTransaction,
} from '@/api/queries'
import { RecurringSection } from '@/components/transactions/RecurringSection'
import { TransactionFilterBar, EMPTY_FILTERS } from '@/components/transactions/TransactionFilterBar'
import { TransactionForm } from '@/components/transactions/TransactionForm'
import { TransactionRow } from '@/components/transactions/TransactionRow'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Icon } from '@/components/ui/Icon'
import { PageHeader } from '@/components/ui/PageHeader'
import { Pagination } from '@/components/ui/Pagination'
import { SkeletonRows } from '@/components/ui/Skeleton'
import { Tabs } from '@/components/ui/Tabs'
import { useCurrency } from '@/context/auth-context'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import { useLookups } from '@/hooks/useLookups'
import { useToastAction } from '@/hooks/useToastAction'
import { downloadBlob } from '@/lib/download'
import { todayInput } from '@/lib/format'
import type { Transaction, TransactionFilters, TransactionInput } from '@/types'

const PAGE_SIZE = 20
const TABS = [
  { id: 'transactions' as const, label: 'Transactions' },
  { id: 'recurring' as const, label: 'Recurring' },
]

const COLUMNS = ['Date', 'Description', 'Category', 'Account', 'Amount', '']

export function TransactionsPage() {
  const currency = useCurrency()
  const lookups = useLookups()
  const run = useToastAction()
  const fileInput = useRef<HTMLInputElement>(null)

  const [tab, setTab] = useState<'transactions' | 'recurring'>('transactions')
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<TransactionFilters>(EMPTY_FILTERS)

  const query = useTransactions({ ...filters, page, pageSize: PAGE_SIZE })
  const editor = useEditor<Transaction>()
  const confirm = useConfirm<Transaction>()

  const create = useCreateTransaction()
  const update = useUpdateTransaction()
  const remove = useDeleteTransaction()
  const importCsv = useImportTransactions()

  const items = query.data?.items ?? []
  const total = query.data?.total ?? 0

  const patchFilters = useCallback((patch: Partial<TransactionFilters>) => {
    setFilters((current) => ({ ...current, ...patch }))
    setPage(1)
  }, [])

  const resetFilters = useCallback(() => {
    setFilters(EMPTY_FILTERS)
    setPage(1)
  }, [])

  const save = async (input: TransactionInput) => {
    const result = await run(
      () =>
        editor.editing
          ? update.mutateAsync({ id: editor.editing.id, body: input })
          : create.mutateAsync(input),
      editor.editing ? 'Transaction updated' : 'Transaction added',
    )
    if (result) editor.close()
  }

  const confirmDelete = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => remove.mutateAsync(target.id), 'Transaction deleted')
    confirm.cancel()
  }

  const exportCsv = () =>
    run(async () => {
      const blob = await transactionsApi.exportCsv({
        from: filters.from,
        to: filters.to,
      })
      downloadBlob(blob, `transactions-${todayInput()}.csv`)
    }, 'Export ready')

  const onFilePicked = async (file: File | undefined) => {
    if (!file) return
    await run(
      () => importCsv.mutateAsync(file),
      (result) => `Imported ${result.imported} · skipped ${result.skipped}`,
    )
    if (fileInput.current) fileInput.current.value = ''
  }

  return (
    <>
      <PageHeader
        title="Transactions"
        description="Every movement across your accounts."
        actions={
          <>
            <Button
              variant="secondary"
              onClick={() => void exportCsv()}
              icon={<Icon name="download" className="size-4" />}
            >
              Export CSV
            </Button>
            <Button
              variant="secondary"
              loading={importCsv.isPending}
              onClick={() => fileInput.current?.click()}
              icon={<Icon name="upload" className="size-4" />}
            >
              Import CSV
            </Button>
            <input
              ref={fileInput}
              type="file"
              accept=".csv,text/csv"
              className="hidden"
              aria-label="Import transactions CSV"
              onChange={(event) => void onFilePicked(event.target.files?.[0])}
            />
            <Button
              onClick={editor.openCreate}
              icon={<Icon name="plus" className="size-4" />}
            >
              Add transaction
            </Button>
          </>
        }
      />

      <Tabs items={TABS} active={tab} onChange={setTab} />

      {tab === 'recurring' ? (
        <RecurringSection lookups={lookups} currency={currency} />
      ) : (
        <Card>
          <TransactionFilterBar
            filters={filters}
            lookups={lookups}
            onChange={patchFilters}
            onReset={resetFilters}
          />

          <DataState
            isLoading={query.isLoading}
            isError={query.isError}
            error={query.error}
            isEmpty={items.length === 0}
            onRetry={() => void query.refetch()}
            skeleton={<SkeletonRows rows={6} className="p-4" />}
            empty={{
              icon: 'transactions',
              title: 'No transactions found',
              description:
                'Try widening your filters, or record your first transaction.',
              action: (
                <Button size="sm" onClick={editor.openCreate}>
                  Add transaction
                </Button>
              ),
            }}
          >
            <div className="overflow-x-auto">
              <table className="w-full min-w-[720px] border-collapse text-left">
                <thead>
                  <tr className="border-b border-slate-200 dark:border-slate-800">
                    {COLUMNS.map((column, index) => (
                      <th
                        key={column || `col-${index}`}
                        scope="col"
                        className={`px-4 py-2.5 text-xs font-medium text-slate-500 dark:text-slate-400 ${
                          column === 'Amount' ? 'text-right' : ''
                        }`}
                      >
                        {column}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {items.map((transaction) => (
                    <TransactionRow
                      key={transaction.id}
                      transaction={transaction}
                      currency={currency}
                      lookups={lookups}
                      onEdit={editor.openEdit}
                      onDelete={confirm.request}
                    />
                  ))}
                </tbody>
              </table>
            </div>
            <Pagination
              page={page}
              pageSize={PAGE_SIZE}
              total={total}
              onPageChange={setPage}
            />
          </DataState>
        </Card>
      )}

      {editor.isOpen ? (
        <TransactionForm
          open
          transaction={editor.editing}
          lookups={lookups}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Delete transaction"
        message={`Delete “${confirm.target?.description ?? ''}”? This cannot be undone.`}
        loading={remove.isPending}
        onConfirm={() => void confirmDelete()}
        onCancel={confirm.cancel}
      />
    </>
  )
}
