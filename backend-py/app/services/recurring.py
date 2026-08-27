"""Recurring rules and the pass that turns them into transactions."""

from __future__ import annotations

import uuid
from datetime import datetime, timedelta
from typing import Any

from app.api.schemas import RecurringRequest
from app.core.errors import not_found, validation
from app.domain.dates import add_months, add_years
from app.domain.enums import Frequency, TransactionType
from app.domain.money import as_utc, round_money
from app.repositories.accounts import AccountRepository
from app.repositories.recurring import RecurringRepository, RecurringRule
from app.repositories.transactions import Transaction, TransactionRepository
from app.services.accounts import ACCOUNT_ENTITY
from app.services.categories import CategoryService

RECURRING_ENTITY = "Recurring rule"
RECURRING_TRANSFER_MESSAGE = "Recurring transfers are not supported."
RECURRING_END_DATE_MESSAGE = "End date must not be before the start date."

# A cap so one long-dormant rule cannot generate an unbounded batch in a single
# pass; the rule stays active and the next pass carries on.
MAX_OCCURRENCES_PER_PASS = 500
RECURRING_TAG = "recurring"


def recurring_dto(rule: RecurringRule) -> dict[str, Any]:
    return {
        "id": rule.id,
        "accountId": rule.account_id,
        "categoryId": rule.category_id,
        "type": rule.type,
        "amount": rule.amount,
        "description": rule.description,
        "frequency": rule.frequency,
        "startDate": rule.start_date,
        "endDate": rule.end_date,
        "nextRunDate": rule.next_run_date,
        "isActive": rule.is_active,
    }


def advance(moment: datetime, frequency: Frequency) -> datetime:
    """The next occurrence after `moment`. Clock time is preserved throughout."""
    if frequency is Frequency.DAILY:
        return moment + timedelta(days=1)
    if frequency is Frequency.WEEKLY:
        return moment + timedelta(days=7)
    if frequency is Frequency.YEARLY:
        return add_years(moment, 1)
    # Monthly, and anything undefined, advance by a clamped month.
    return add_months(moment, 1)


def materialize(rule: RecurringRule, cutoff: datetime) -> list[Transaction]:
    """Emits the occurrences due at or before the cutoff, mutating the rule."""
    created: list[Transaction] = []

    for _ in range(MAX_OCCURRENCES_PER_PASS):
        if not rule.is_active or rule.next_run_date > cutoff:
            break
        if rule.end_date is not None and rule.next_run_date > rule.end_date:
            # An occurrence exactly *on* the end date is still created; only a
            # run past it retires the rule.
            rule.is_active = False
            break

        created.append(
            Transaction(
                id=uuid.uuid4(),
                user_id=rule.user_id,
                account_id=rule.account_id,
                category_id=rule.category_id,
                type=rule.type,
                amount=rule.amount,
                date=rule.next_run_date,
                description=rule.description,
                notes=None,
                tags=[RECURRING_TAG],
                transfer_account_id=None,
            )
        )
        rule.next_run_date = advance(rule.next_run_date, rule.frequency)

    return created


class RecurringService:
    def __init__(
        self,
        rules: RecurringRepository,
        transactions: TransactionRepository,
        accounts: AccountRepository,
        categories: CategoryService,
    ) -> None:
        self._rules = rules
        self._transactions = transactions
        self._accounts = accounts
        self._categories = categories

    async def list_all(self, user_id: uuid.UUID) -> list[dict[str, Any]]:
        return [recurring_dto(rule) for rule in await self._rules.list_all(user_id)]

    async def create(self, user_id: uuid.UUID, request: RecurringRequest) -> dict[str, Any]:
        await self._check(user_id, request)
        assert request.start_date is not None

        rule = RecurringRule(
            id=uuid.uuid4(),
            user_id=user_id,
            account_id=uuid.UUID(int=0),
            category_id=None,
            type=TransactionType.EXPENSE,
            amount=round_money(request.amount),  # type: ignore[arg-type]
            description="",
            frequency=Frequency.MONTHLY,
            start_date=as_utc(request.start_date),
            end_date=None,
            next_run_date=as_utc(request.start_date),
            is_active=True,
        )
        _apply(rule, request)
        # The first occurrence is the start date itself.
        rule.next_run_date = rule.start_date

        await self._rules.add(rule)
        return recurring_dto(rule)

    async def update(
        self, user_id: uuid.UUID, rule_id: uuid.UUID, request: RecurringRequest
    ) -> dict[str, Any]:
        await self._check(user_id, request)
        rule = await self._load(user_id, rule_id)
        _apply(rule, request)

        # Pull the next run forward if the start moved later; never push it back.
        if rule.next_run_date < rule.start_date:
            rule.next_run_date = rule.start_date

        await self._rules.update(rule)
        return recurring_dto(rule)

    async def delete(self, user_id: uuid.UUID, rule_id: uuid.UUID) -> None:
        if not await self._rules.delete(user_id, rule_id):
            raise not_found(RECURRING_ENTITY)

    async def materialize_due(self, now: datetime) -> int:
        """Runs one pass. The caller owns the transaction and the lock."""
        cutoff = as_utc(now)
        due = await self._rules.list_due(cutoff)

        created: list[Transaction] = []
        for rule in due:
            created.extend(materialize(rule, cutoff))

        if created:
            await self._transactions.add_many(created)
        for rule in due:
            await self._rules.update(rule)

        return len(created)

    async def _check(self, user_id: uuid.UUID, request: RecurringRequest) -> None:
        """Order fixed: it decides which error a doubly-wrong request gets."""
        if request.type is TransactionType.TRANSFER:
            raise validation(RECURRING_TRANSFER_MESSAGE)

        assert request.account_id is not None
        if not await self._accounts.exists(user_id, request.account_id):
            raise not_found(ACCOUNT_ENTITY)

        if (
            request.end_date is not None
            and request.start_date is not None
            and request.end_date < request.start_date
        ):
            raise validation(RECURRING_END_DATE_MESSAGE)

        await self._categories.ensure_usable(user_id, request.category_id)

    async def _load(self, user_id: uuid.UUID, rule_id: uuid.UUID) -> RecurringRule:
        rule = await self._rules.get(user_id, rule_id)
        if rule is None:
            raise not_found(RECURRING_ENTITY)
        return rule


def _apply(rule: RecurringRule, request: RecurringRequest) -> None:
    assert request.account_id is not None
    assert request.type is not None
    assert request.amount is not None
    assert request.frequency is not None
    assert request.start_date is not None

    rule.account_id = request.account_id
    rule.category_id = request.category_id
    rule.type = request.type
    rule.amount = round_money(request.amount)
    rule.description = request.description.strip()
    rule.frequency = request.frequency
    rule.start_date = as_utc(request.start_date)
    rule.end_date = as_utc(request.end_date) if request.end_date else None
    # An omitted flag leaves the rule running.
    rule.is_active = True if request.is_active is None else request.is_active
