"""How a transaction moves an account balance and the net worth.

Amounts are always stored positive; the type carries the sign. A transfer
debits its source account and credits its destination, which is why the
destination check has to come first -- the destination is not the row's
"AccountId".
"""

from __future__ import annotations

import uuid
from decimal import Decimal

from app.domain.enums import TransactionType
from app.domain.money import ZERO
from app.repositories.accounts import Account
from app.repositories.transactions import TransactionSlice


def delta_for(account_id: uuid.UUID, item: TransactionSlice) -> Decimal:
    """What one transaction does to one account's balance."""
    # The receiving side of a transfer, which the row records as the *transfer*
    # account rather than the owning one.
    if (
        item.type is TransactionType.TRANSFER
        and item.transfer_account_id is not None
        and item.transfer_account_id == account_id
    ):
        return item.amount

    if item.account_id != account_id:
        return ZERO

    if item.type is TransactionType.INCOME:
        return item.amount
    # An expense and the paying side of a transfer both debit.
    return -item.amount


def net_worth_delta(item: TransactionSlice) -> Decimal:
    """What one transaction does to the total. A transfer moves money, so zero."""
    if item.type is TransactionType.INCOME:
        return item.amount
    if item.type is TransactionType.EXPENSE:
        return -item.amount
    return ZERO


def balance_of(account: Account, slices: list[TransactionSlice]) -> Decimal:
    """The opening balance plus every movement. Deliberately not rounded."""
    total = ZERO
    for item in slices:
        total += delta_for(account.id, item)
    return account.initial_balance + total
