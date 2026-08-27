"""Accounts. Archiving is an update, never a delete -- history must survive."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import asyncpg

from app.domain.enums import AccountType

_COLUMNS = '"Id", "UserId", "Name", "Type", "InitialBalance", "Currency", "IsArchived", "CreatedAt"'


@dataclass(slots=True)
class Account:
    id: uuid.UUID
    user_id: uuid.UUID
    name: str
    type: AccountType
    initial_balance: Decimal
    currency: str
    is_archived: bool
    created_at: datetime


def _row(record: asyncpg.Record) -> Account:
    return Account(
        id=record["Id"],
        user_id=record["UserId"],
        name=record["Name"],
        type=AccountType(record["Type"]),
        initial_balance=record["InitialBalance"],
        currency=record["Currency"],
        is_archived=record["IsArchived"],
        created_at=record["CreatedAt"],
    )


class AccountRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def list_all(self, user_id: uuid.UUID) -> list[Account]:
        """Active accounts first, then by name -- the order the UI expects."""
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Accounts" WHERE "UserId" = $1 ORDER BY "IsArchived", "Name"',
            user_id,
        )
        return [_row(record) for record in records]

    async def get(self, user_id: uuid.UUID, account_id: uuid.UUID) -> Account | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2',
            account_id,
            user_id,
        )
        return _row(record) if record else None

    async def exists(self, user_id: uuid.UUID, account_id: uuid.UUID) -> bool:
        return bool(
            await self._conn.fetchval(
                'SELECT 1 FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2',
                account_id,
                user_id,
            )
        )

    async def add(self, account: Account) -> None:
        await self._conn.execute(
            'INSERT INTO "Accounts" ("Id", "UserId", "Name", "Type", "InitialBalance", '
            '"Currency", "IsArchived", "CreatedAt") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)',
            account.id,
            account.user_id,
            account.name,
            int(account.type),
            account.initial_balance,
            account.currency,
            account.is_archived,
            account.created_at,
        )

    async def update(self, account: Account) -> None:
        await self._conn.execute(
            'UPDATE "Accounts" SET "Name" = $3, "Type" = $4, "InitialBalance" = $5, '
            '"Currency" = $6, "IsArchived" = $7 WHERE "Id" = $1 AND "UserId" = $2',
            account.id,
            account.user_id,
            account.name,
            int(account.type),
            account.initial_balance,
            account.currency,
            account.is_archived,
        )

    async def archive(self, user_id: uuid.UUID, account_id: uuid.UUID) -> bool:
        status = await self._conn.execute(
            'UPDATE "Accounts" SET "IsArchived" = true WHERE "Id" = $1 AND "UserId" = $2',
            account_id,
            user_id,
        )
        return status != "UPDATE 0"
