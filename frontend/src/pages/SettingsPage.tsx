import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import {
  useCategories,
  useCreateCategory,
  useDeleteCategory,
  useUpdateCategory,
} from '@/api/queries'
import { auth as authApi } from '@/api/endpoints'
import {
  CategoryForm,
  CategoryIcon,
} from '@/components/settings/CategoryForm'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { DataState } from '@/components/ui/DataState'
import { Input, Select } from '@/components/ui/Field'
import { Icon } from '@/components/ui/Icon'
import { PageHeader } from '@/components/ui/PageHeader'
import { SkeletonRows } from '@/components/ui/Skeleton'
import { useAuth } from '@/context/auth-context'
import { useTheme } from '@/context/theme-context'
import { useConfirm, useEditor } from '@/hooks/useEditor'
import { useToastAction } from '@/hooks/useToastAction'
import { BACKEND_LABELS, getBackend } from '@/lib/backend'
import { CURRENCIES } from '@/lib/currencies'
import { requiredString } from '@/lib/validation'
import type { Category, CategoryInput } from '@/types'
import { z } from 'zod'

const profileSchema = z.object({
  name: requiredString('Name is required').max(80, 'Name is too long'),
  currency: requiredString('Currency is required'),
})

type ProfileValues = z.infer<typeof profileSchema>

function ProfileCard() {
  const { user, setUser } = useAuth()
  const run = useToastAction()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: {
      name: user?.name ?? '',
      currency: user?.currency ?? 'USD',
    },
  })

  const submit = handleSubmit(async (values) => {
    const updated = await run(
      () => authApi.updateMe(values),
      'Profile updated',
    )
    if (updated) setUser(updated)
  })

  return (
    <Card>
      <CardHeader
        title="Profile"
        subtitle="Your display name and the currency used across the app"
      />
      <form onSubmit={submit} noValidate className="grid gap-4 p-5 sm:grid-cols-2">
        <Input
          label="Name"
          error={errors.name?.message}
          {...register('name')}
        />
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
        <div className="sm:col-span-2">
          <Button type="submit" loading={isSubmitting}>
            Save profile
          </Button>
        </div>
      </form>
    </Card>
  )
}

function AppearanceCard() {
  const { theme, toggleTheme } = useTheme()
  return (
    <Card>
      <CardHeader
        title="Appearance"
        subtitle="Your choice is remembered on this device"
      />
      <div className="flex items-center justify-between gap-4 p-5">
        <div>
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
            {theme === 'dark' ? 'Dark mode' : 'Light mode'}
          </p>
          <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            Switch between the light and dark palettes.
          </p>
        </div>
        <Button
          variant="secondary"
          onClick={toggleTheme}
          icon={<Icon name={theme === 'dark' ? 'sun' : 'moon'} className="size-4" />}
        >
          Switch to {theme === 'dark' ? 'light' : 'dark'}
        </Button>
      </div>
    </Card>
  )
}

function BackendCard() {
  const backend = getBackend()
  return (
    <Card>
      <CardHeader
        title="API backend"
        subtitle="Picked on the sign-in screen and remembered on this device"
      />
      <div className="flex items-center justify-between gap-4 p-5">
        <div>
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">
            {BACKEND_LABELS[backend]}
          </p>
          <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            Requests from this browser go to the {BACKEND_LABELS[backend]} API.
          </p>
        </div>
        <Badge tone="info">{BACKEND_LABELS[backend]}</Badge>
      </div>
    </Card>
  )
}

function CategoryList({
  title,
  subtitle,
  categories,
  onEdit,
  onDelete,
}: {
  title: string
  subtitle: string
  categories: Category[]
  onEdit?: (category: Category) => void
  onDelete?: (category: Category) => void
}) {
  if (categories.length === 0) {
    return (
      <div className="px-5 py-4">
        <p className="text-xs font-medium text-slate-600 dark:text-slate-300">
          {title}
        </p>
        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
          {subtitle}
        </p>
      </div>
    )
  }

  return (
    <div className="px-5 py-4">
      <p className="text-xs font-medium text-slate-600 dark:text-slate-300">
        {title}
      </p>
      <ul className="mt-3 grid gap-2 sm:grid-cols-2">
        {categories.map((category) => (
          <li
            key={category.id}
            className="flex items-center justify-between gap-2 rounded-lg border border-slate-200 px-3 py-2 dark:border-slate-800"
          >
            <span className="flex min-w-0 items-center gap-2">
              <span
                className="grid size-7 shrink-0 place-items-center rounded-md text-white"
                style={{ backgroundColor: category.color || '#94a3b8' }}
              >
                <CategoryIcon name={category.icon} className="size-3.5" />
              </span>
              <span className="truncate text-sm text-slate-900 dark:text-slate-100">
                {category.name}
              </span>
              <Badge tone={category.type === 'income' ? 'success' : 'danger'}>
                {category.type}
              </Badge>
            </span>
            {onEdit && onDelete ? (
              <span className="flex gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Edit ${category.name}`}
                  onClick={() => onEdit(category)}
                >
                  <Icon name="edit" className="size-4" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Delete ${category.name}`}
                  onClick={() => onDelete(category)}
                  className="text-rose-600 dark:text-rose-400"
                >
                  <Icon name="trash" className="size-4" />
                </Button>
              </span>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  )
}

function CategoriesCard() {
  const query = useCategories()
  const editor = useEditor<Category>()
  const confirm = useConfirm<Category>()
  const run = useToastAction()

  const create = useCreateCategory()
  const update = useUpdateCategory()
  const remove = useDeleteCategory()

  const categories = query.data ?? []
  const defaults = categories.filter((category) => category.isDefault)
  const custom = categories.filter((category) => !category.isDefault)

  const save = async (input: CategoryInput) => {
    const editing = editor.editing
    const result = await run(
      () =>
        editing
          ? update.mutateAsync({ id: editing.id, body: input })
          : create.mutateAsync(input),
      editing ? 'Category updated' : 'Category added',
    )
    if (result) editor.close()
  }

  const confirmDelete = async () => {
    const target = confirm.target
    if (!target) return
    await run(() => remove.mutateAsync(target.id), 'Category deleted')
    confirm.cancel()
  }

  return (
    <Card>
      <CardHeader
        title="Categories"
        subtitle="Built-in categories are read-only; yours can be edited"
        action={
          <Button
            size="sm"
            onClick={editor.openCreate}
            icon={<Icon name="plus" className="size-4" />}
          >
            New category
          </Button>
        }
      />
      <DataState
        isLoading={query.isLoading}
        isError={query.isError}
        error={query.error}
        isEmpty={categories.length === 0}
        onRetry={() => void query.refetch()}
        skeleton={<SkeletonRows rows={4} className="p-5" />}
        empty={{
          icon: 'budgets',
          title: 'No categories yet',
          description: 'Add one to start classifying your transactions.',
        }}
      >
        <div className="divide-y divide-slate-100 dark:divide-slate-800">
          <CategoryList
            title="Built-in"
            subtitle="No built-in categories were returned."
            categories={defaults}
          />
          <CategoryList
            title="Your categories"
            subtitle="You have not added any custom categories yet."
            categories={custom}
            onEdit={editor.openEdit}
            onDelete={confirm.request}
          />
        </div>
      </DataState>

      {editor.isOpen ? (
        <CategoryForm
          open
          category={editor.editing}
          onClose={editor.close}
          onSubmit={save}
        />
      ) : null}

      <ConfirmDialog
        open={confirm.target !== null}
        title="Delete category"
        message={`Delete “${confirm.target?.name ?? ''}”? Transactions keep their history but lose this label.`}
        loading={remove.isPending}
        onConfirm={() => void confirmDelete()}
        onCancel={confirm.cancel}
      />
    </Card>
  )
}

export function SettingsPage() {
  return (
    <>
      <PageHeader
        title="Settings"
        description="Profile, appearance, API backend and your category list."
      />
      <div className="grid gap-4 xl:grid-cols-2">
        <ProfileCard />
        <AppearanceCard />
        <BackendCard />
      </div>
      <CategoriesCard />
    </>
  )
}
