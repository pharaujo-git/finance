/** End-to-end tests through the Express app against a real Postgres schema. */

import { randomUUID } from 'node:crypto'
import request from 'supertest'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { DEMO_PASSWORD, createHarness, databaseUrl, type Harness } from './harness.js'

const MISSING = '00000000-0000-0000-0000-000000000000'

/** supertest types `body` as any; name the shape where a test iterates it. */
function rows<T>(response: { body: unknown }): T[] {
  return response.body as T[]
}

// Without a database every case here would be meaningless, so the whole file
// skips rather than pretending to pass.
const suite = databaseUrl() === null ? describe.skip : describe

suite('API', () => {
  let harness: Harness
  let auth: { Authorization: string }

  const api = () => request(harness.app)

  async function registerUser(email: string): Promise<string> {
    const response = await api()
      .post('/api/auth/register')
      .send({ email, password: DEMO_PASSWORD, name: 'Harness' })
    expect(response.status).toBe(200)
    return response.body.token as string
  }

  async function makeAccount(overrides: Record<string, unknown> = {}) {
    const response = await api()
      .post('/api/accounts')
      .set(auth)
      .send({ name: 'Checking', type: 'checking', initialBalance: 1000, ...overrides })
    expect(response.status).toBe(200)
    return response.body
  }

  async function makeCategory() {
    const response = await api()
      .post('/api/categories')
      .set(auth)
      .send({ name: 'Food', type: 'expense', icon: 'utensils', color: '#ef4444' })
    expect(response.status).toBe(200)
    return response.body
  }

  beforeEach(async () => {
    harness = await createHarness()
    auth = { Authorization: `Bearer ${await registerUser('harness@test.dev')}` }
  })

  afterEach(async () => {
    await harness.close()
  })

  describe('probes', () => {
    it('answers health as plain text', async () => {
      const response = await api().get('/health')
      expect(response.status).toBe(200)
      expect(response.text).toBe('ok')
    })

    it('names this backend in the service document', async () => {
      const response = await api().get('/')
      expect(response.body).toEqual({
        service: 'FinanceTracker API (Node)',
        status: 'ok',
        docs: '/swagger',
      })
    })
  })

  describe('auth', () => {
    it('normalises the address and trims the name on register', async () => {
      const response = await api()
        .post('/api/auth/register')
        .send({ email: 'New@Example.COM ', password: DEMO_PASSWORD, name: ' Ada ' })

      expect(response.status).toBe(200)
      expect(response.body.user).toMatchObject({
        email: 'new@example.com',
        name: 'Ada',
        currency: 'USD',
      })
    })

    it('rejects a duplicate email', async () => {
      const response = await api()
        .post('/api/auth/register')
        .send({ email: 'harness@test.dev', password: DEMO_PASSWORD, name: 'Twin' })

      expect(response.status).toBe(409)
      expect(response.body.detail).toBe('An account with that email already exists.')
    })

    it('round-trips a login', async () => {
      const response = await api()
        .post('/api/auth/login')
        .send({ email: 'harness@test.dev', password: DEMO_PASSWORD })

      expect(response.status).toBe(200)
      expect(response.body.user.email).toBe('harness@test.dev')
    })

    it.each([
      { email: 'harness@test.dev', password: 'wrong-password' },
      { email: 'nobody@test.dev', password: DEMO_PASSWORD },
    ])('fails bad credentials indistinguishably', async (payload) => {
      const response = await api().post('/api/auth/login').send(payload)
      expect(response.status).toBe(401)
      expect(response.body.detail).toBe('Invalid email or password.')
    })

    it('requires a token for the profile', async () => {
      const response = await api().get('/api/auth/me')
      expect(response.status).toBe(401)
      expect(response.headers['www-authenticate']).toBe('Bearer')
    })

    it('rejects a forged token', async () => {
      const response = await api().get('/api/auth/me').set('Authorization', 'Bearer not.a.token')
      expect(response.status).toBe(401)
    })

    it('trims the name and upper-cases the currency on update', async () => {
      const response = await api()
        .put('/api/auth/me')
        .set(auth)
        .send({ name: ' Grace ', currency: ' eur ' })

      expect(response.status).toBe(200)
      expect(response.body).toMatchObject({ name: 'Grace', currency: 'EUR' })
    })

    it('reports every broken rule at once', async () => {
      const response = await api()
        .post('/api/auth/register')
        .send({ email: 'bad', password: 'x', name: '' })

      expect(response.status).toBe(400)
      expect(response.headers['content-type']).toContain('application/json')
      expect(response.body.errors.Email).toEqual([
        'The Email field is not a valid e-mail address.',
      ])
      expect(response.body.errors.Name).toContain('The Name field is required.')
    })
  })

  describe('accounts', () => {
    it('creates and lists', async () => {
      const account = await makeAccount()
      expect(account.balance).toBe(1000)

      const listed = await api().get('/api/accounts').set(auth)
      expect(rows<{ id: string }>(listed).map((item) => item.id)).toEqual([account.id])
    })

    it('follows the transactions in the balance', async () => {
      const account = await makeAccount()
      await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        type: 'expense',
        amount: 42.5,
        date: '2026-08-10',
        description: 'Lunch',
      })

      const refreshed = await api().get(`/api/accounts/${account.id}`).set(auth)
      expect(refreshed.body.balance).toBe(957.5)
    })

    it('moves money between two accounts on a transfer', async () => {
      const checking = await makeAccount()
      const savings = await makeAccount({ name: 'Savings', type: 'savings', initialBalance: 0 })

      await api().post('/api/transactions').set(auth).send({
        accountId: checking.id,
        type: 'transfer',
        amount: 250,
        date: '2026-08-15',
        description: 'To savings',
        transferAccountId: savings.id,
      })

      const listed = await api().get('/api/accounts').set(auth)
      const balances = Object.fromEntries(
        rows<{ name: string; balance: number }>(listed).map((item) => [item.name, item.balance]),
      )
      expect(balances.Checking).toBe(750)
      expect(balances.Savings).toBe(250)
    })

    it('archives rather than removing on delete', async () => {
      const account = await makeAccount()
      expect((await api().delete(`/api/accounts/${account.id}`).set(auth)).status).toBe(204)

      const refreshed = await api().get(`/api/accounts/${account.id}`).set(auth)
      expect(refreshed.body.isArchived).toBe(true)
    })

    it('renders an unknown account as a problem document', async () => {
      const response = await api().get(`/api/accounts/${MISSING}`).set(auth)
      expect(response.status).toBe(404)
      expect(response.headers['content-type']).toContain('application/problem+json')
      expect(response.body.detail).toBe('Account was not found.')
    })

    it('answers a bare 404 for a non-uuid path', async () => {
      const response = await api().get('/api/accounts/not-a-uuid').set(auth)
      expect(response.status).toBe(404)
      expect(response.text).toBe('')
    })

    it('short-circuits on a missing required member', async () => {
      const response = await api().post('/api/accounts').set(auth).send({ name: '' })
      expect(response.status).toBe(400)
      // Only the "$" error: no other rule runs once a member is missing.
      expect(response.body.errors).toEqual({
        $: ['The JSON payload was missing required properties, including the following: type'],
      })
    })
  })

  describe('categories', () => {
    it('shows the shared defaults', async () => {
      const listed = await api().get('/api/categories').set(auth)
      expect(rows<{ isDefault: boolean }>(listed).some((item) => item.isDefault)).toBe(true)
    })

    it('creates and updates', async () => {
      const category = await makeCategory()
      expect(category.isDefault).toBe(false)

      const updated = await api()
        .put(`/api/categories/${category.id}`)
        .set(auth)
        .send({ name: 'Dining', type: 'expense', icon: 'fork', color: '#000000' })
      expect(updated.body.name).toBe('Dining')
    })

    it('refuses to modify a default category', async () => {
      const listed = await api().get('/api/categories').set(auth)
      const shared = rows<{ isDefault: boolean; id: string }>(listed).find(
        (item) => item.isDefault,
      )!

      const response = await api().delete(`/api/categories/${shared.id}`).set(auth)
      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('Default categories cannot be modified.')
    })

    it('detaches a deleted category from its transactions', async () => {
      const account = await makeAccount()
      const category = await makeCategory()
      const created = await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        categoryId: category.id,
        type: 'expense',
        amount: 10,
        date: '2026-08-10',
        description: 'Snack',
      })

      expect((await api().delete(`/api/categories/${category.id}`).set(auth)).status).toBe(204)

      const refreshed = await api().get(`/api/transactions/${created.body.id}`).set(auth)
      expect(refreshed.body.categoryId).toBeNull()
    })
  })

  describe('transactions', () => {
    it('normalises the row it stores', async () => {
      const account = await makeAccount()
      const response = await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        type: 'expense',
        amount: 42.505,
        date: '2026-08-10',
        description: '  Lunch  ',
        notes: '   ',
        tags: [' food ', '', 'out'],
      })

      expect(response.body.amount).toBe(42.51) // rounded half away from zero
      expect(response.body.description).toBe('Lunch')
      expect(response.body.notes).toBeNull() // whitespace collapses to null
      expect(response.body.tags).toEqual(['food', 'out'])
    })

    it('pages and filters the search', async () => {
      const account = await makeAccount()
      for (let index = 0; index < 3; index += 1) {
        await api().post('/api/transactions').set(auth).send({
          accountId: account.id,
          type: 'expense',
          amount: 5 + index,
          date: `2026-08-1${index}`,
          description: `Item ${index}`,
        })
      }

      const page = await api().get('/api/transactions?page=1&pageSize=2').set(auth)
      expect(page.body).toMatchObject({ total: 3, page: 1, pageSize: 2 })
      expect(page.body.items).toHaveLength(2)

      const found = await api().get('/api/transactions?search=item%201').set(auth)
      expect(found.body.total).toBe(1)
    })

    it('requires a different destination for a transfer', async () => {
      const account = await makeAccount()
      const payload = {
        accountId: account.id,
        type: 'transfer',
        amount: 10,
        date: '2026-08-10',
        description: 'Nowhere',
      }

      const missing = await api().post('/api/transactions').set(auth).send(payload)
      expect(missing.status).toBe(400)
      expect(missing.body.detail).toBe('A transfer requires a destination account.')

      const same = await api()
        .post('/api/transactions')
        .set(auth)
        .send({ ...payload, transferAccountId: account.id })
      expect(same.body.detail).toBe('A transfer must use two different accounts.')
    })

    it('updates and deletes', async () => {
      const account = await makeAccount()
      const created = await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        type: 'expense',
        amount: 10,
        date: '2026-08-10',
        description: 'Before',
      })

      const updated = await api()
        .put(`/api/transactions/${created.body.id}`)
        .set(auth)
        .send({
          accountId: account.id,
          type: 'income',
          amount: 20,
          date: '2026-08-11',
          description: 'After',
        })
      expect(updated.body).toMatchObject({ description: 'After', type: 'income' })

      expect((await api().delete(`/api/transactions/${created.body.id}`).set(auth)).status).toBe(204)
      expect((await api().get(`/api/transactions/${created.body.id}`).set(auth)).status).toBe(404)
    })

    it('round-trips through CSV', async () => {
      const account = await makeAccount()
      const category = await makeCategory()
      await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        categoryId: category.id,
        type: 'expense',
        amount: 12.5,
        date: '2026-08-10',
        description: 'Groceries, bulk',
      })

      const exported = await api().get('/api/transactions/export').set(auth)
      expect(exported.status).toBe(200)
      expect(exported.headers['content-type']).toContain('text/csv')
      expect(exported.headers['content-disposition']).toContain(
        'attachment; filename=transactions-',
      )

      const lines = exported.text.split('\n')
      expect(lines[0]).toBe('Date,Type,Amount,Account,Category,Description,Notes,Tags')
      // The description carries a comma, so it must come back quoted, and the
      // CSV column always writes two places.
      expect(lines[1]).toContain('"Groceries, bulk"')
      expect(lines[1]).toContain(',12.50,')

      const imported = await api()
        .post('/api/transactions/import')
        .set(auth)
        .attach('file', Buffer.from(exported.text), 't.csv')
      expect(imported.body).toEqual({ imported: 1, skipped: 0 })
    })

    it('rejects an empty upload', async () => {
      const response = await api()
        .post('/api/transactions/import')
        .set(auth)
        .attach('file', Buffer.from(''), 't.csv')

      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('A non-empty CSV file is required.')
    })

    it('skips unusable rows on import', async () => {
      await makeAccount()
      const csv = [
        'Date,Type,Amount,Account,Category,Description,Notes,Tags',
        '2026-08-10,expense,10.00,Checking,,Good row,,',
        'not-a-date,expense,10.00,Checking,,Bad date,,',
        '2026-08-10,transfer,10.00,Checking,,Transfers never import,,',
        '2026-08-10,expense,10.00,No Such Account,,Unknown account,,',
        '',
      ].join('\n')

      const response = await api()
        .post('/api/transactions/import')
        .set(auth)
        .attach('file', Buffer.from(csv), 't.csv')
      expect(response.body).toEqual({ imported: 1, skipped: 3 })
    })
  })

  describe('budgets', () => {
    it('measures the month spend', async () => {
      const account = await makeAccount()
      const category = await makeCategory()
      await api().post('/api/transactions').set(auth).send({
        accountId: account.id,
        categoryId: category.id,
        type: 'expense',
        amount: 30,
        date: '2026-08-10',
        description: 'Food',
      })

      const created = await api()
        .post('/api/budgets')
        .set(auth)
        .send({ categoryId: category.id, month: '2026-08', limit: 100 })

      expect(created.status).toBe(200)
      expect(created.body.spent).toBe(30)
      expect(created.body.remaining).toBe(70)
    })

    it('rejects a duplicate', async () => {
      const category = await makeCategory()
      const payload = { categoryId: category.id, month: '2026-08', limit: 100 }
      expect((await api().post('/api/budgets').set(auth).send(payload)).status).toBe(200)

      const clash = await api().post('/api/budgets').set(auth).send(payload)
      expect(clash.status).toBe(409)
      expect(clash.body.detail).toBe('A budget already exists for that category and month.')
    })

    it.each(['nope', '2026-13'])('rejects the month %s', async (month) => {
      const category = await makeCategory()
      const response = await api()
        .post('/api/budgets')
        .set(auth)
        .send({ categoryId: category.id, month, limit: 100 })
      expect(response.status).toBe(400)
    })

    it('moves only the limit on update', async () => {
      const category = await makeCategory()
      const created = await api()
        .post('/api/budgets')
        .set(auth)
        .send({ categoryId: category.id, month: '2026-08', limit: 100 })

      const updated = await api()
        .put(`/api/budgets/${created.body.id}`)
        .set(auth)
        .send({ limit: 250 })
      expect(updated.body).toMatchObject({ limit: 250, month: '2026-08' })
    })
  })

  describe('goals', () => {
    it('creates and contributes', async () => {
      const created = await api()
        .post('/api/goals')
        .set(auth)
        .send({ name: 'Trip', targetAmount: 2000, currentAmount: 100 })
      expect(created.body.currentAmount).toBe(100)

      const after = await api()
        .post(`/api/goals/${created.body.id}/contribute`)
        .set(auth)
        .send({ amount: 50.5 })
      expect(after.body.currentAmount).toBe(150.5)
    })

    it('lets a goal exceed its target', async () => {
      const created = await api()
        .post('/api/goals')
        .set(auth)
        .send({ name: 'Small', targetAmount: 10 })

      const after = await api()
        .post(`/api/goals/${created.body.id}/contribute`)
        .set(auth)
        .send({ amount: 999 })
      expect(after.body.currentAmount).toBe(999)
    })

    it('rejects a zero contribution', async () => {
      const created = await api()
        .post('/api/goals')
        .set(auth)
        .send({ name: 'Trip', targetAmount: 100 })

      const response = await api()
        .post(`/api/goals/${created.body.id}/contribute`)
        .set(auth)
        .send({ amount: 0 })
      expect(response.status).toBe(400)
    })
  })

  describe('recurring', () => {
    it('starts on the start date', async () => {
      const account = await makeAccount()
      const created = await api().post('/api/recurring').set(auth).send({
        accountId: account.id,
        type: 'expense',
        amount: 9.99,
        description: 'Streaming',
        frequency: 'monthly',
        startDate: '2026-09-01',
      })

      expect(created.status).toBe(200)
      expect(created.body.nextRunDate).toContain('2026-09-01')
      expect(created.body.isActive).toBe(true)
    })

    it('does not support transfers', async () => {
      const account = await makeAccount()
      const response = await api().post('/api/recurring').set(auth).send({
        accountId: account.id,
        type: 'transfer',
        amount: 10,
        description: 'Nope',
        frequency: 'monthly',
        startDate: '2026-09-01',
      })

      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('Recurring transfers are not supported.')
    })

    it('refuses an end date before the start', async () => {
      const account = await makeAccount()
      const response = await api().post('/api/recurring').set(auth).send({
        accountId: account.id,
        type: 'expense',
        amount: 10,
        description: 'Backwards',
        frequency: 'monthly',
        startDate: '2026-09-01',
        endDate: '2026-08-01',
      })

      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('End date must not be before the start date.')
    })
  })

  describe('analytics', () => {
    beforeEach(async () => {
      const account = await makeAccount()
      const category = await makeCategory()
      for (const payload of [
        { type: 'income', amount: 3000, date: '2026-08-01', description: 'Salary' },
        {
          type: 'expense',
          amount: 750,
          date: '2026-08-05',
          description: 'Rent',
          categoryId: category.id,
        },
      ]) {
        await api()
          .post('/api/transactions')
          .set(auth)
          .send({ accountId: account.id, ...payload })
      }
    })

    it('returns the summary shape', async () => {
      const response = await api().get('/api/dashboard/summary').set(auth)
      expect(Object.keys(response.body).sort()).toEqual([
        'netWorth',
        'savingsRate',
        'totalExpenses',
        'totalIncome',
      ])
    })

    it('returns a window of the requested length', async () => {
      const response = await api().get('/api/dashboard/networth?months=3').set(auth)
      expect(response.body).toHaveLength(3)
      expect(Object.keys(response.body[0]).sort()).toEqual(['month', 'value'])
    })

    it('clamps the window rather than rejecting it', async () => {
      expect((await api().get('/api/dashboard/networth?months=0').set(auth)).body).toHaveLength(1)
      expect((await api().get('/api/dashboard/networth?months=999').set(auth)).body).toHaveLength(
        120,
      )
    })

    it('rejects a non-numeric months value', async () => {
      const response = await api().get('/api/dashboard/networth?months=abc').set(auth)
      expect(response.status).toBe(400)
      expect(response.body.errors.months).toEqual(["The value 'abc' is not valid for months."])
    })

    it('buckets the cashflow', async () => {
      const response = await api().get('/api/dashboard/cashflow?months=2').set(auth)
      expect(response.body).toHaveLength(2)
      expect(Object.keys(response.body[0]).sort()).toEqual(['expenses', 'income', 'month'])
    })

    it('sorts spending descending', async () => {
      const response = await api().get('/api/dashboard/spending?month=2026-08').set(auth)
      const amounts = rows<{ amount: number }>(response).map((row) => row.amount)
      expect(amounts).toEqual([...amounts].sort((a: number, b: number) => b - a))
    })

    it('rejects a bad spending month', async () => {
      const response = await api().get('/api/dashboard/spending?month=nope').set(auth)
      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('Month must be in YYYY-MM format.')
    })

    it('always returns twelve months in the yearly report', async () => {
      const response = await api().get('/api/reports/monthly?year=2026').set(auth)
      expect(response.body).toHaveLength(12)
      expect(response.body[0].month).toBe('2026-01')
      expect(response.body[11].month).toBe('2026-12')
    })

    it.each([1800, 10000])('rejects the out-of-range year %i', async (year) => {
      const response = await api().get(`/api/reports/monthly?year=${year}`).set(auth)
      expect(response.status).toBe(400)
      expect(response.body.detail).toBe('Year must be between 1900 and 9999.')
    })

    it('returns the category report shape', async () => {
      const response = await api().get('/api/reports/categories').set(auth)
      expect(Object.keys(response.body[0]).sort()).toEqual([
        'amount',
        'categoryId',
        'categoryName',
        'color',
        'type',
      ])
    })
  })

  describe('isolation', () => {
    it('hides one user\'s data from another', async () => {
      const account = await makeAccount()
      const otherToken = await registerUser(`other-${randomUUID().slice(0, 8)}@test.dev`)
      const otherAuth = { Authorization: `Bearer ${otherToken}` }

      expect((await api().get('/api/accounts').set(otherAuth)).body).toEqual([])
      // And the first user's account is invisible, not merely absent.
      expect((await api().get(`/api/accounts/${account.id}`).set(otherAuth)).status).toBe(404)
    })
  })
})
