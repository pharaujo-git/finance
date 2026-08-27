"""Savings goals."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import asyncpg

_COLUMNS = '"Id", "UserId", "Name", "TargetAmount", "CurrentAmount", "TargetDate", "Color"'


@dataclass(slots=True)
class Goal:
    id: uuid.UUID
    user_id: uuid.UUID
    name: str
    target_amount: Decimal
    current_amount: Decimal
    target_date: datetime | None
    color: str


def _row(record: asyncpg.Record) -> Goal:
    return Goal(
        id=record["Id"],
        user_id=record["UserId"],
        name=record["Name"],
        target_amount=record["TargetAmount"],
        current_amount=record["CurrentAmount"],
        target_date=record["TargetDate"],
        color=record["Color"],
    )


class GoalRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def list_all(self, user_id: uuid.UUID) -> list[Goal]:
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Goals" WHERE "UserId" = $1 ORDER BY "Name"', user_id
        )
        return [_row(record) for record in records]

    async def get(self, user_id: uuid.UUID, goal_id: uuid.UUID) -> Goal | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2',
            goal_id,
            user_id,
        )
        return _row(record) if record else None

    async def add(self, goal: Goal) -> None:
        await self._conn.execute(
            'INSERT INTO "Goals" ("Id", "UserId", "Name", "TargetAmount", "CurrentAmount", '
            '"TargetDate", "Color") VALUES ($1, $2, $3, $4, $5, $6, $7)',
            goal.id,
            goal.user_id,
            goal.name,
            goal.target_amount,
            goal.current_amount,
            goal.target_date,
            goal.color,
        )

    async def update(self, goal: Goal) -> None:
        await self._conn.execute(
            'UPDATE "Goals" SET "Name" = $3, "TargetAmount" = $4, "CurrentAmount" = $5, '
            '"TargetDate" = $6, "Color" = $7 WHERE "Id" = $1 AND "UserId" = $2',
            goal.id,
            goal.user_id,
            goal.name,
            goal.target_amount,
            goal.current_amount,
            goal.target_date,
            goal.color,
        )

    async def delete(self, user_id: uuid.UUID, goal_id: uuid.UUID) -> bool:
        status = await self._conn.execute(
            'DELETE FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2', goal_id, user_id
        )
        return status != "DELETE 0"
