/**
 * How a transaction moves an account balance and the net worth.
 *
 * Amounts are always stored positive; the type carries the sign. A transfer
 * debits its source and credits its destination, which is why the destination
 * check comes first -- the destination is not the row's "AccountId".
 */

import { TransactionType } from '../domain/enums.js'
import { MONEY_ZERO, type Money } from '../domain/money.js'
import type { Account, TransactionSlice } from '../repositories/index.js'

/** What one transaction does to one account's balance. */
export function deltaFor(accountId: string, item: TransactionSlice): Money {
  // The receiving side of a transfer, which the row records as the *transfer*
  // account rather than the owning one.
  if (
    item.type.equals(TransactionType.Transfer) &&
    item.transferAccountId !== null &&
    item.transferAccountId === accountId
  ) {
    return item.amount
  }

  if (item.accountId !== accountId) return MONEY_ZERO
  if (item.type.equals(TransactionType.Income)) return item.amount

  // An expense and the paying side of a transfer both debit.
  return item.amount.negate()
}

/** What one transaction does to the total. A transfer moves money, so zero. */
export function netWorthDelta(item: TransactionSlice): Money {
  if (item.type.equals(TransactionType.Income)) return item.amount
  if (item.type.equals(TransactionType.Expense)) return item.amount.negate()
  return MONEY_ZERO
}

/** The opening balance plus every movement. Deliberately not rounded. */
export function balanceOf(account: Account, slices: readonly TransactionSlice[]): Money {
  let total = MONEY_ZERO
  for (const item of slices) {
    total = total.add(deltaFor(account.id, item))
  }
  return account.initialBalance.add(total)
}
