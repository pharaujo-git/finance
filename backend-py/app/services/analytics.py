"""Dashboard and report aggregations.

Every window here is half-open -- [start, start + 1 month) -- so a transaction
timestamped midnight on the 1st lands in exactly one bucket.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from decimal import ROUND_HALF_UP, Decimal
from typing import Any

from app.core.errors import validation
from app.domain.dates import (
    MAX_REPORT_YEAR,
    MIN_REPORT_YEAR,
    YEAR_RANGE_MESSAGE,
    add_months,
    add_years,
    clamp_months,
    first_day_utc,
    month_from,
    parse_month,
    start_of_month,
    trailing_months,
)
from app.domain.enums import CategoryType, TransactionType
from app.domain.money import ZERO, trim_money
from app.repositories.accounts import AccountRepository
from app.repositories.transactions import TransactionRepository, TransactionSlice
from app.services.balance import net_worth_delta
from app.services.categories import CategoryService, describe

DEFAULT_NET_WORTH_MONTHS = 12
DEFAULT_CASHFLOW_MONTHS = 6

# The savings rate is a ratio, so it gets more places than money does.
_SAVINGS_RATE_SCALE = Decimal("0.0001")

# The yearly report's upper bound is inclusive in SQL, so it stops just short of
# next January rather than reaching it.
_TICK = timedelta(microseconds=1)


@dataclass(slots=True)
class _Group:
    """One category's running total, in first-seen order."""

    category_id: uuid.UUID | None
    total: Decimal = ZERO
    has_income: bool = False


def _amount_of(row: dict[str, Any]) -> Decimal:
    """Sort key for the descending charts; every row carries an amount."""
    amount = row["amount"]
    assert isinstance(amount, Decimal)
    return amount


def _bucket(slices: list[TransactionSlice], start: datetime) -> tuple[str, Decimal, Decimal]:
    """Income and expense totals for the month beginning at `start`."""
    end = add_months(start, 1)
    income = ZERO
    expenses = ZERO
    for item in slices:
        if not (start <= item.date < end):
            continue
        if item.type is TransactionType.INCOME:
            income += item.amount
        elif item.type is TransactionType.EXPENSE:
            expenses += item.amount
    return month_from(start), income, expenses


def _group_by_category(slices: list[TransactionSlice], keep: Any) -> list[_Group]:
    """Groups by category, preserving first-seen key order like LINQ's GroupBy."""
    groups: dict[uuid.UUID | None, _Group] = {}
    order: list[uuid.UUID | None] = []

    for item in slices:
        if not keep(item):
            continue
        key = item.category_id
        group = groups.get(key)
        if group is None:
            group = _Group(key)
            groups[key] = group
            order.append(key)
        group.total += item.amount
        group.has_income = group.has_income or item.type is TransactionType.INCOME

    return [groups[key] for key in order]


class AnalyticsService:
    def __init__(
        self,
        transactions: TransactionRepository,
        accounts: AccountRepository,
        categories: CategoryService,
    ) -> None:
        self._transactions = transactions
        self._accounts = accounts
        self._categories = categories

    async def summary(self, user_id: uuid.UUID, now: datetime) -> dict[str, Any]:
        slices = await self._transactions.slices(user_id, None, None)
        opening = await self._opening_balance(user_id)

        month_start = start_of_month(now)
        month_end = add_months(month_start, 1)

        income = ZERO
        expenses = ZERO
        for item in slices:
            if not (month_start <= item.date < month_end):
                continue
            if item.type is TransactionType.INCOME:
                income += item.amount
            elif item.type is TransactionType.EXPENSE:
                expenses += item.amount

        # Net worth spans all time, not just this month.
        net_worth = opening + sum((net_worth_delta(item) for item in slices), ZERO)

        if income > ZERO:
            rate = trim_money(
                ((income - expenses) / income).quantize(_SAVINGS_RATE_SCALE, rounding=ROUND_HALF_UP)
            )
        else:
            rate = ZERO

        return {
            "netWorth": net_worth,
            "totalIncome": income,
            "totalExpenses": expenses,
            "savingsRate": rate,
        }

    async def net_worth(
        self, user_id: uuid.UUID, now: datetime, months: int
    ) -> list[dict[str, Any]]:
        """A cumulative series: each point is the total as of that month's end."""
        window = trailing_months(now, clamp_months(months))
        slices = await self._transactions.slices(user_id, None, None)
        opening = await self._opening_balance(user_id)

        points = []
        for start in window:
            end = add_months(start, 1)
            value = opening + sum(
                (net_worth_delta(item) for item in slices if item.date < end), ZERO
            )
            points.append({"month": month_from(start), "value": value})
        return points

    async def cashflow(
        self, user_id: uuid.UUID, now: datetime, months: int
    ) -> list[dict[str, Any]]:
        window = trailing_months(now, clamp_months(months))
        slices = await self._transactions.slices(user_id, window[0], None)

        points = []
        for start in window:
            month, income, expenses = _bucket(slices, start)
            points.append({"month": month, "income": income, "expenses": expenses})
        return points

    async def spending(
        self, user_id: uuid.UUID, now: datetime, month: str | None
    ) -> list[dict[str, Any]]:
        start = parse_month(month) if month is not None else start_of_month(now)
        end = add_months(start, 1)

        slices = await self._transactions.slices(user_id, start, None)
        lookup = await self._categories.lookup(user_id)

        groups = _group_by_category(
            slices, lambda item: item.type is TransactionType.EXPENSE and item.date < end
        )

        result = [
            {
                "categoryId": group.category_id,
                "categoryName": describe(lookup, group.category_id).name,
                "color": describe(lookup, group.category_id).color,
                "amount": group.total,
            }
            for group in groups
        ]
        # Stable, so ties keep the order the transactions came back in.
        result.sort(key=_amount_of, reverse=True)
        return result

    async def monthly_report(self, user_id: uuid.UUID, year: int) -> list[dict[str, Any]]:
        if year < MIN_REPORT_YEAR or year > MAX_REPORT_YEAR:
            raise validation(YEAR_RANGE_MESSAGE)

        start = first_day_utc(year, 1)
        # The SQL bound is inclusive, so stop just short of next January.
        end = add_years(start, 1) - _TICK

        slices = await self._transactions.slices(user_id, start, end)

        report = []
        for offset in range(12):
            month, income, expenses = _bucket(slices, add_months(start, offset))
            report.append(
                {
                    "month": month,
                    "income": income,
                    "expenses": expenses,
                    "net": income - expenses,
                }
            )
        return report

    async def category_report(
        self, user_id: uuid.UUID, date_from: datetime | None, date_to: datetime | None
    ) -> list[dict[str, Any]]:
        slices = await self._transactions.slices(user_id, date_from, date_to)
        lookup = await self._categories.lookup(user_id)

        groups = _group_by_category(slices, lambda item: item.type is not TransactionType.TRANSFER)

        result = []
        for group in groups:
            info = describe(lookup, group.category_id)
            category_type = info.type
            # An uncategorized bucket holding any income reads as income.
            if group.category_id is None and group.has_income:
                category_type = CategoryType.INCOME
            result.append(
                {
                    "categoryId": group.category_id,
                    "categoryName": info.name,
                    "type": category_type,
                    "color": info.color,
                    "amount": group.total,
                }
            )

        result.sort(key=_amount_of, reverse=True)
        return result

    async def _opening_balance(self, user_id: uuid.UUID) -> Decimal:
        accounts = await self._accounts.list_all(user_id)
        return sum((account.initial_balance for account in accounts), ZERO)


def utc_now() -> datetime:
    return datetime.now(UTC)
