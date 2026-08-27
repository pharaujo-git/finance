"""Accounts and their computed balances."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any

from app.core.errors import not_found
from app.domain.enums import AccountType
from app.domain.money import ZERO
from app.repositories.accounts import Account, AccountRepository
from app.repositories.transactions import TransactionRepository
from app.services.balance import balance_of

ACCOUNT_ENTITY = "Account"


def account_dto(account: Account, balance: Decimal) -> dict[str, Any]:
    return {
        "id": account.id,
        "name": account.name,
        "type": account.type,
        "balance": balance,
        "currency": account.currency,
        "isArchived": account.is_archived,
        "createdAt": account.created_at,
    }


class AccountService:
    def __init__(self, accounts: AccountRepository, transactions: TransactionRepository) -> None:
        self._accounts = accounts
        self._transactions = transactions

    async def list_all(self, user_id: uuid.UUID) -> list[dict[str, Any]]:
        accounts = await self._accounts.list_all(user_id)
        slices = await self._transactions.slices(user_id, None, None)
        return [account_dto(account, balance_of(account, slices)) for account in accounts]

    async def get(self, user_id: uuid.UUID, account_id: uuid.UUID) -> dict[str, Any]:
        account = await self._load(user_id, account_id)
        slices = await self._transactions.slices(user_id, None, None)
        return account_dto(account, balance_of(account, slices))

    async def create(
        self,
        user_id: uuid.UUID,
        *,
        name: str,
        account_type: AccountType,
        initial_balance: Decimal | None,
        currency: str,
    ) -> dict[str, Any]:
        account = Account(
            id=uuid.uuid4(),
            user_id=user_id,
            name=name.strip(),
            type=account_type,
            # Stored verbatim, not rounded: both other backends echo back
            # whatever scale the caller sent, and the column rounds on write.
            initial_balance=initial_balance if initial_balance is not None else ZERO,
            currency=normalize_currency(currency),
            is_archived=False,
            created_at=datetime.now(UTC),
        )
        await self._accounts.add(account)
        # A brand-new account has no transactions, so the opening balance is
        # the balance; no need to re-read them.
        return account_dto(account, account.initial_balance)

    async def update(
        self,
        user_id: uuid.UUID,
        account_id: uuid.UUID,
        *,
        name: str,
        account_type: AccountType,
        currency: str,
        is_archived: bool | None,
    ) -> dict[str, Any]:
        account = await self._load(user_id, account_id)
        account.name = name.strip()
        account.type = account_type
        account.currency = normalize_currency(currency)
        # An omitted flag un-archives, matching `request.IsArchived ?? false`.
        account.is_archived = bool(is_archived)

        await self._accounts.update(account)
        slices = await self._transactions.slices(user_id, None, None)
        return account_dto(account, balance_of(account, slices))

    async def archive(self, user_id: uuid.UUID, account_id: uuid.UUID) -> None:
        """The DELETE handler's work: flag the row so history stays intact."""
        if not await self._accounts.archive(user_id, account_id):
            raise not_found(ACCOUNT_ENTITY)

    async def _load(self, user_id: uuid.UUID, account_id: uuid.UUID) -> Account:
        account = await self._accounts.get(user_id, account_id)
        if account is None:
            raise not_found(ACCOUNT_ENTITY)
        return account


def normalize_currency(currency: str) -> str:
    return currency.strip().upper()
