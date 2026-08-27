"""Categories. A row with a null UserId is a shared default every user sees."""

from __future__ import annotations

import uuid
from dataclasses import dataclass

import asyncpg

from app.domain.enums import CategoryType

_COLUMNS = '"Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault"'

UNCATEGORIZED_NAME = "Uncategorized"
UNCATEGORIZED_COLOR = "#94a3b8"


@dataclass(slots=True)
class Category:
    id: uuid.UUID
    user_id: uuid.UUID | None
    name: str
    type: CategoryType
    icon: str
    color: str
    is_default: bool


def _row(record: asyncpg.Record) -> Category:
    return Category(
        id=record["Id"],
        user_id=record["UserId"],
        name=record["Name"],
        type=CategoryType(record["Type"]),
        icon=record["Icon"],
        color=record["Color"],
        is_default=record["IsDefault"],
    )


class CategoryRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def list_visible(self, user_id: uuid.UUID) -> list[Category]:
        """The shared defaults plus the caller's own."""
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Categories" '
            'WHERE "IsDefault" = true OR "UserId" = $1 ORDER BY "Type", "Name"',
            user_id,
        )
        return [_row(record) for record in records]

    async def get(self, user_id: uuid.UUID, category_id: uuid.UUID) -> Category | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Categories" '
            'WHERE "Id" = $1 AND ("IsDefault" = true OR "UserId" = $2)',
            category_id,
            user_id,
        )
        return _row(record) if record else None

    async def get_owned(self, user_id: uuid.UUID, category_id: uuid.UUID) -> Category | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2',
            category_id,
            user_id,
        )
        return _row(record) if record else None

    async def add(self, category: Category) -> None:
        await self._conn.execute(
            'INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", '
            '"IsDefault") VALUES ($1, $2, $3, $4, $5, $6, $7)',
            category.id,
            category.user_id,
            category.name,
            int(category.type),
            category.icon,
            category.color,
            category.is_default,
        )

    async def update(self, category: Category) -> None:
        await self._conn.execute(
            'UPDATE "Categories" SET "Name" = $3, "Type" = $4, "Icon" = $5, "Color" = $6 '
            'WHERE "Id" = $1 AND "UserId" = $2',
            category.id,
            category.user_id,
            category.name,
            int(category.type),
            category.icon,
            category.color,
        )

    async def delete(self, user_id: uuid.UUID, category_id: uuid.UUID) -> None:
        """Detaches the category from its transactions, then drops its budgets.

        All three writes share one transaction: a half-applied delete would
        leave transactions pointing at a row that no longer exists.
        """
        async with self._conn.transaction():
            await self._conn.execute(
                'UPDATE "Transactions" SET "CategoryId" = NULL '
                'WHERE "UserId" = $1 AND "CategoryId" = $2',
                user_id,
                category_id,
            )
            await self._conn.execute(
                'DELETE FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2',
                user_id,
                category_id,
            )
            await self._conn.execute(
                'DELETE FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2',
                category_id,
                user_id,
            )
