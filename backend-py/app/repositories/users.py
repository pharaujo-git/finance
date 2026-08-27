"""Users. The email unique index is what actually decides registration races."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime

import asyncpg

_COLUMNS = '"Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt"'


@dataclass(slots=True)
class User:
    id: uuid.UUID
    email: str
    name: str
    password_hash: str
    currency: str
    created_at: datetime


class EmailTakenError(Exception):
    """The unique index rejected the insert."""


def _row(record: asyncpg.Record | None) -> User | None:
    if record is None:
        return None
    return User(
        id=record["Id"],
        email=record["Email"],
        name=record["Name"],
        password_hash=record["PasswordHash"],
        currency=record["Currency"],
        created_at=record["CreatedAt"],
    )


class UserRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def find_by_email(self, email: str) -> User | None:
        return _row(
            await self._conn.fetchrow(f'SELECT {_COLUMNS} FROM "Users" WHERE "Email" = $1', email)
        )

    async def find_by_id(self, user_id: uuid.UUID) -> User | None:
        return _row(
            await self._conn.fetchrow(f'SELECT {_COLUMNS} FROM "Users" WHERE "Id" = $1', user_id)
        )

    async def add(self, user: User) -> None:
        try:
            await self._conn.execute(
                'INSERT INTO "Users" ("Id", "Email", "Name", "PasswordHash", "Currency", '
                '"CreatedAt") VALUES ($1, $2, $3, $4, $5, $6)',
                user.id,
                user.email,
                user.name,
                user.password_hash,
                user.currency,
                user.created_at,
            )
        except asyncpg.UniqueViolationError as exc:
            raise EmailTakenError(user.email) from exc

    async def update_password_hash(self, user_id: uuid.UUID, password_hash: str) -> None:
        await self._conn.execute(
            'UPDATE "Users" SET "PasswordHash" = $2 WHERE "Id" = $1', user_id, password_hash
        )

    async def update_profile(self, user_id: uuid.UUID, name: str, currency: str) -> bool:
        status = await self._conn.execute(
            'UPDATE "Users" SET "Name" = $2, "Currency" = $3 WHERE "Id" = $1',
            user_id,
            name,
            currency,
        )
        return status != "UPDATE 0"
