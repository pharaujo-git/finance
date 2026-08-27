"""Transactions: the write path, and the paged search behind the grid."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from typing import Any

from app.api.schemas import TransactionRequest
from app.core.errors import ValidationError, not_found, validation
from app.domain import validation as rules
from app.domain.enums import TransactionType
from app.domain.money import as_utc, join_tags, round_money, split_tags, trimmed_or_none
from app.repositories.accounts import AccountRepository
from app.repositories.transactions import (
    Transaction,
    TransactionFilter,
    TransactionRepository,
)
from app.services.accounts import ACCOUNT_ENTITY
from app.services.categories import CategoryService

TRANSACTION_ENTITY = "Transaction"
TRANSFER_TARGET_MESSAGE = "A transfer requires a destination account."
TRANSFER_SAME_ACCOUNT_MESSAGE = "A transfer must use two different accounts."

DEFAULT_PAGE = 1
DEFAULT_PAGE_SIZE = 20
MAX_PAGE_SIZE = 200
MAX_INT32 = 2147483647


def transaction_dto(item: Transaction) -> dict[str, Any]:
    return {
        "id": item.id,
        "accountId": item.account_id,
        "categoryId": item.category_id,
        "type": item.type,
        "amount": item.amount,
        "date": item.date,
        "description": item.description,
        "notes": item.notes,
        "tags": item.tags,
        "transferAccountId": item.transfer_account_id,
    }


@dataclass(slots=True)
class TransactionQuery:
    page: int = DEFAULT_PAGE
    page_size: int = DEFAULT_PAGE_SIZE
    account_id: uuid.UUID | None = None
    category_id: uuid.UUID | None = None
    type: TransactionType | None = None
    date_from: datetime | None = None
    date_to: datetime | None = None
    search: str = ""

    def validate(self) -> None:
        errs = ValidationError()
        rules.int_range(errs, rules.FIELD_PAGE, self.page, 1, MAX_INT32)
        rules.int_range(errs, rules.FIELD_PAGE_SIZE, self.page_size, 1, MAX_PAGE_SIZE)
        rules.max_length(errs, rules.FIELD_SEARCH, self.search, rules.SEARCH_MAX_LENGTH)
        rules.raise_if_any(errs)


class TransactionService:
    def __init__(
        self,
        transactions: TransactionRepository,
        accounts: AccountRepository,
        categories: CategoryService,
    ) -> None:
        self._transactions = transactions
        self._accounts = accounts
        self._categories = categories

    async def search(self, user_id: uuid.UUID, query: TransactionQuery) -> dict[str, Any]:
        query.validate()
        filters = TransactionFilter(
            account_id=query.account_id,
            category_id=query.category_id,
            type=query.type,
            date_from=as_utc(query.date_from) if query.date_from else None,
            date_to=as_utc(query.date_to) if query.date_to else None,
            search=query.search.strip().lower(),
            limit=query.page_size,
            offset=(query.page - 1) * query.page_size,
        )
        items, total = await self._transactions.search(user_id, filters)
        return {
            "items": [transaction_dto(item) for item in items],
            "total": total,
            "page": query.page,
            "pageSize": query.page_size,
        }

    async def get(self, user_id: uuid.UUID, transaction_id: uuid.UUID) -> dict[str, Any]:
        return transaction_dto(await self._load(user_id, transaction_id))

    async def create(self, user_id: uuid.UUID, request: TransactionRequest) -> dict[str, Any]:
        item = Transaction(
            id=uuid.uuid4(),
            user_id=user_id,
            account_id=uuid.UUID(int=0),
            category_id=None,
            type=TransactionType.EXPENSE,
            amount=Decimal(0),
            date=datetime.now(),
            description="",
            notes=None,
        )
        await self._apply(user_id, item, request)
        await self._transactions.add(item)
        return transaction_dto(item)

    async def update(
        self, user_id: uuid.UUID, transaction_id: uuid.UUID, request: TransactionRequest
    ) -> dict[str, Any]:
        item = await self._load(user_id, transaction_id)
        await self._apply(user_id, item, request)
        await self._transactions.update(item)
        return transaction_dto(item)

    async def delete(self, user_id: uuid.UUID, transaction_id: uuid.UUID) -> None:
        if not await self._transactions.delete(user_id, transaction_id):
            raise not_found(TRANSACTION_ENTITY)

    async def _apply(
        self, user_id: uuid.UUID, item: Transaction, request: TransactionRequest
    ) -> None:
        """Validates the references, then copies the request onto the row.

        The order of the checks decides which error a doubly-wrong request
        gets, so it is fixed: account, then category, then the transfer rules.
        """
        assert request.account_id is not None
        assert request.type is not None
        assert request.amount is not None
        assert request.date is not None

        await self._ensure_account(user_id, request.account_id)
        await self._categories.ensure_usable(user_id, request.category_id)

        transfer_account_id: uuid.UUID | None = None
        if request.type is TransactionType.TRANSFER:
            if request.transfer_account_id is None:
                raise validation(TRANSFER_TARGET_MESSAGE)
            if request.transfer_account_id == request.account_id:
                raise validation(TRANSFER_SAME_ACCOUNT_MESSAGE)
            await self._ensure_account(user_id, request.transfer_account_id)
            transfer_account_id = request.transfer_account_id

        item.account_id = request.account_id
        item.category_id = request.category_id
        item.type = request.type
        item.amount = round_money(request.amount)
        item.date = as_utc(request.date)
        item.description = request.description.strip()
        item.notes = trimmed_or_none(request.notes)
        # Normalised here, not just on the way to the column: the response has
        # to show the tags as stored, trimmed and without the blanks.
        item.tags = split_tags(join_tags(request.tags))
        item.transfer_account_id = transfer_account_id

    async def _ensure_account(self, user_id: uuid.UUID, account_id: uuid.UUID) -> None:
        if not await self._accounts.exists(user_id, account_id):
            raise not_found(ACCOUNT_ENTITY)

    async def _load(self, user_id: uuid.UUID, transaction_id: uuid.UUID) -> Transaction:
        item = await self._transactions.get(user_id, transaction_id)
        if item is None:
            raise not_found(TRANSACTION_ENTITY)
        return item
