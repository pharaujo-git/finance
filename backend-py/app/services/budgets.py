"""Budgets and the month's spend they are measured against."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any

from app.core.errors import conflict, not_found, validation
from app.domain.dates import (
    MONTH_FORMAT_MESSAGE,
    add_months,
    month_from,
    parse_month,
    try_parse_month,
)
from app.domain.enums import TransactionType
from app.domain.money import ZERO, round_money
from app.repositories.budgets import Budget, BudgetRepository
from app.repositories.transactions import TransactionRepository
from app.services.categories import CategoryService

BUDGET_ENTITY = "Budget"
DUPLICATE_BUDGET_MESSAGE = "A budget already exists for that category and month."


def budget_dto(budget: Budget, spent: Decimal) -> dict[str, Any]:
    return {
        "id": budget.id,
        "categoryId": budget.category_id,
        "month": budget.month,
        "limit": budget.limit,
        "spent": spent,
        # Allowed to go negative: an overspent budget should show how far over.
        "remaining": budget.limit - spent,
    }


def compare_guid(value: uuid.UUID) -> tuple[int, int, int, bytes]:
    """A sort key matching .NET's Guid.CompareTo, which is not byte order.

    The first three groups compare as *signed* integers, so a uuid starting
    0x80 sorts before one starting 0x7f. Reproduced because it decides the
    order budgets come back in.
    """
    raw = value.bytes
    return (
        int.from_bytes(raw[0:4], "big", signed=True),
        int.from_bytes(raw[4:6], "big", signed=True),
        int.from_bytes(raw[6:8], "big", signed=True),
        raw[8:16],
    )


class BudgetService:
    def __init__(
        self,
        budgets: BudgetRepository,
        transactions: TransactionRepository,
        categories: CategoryService,
    ) -> None:
        self._budgets = budgets
        self._transactions = transactions
        self._categories = categories

    async def list_all(self, user_id: uuid.UUID, month: str | None) -> list[dict[str, Any]]:
        key = month if month is not None else month_from(datetime.now(UTC))
        if month is not None and try_parse_month(month) is None:
            # An explicitly supplied month that is not a real one is a 400;
            # note an *absent* key means "this month", but "?month=" does not.
            raise validation(MONTH_FORMAT_MESSAGE)

        start = parse_month(key)
        budgets = await self._budgets.list_for_month(user_id, key)
        if not budgets:
            # Nothing to measure, so skip the spend query entirely.
            return []

        spent = await self._spent_by_category(user_id, start)
        ordered = sorted(budgets, key=lambda budget: compare_guid(budget.category_id))
        return [budget_dto(budget, spent.get(budget.category_id, ZERO)) for budget in ordered]

    async def create(
        self, user_id: uuid.UUID, *, category_id: uuid.UUID, month: str, limit: Decimal
    ) -> dict[str, Any]:
        start = parse_month(month)
        await self._categories.ensure_usable(user_id, category_id)

        if await self._budgets.exists(user_id, category_id, month):
            raise conflict(DUPLICATE_BUDGET_MESSAGE)

        budget = Budget(
            id=uuid.uuid4(),
            user_id=user_id,
            category_id=category_id,
            month=month,
            limit=round_money(limit),
        )
        await self._budgets.add(budget)

        spent = await self._spent_by_category(user_id, start)
        return budget_dto(budget, spent.get(category_id, ZERO))

    async def update(
        self, user_id: uuid.UUID, budget_id: uuid.UUID, limit: Decimal
    ) -> dict[str, Any]:
        """Only the limit moves; the category and month are immutable."""
        budget = await self._load(user_id, budget_id)
        budget.limit = round_money(limit)
        await self._budgets.update_limit(user_id, budget_id, budget.limit)

        spent = await self._spent_by_category(user_id, parse_month(budget.month))
        return budget_dto(budget, spent.get(budget.category_id, ZERO))

    async def delete(self, user_id: uuid.UUID, budget_id: uuid.UUID) -> None:
        if not await self._budgets.delete(user_id, budget_id):
            raise not_found(BUDGET_ENTITY)

    async def _load(self, user_id: uuid.UUID, budget_id: uuid.UUID) -> Budget:
        budget = await self._budgets.get(user_id, budget_id)
        if budget is None:
            raise not_found(BUDGET_ENTITY)
        return budget

    async def _spent_by_category(
        self, user_id: uuid.UUID, start: datetime
    ) -> dict[uuid.UUID, Decimal]:
        """Expenses in [start, start+1 month). Income and transfers never count."""
        end = add_months(start, 1)
        slices = await self._transactions.slices(user_id, start, None)

        totals: dict[uuid.UUID, Decimal] = {}
        for item in slices:
            if item.type is not TransactionType.EXPENSE:
                continue
            # Uncategorized spending is not measured against any budget.
            if item.category_id is None:
                continue
            if item.date >= end:
                continue
            totals[item.category_id] = totals.get(item.category_id, ZERO) + item.amount
        return totals
