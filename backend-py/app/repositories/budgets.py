"""Budgets. One row per category and month; the pair is unique per user."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from decimal import Decimal

import asyncpg

_COLUMNS = '"Id", "UserId", "CategoryId", "Month", "Limit"'


@dataclass(slots=True)
class Budget:
    id: uuid.UUID
    user_id: uuid.UUID
    category_id: uuid.UUID
    month: str
    limit: Decimal


def _row(record: asyncpg.Record) -> Budget:
    return Budget(
        id=record["Id"],
        user_id=record["UserId"],
        category_id=record["CategoryId"],
        month=record["Month"],
        limit=record["Limit"],
    )


class BudgetRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def list_for_month(self, user_id: uuid.UUID, month: str) -> list[Budget]:
        """Unordered on purpose -- the service sorts by category id."""
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Budgets" WHERE "UserId" = $1 AND "Month" = $2',
            user_id,
            month,
        )
        return [_row(record) for record in records]

    async def get(self, user_id: uuid.UUID, budget_id: uuid.UUID) -> Budget | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2',
            budget_id,
            user_id,
        )
        return _row(record) if record else None

    async def exists(self, user_id: uuid.UUID, category_id: uuid.UUID, month: str) -> bool:
        return bool(
            await self._conn.fetchval(
                'SELECT 1 FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2 '
                'AND "Month" = $3',
                user_id,
                category_id,
                month,
            )
        )

    async def add(self, budget: Budget) -> None:
        await self._conn.execute(
            'INSERT INTO "Budgets" ("Id", "UserId", "CategoryId", "Month", "Limit") '
            "VALUES ($1, $2, $3, $4, $5)",
            budget.id,
            budget.user_id,
            budget.category_id,
            budget.month,
            budget.limit,
        )

    async def update_limit(self, user_id: uuid.UUID, budget_id: uuid.UUID, limit: Decimal) -> bool:
        status = await self._conn.execute(
            'UPDATE "Budgets" SET "Limit" = $3 WHERE "Id" = $1 AND "UserId" = $2',
            budget_id,
            user_id,
            limit,
        )
        return status != "UPDATE 0"

    async def delete(self, user_id: uuid.UUID, budget_id: uuid.UUID) -> bool:
        status = await self._conn.execute(
            'DELETE FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2', budget_id, user_id
        )
        return status != "DELETE 0"
