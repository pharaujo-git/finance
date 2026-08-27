"""Recurring rules. ListDue crosses users: the worker runs one pass for everyone."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import asyncpg

from app.domain.enums import Frequency, TransactionType

_COLUMNS = (
    '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Description", '
    '"Frequency", "StartDate", "EndDate", "NextRunDate", "IsActive"'
)


@dataclass(slots=True)
class RecurringRule:
    id: uuid.UUID
    user_id: uuid.UUID
    account_id: uuid.UUID
    category_id: uuid.UUID | None
    type: TransactionType
    amount: Decimal
    description: str
    frequency: Frequency
    start_date: datetime
    end_date: datetime | None
    next_run_date: datetime
    is_active: bool


def _row(record: asyncpg.Record) -> RecurringRule:
    return RecurringRule(
        id=record["Id"],
        user_id=record["UserId"],
        account_id=record["AccountId"],
        category_id=record["CategoryId"],
        type=TransactionType(record["Type"]),
        amount=record["Amount"],
        description=record["Description"],
        frequency=Frequency(record["Frequency"]),
        start_date=record["StartDate"],
        end_date=record["EndDate"],
        next_run_date=record["NextRunDate"],
        is_active=record["IsActive"],
    )


class RecurringRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def list_all(self, user_id: uuid.UUID) -> list[RecurringRule]:
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "RecurringRules" WHERE "UserId" = $1 ORDER BY "NextRunDate"',
            user_id,
        )
        return [_row(record) for record in records]

    async def get(self, user_id: uuid.UUID, rule_id: uuid.UUID) -> RecurringRule | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2',
            rule_id,
            user_id,
        )
        return _row(record) if record else None

    async def list_due(self, cutoff: datetime) -> list[RecurringRule]:
        """Every active rule due at or before the cutoff, across all users."""
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "RecurringRules" '
            'WHERE "IsActive" = true AND "NextRunDate" <= $1 ORDER BY "NextRunDate", "Id"',
            cutoff,
        )
        return [_row(record) for record in records]

    async def add(self, rule: RecurringRule) -> None:
        await self._conn.execute(
            'INSERT INTO "RecurringRules" ("Id", "UserId", "AccountId", "CategoryId", '
            '"Type", "Amount", "Description", "Frequency", "StartDate", "EndDate", '
            '"NextRunDate", "IsActive") '
            "VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
            rule.id,
            rule.user_id,
            rule.account_id,
            rule.category_id,
            int(rule.type),
            rule.amount,
            rule.description,
            int(rule.frequency),
            rule.start_date,
            rule.end_date,
            rule.next_run_date,
            rule.is_active,
        )

    async def update(self, rule: RecurringRule) -> None:
        await self._conn.execute(
            'UPDATE "RecurringRules" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5, '
            '"Amount" = $6, "Description" = $7, "Frequency" = $8, "StartDate" = $9, '
            '"EndDate" = $10, "NextRunDate" = $11, "IsActive" = $12 '
            'WHERE "Id" = $1 AND "UserId" = $2',
            rule.id,
            rule.user_id,
            rule.account_id,
            rule.category_id,
            int(rule.type),
            rule.amount,
            rule.description,
            int(rule.frequency),
            rule.start_date,
            rule.end_date,
            rule.next_run_date,
            rule.is_active,
        )

    async def delete(self, user_id: uuid.UUID, rule_id: uuid.UUID) -> bool:
        status = await self._conn.execute(
            'DELETE FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2', rule_id, user_id
        )
        return status != "DELETE 0"
