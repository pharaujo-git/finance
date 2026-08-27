"""Transactions, plus the lightweight slice projection the analytics run on."""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime
from decimal import Decimal
from typing import Any

import asyncpg

from app.domain.enums import TransactionType
from app.domain.money import join_tags, split_tags

_COLUMNS = (
    '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Date", '
    '"Description", "Notes", "TagsRaw", "TransferAccountId"'
)

# Rows per INSERT when writing imports and materialised recurrences.
_CHUNK = 500


@dataclass(slots=True)
class Transaction:
    id: uuid.UUID
    user_id: uuid.UUID
    account_id: uuid.UUID
    category_id: uuid.UUID | None
    type: TransactionType
    amount: Decimal
    date: datetime
    description: str
    notes: str | None
    tags: list[str] = field(default_factory=list)
    transfer_account_id: uuid.UUID | None = None


@dataclass(slots=True)
class TransactionSlice:
    """Just enough of a transaction to compute a balance or an aggregate."""

    account_id: uuid.UUID
    transfer_account_id: uuid.UUID | None
    category_id: uuid.UUID | None
    type: TransactionType
    amount: Decimal
    date: datetime


def _row(record: asyncpg.Record) -> Transaction:
    return Transaction(
        id=record["Id"],
        user_id=record["UserId"],
        account_id=record["AccountId"],
        category_id=record["CategoryId"],
        type=TransactionType(record["Type"]),
        amount=record["Amount"],
        date=record["Date"],
        description=record["Description"],
        notes=record["Notes"],
        tags=split_tags(record["TagsRaw"]),
        transfer_account_id=record["TransferAccountId"],
    )


@dataclass(slots=True)
class TransactionFilter:
    account_id: uuid.UUID | None = None
    category_id: uuid.UUID | None = None
    type: TransactionType | None = None
    date_from: datetime | None = None
    date_to: datetime | None = None
    search: str = ""
    limit: int = 20
    offset: int = 0


def _predicate(user_id: uuid.UUID, filters: TransactionFilter) -> tuple[str, list[Any]]:
    """Builds the shared WHERE clause; both bounds are inclusive."""
    clauses = ['"UserId" = $1']
    args: list[Any] = [user_id]

    def add(clause: str, value: Any) -> None:
        args.append(value)
        clauses.append(clause.format(n=len(args)))

    if filters.account_id is not None:
        add('("AccountId" = ${n} OR "TransferAccountId" = ${n})', filters.account_id)
    if filters.category_id is not None:
        add('"CategoryId" = ${n}', filters.category_id)
    if filters.type is not None:
        add('"Type" = ${n}', int(filters.type))
    if filters.date_from is not None:
        add('"Date" >= ${n}', filters.date_from)
    if filters.date_to is not None:
        add('"Date" <= ${n}', filters.date_to)
    if filters.search:
        # Lowered on both sides: the search term arrives already lowercased.
        add('LOWER("Description") LIKE ${n}', f"%{filters.search}%")

    return " WHERE " + " AND ".join(clauses), args


class TransactionRepository:
    def __init__(self, conn: asyncpg.Connection) -> None:
        self._conn = conn

    async def search(
        self, user_id: uuid.UUID, filters: TransactionFilter
    ) -> tuple[list[Transaction], int]:
        """One page of matches plus the total number of them."""
        where, args = _predicate(user_id, filters)

        total = await self._conn.fetchval(f'SELECT COUNT(*) FROM "Transactions"{where}', *args)

        paged = [*args, filters.limit, filters.offset]
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Transactions"{where} '
            f'ORDER BY "Date" DESC, "Id" DESC LIMIT ${len(paged) - 1} OFFSET ${len(paged)}',
            *paged,
        )
        return [_row(record) for record in records], int(total or 0)

    async def get(self, user_id: uuid.UUID, transaction_id: uuid.UUID) -> Transaction | None:
        record = await self._conn.fetchrow(
            f'SELECT {_COLUMNS} FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2',
            transaction_id,
            user_id,
        )
        return _row(record) if record else None

    async def list_range(
        self, user_id: uuid.UUID, date_from: datetime | None, date_to: datetime | None
    ) -> list[Transaction]:
        """Newest first -- the order the CSV export writes."""
        where, args = _predicate(user_id, TransactionFilter(date_from=date_from, date_to=date_to))
        records = await self._conn.fetch(
            f'SELECT {_COLUMNS} FROM "Transactions"{where} ORDER BY "Date" DESC, "Id" DESC',
            *args,
        )
        return [_row(record) for record in records]

    async def slices(
        self, user_id: uuid.UUID, date_from: datetime | None, date_to: datetime | None
    ) -> list[TransactionSlice]:
        """Oldest first, so a running total can be accumulated in one pass."""
        where, args = _predicate(user_id, TransactionFilter(date_from=date_from, date_to=date_to))
        records = await self._conn.fetch(
            'SELECT "AccountId", "TransferAccountId", "CategoryId", "Type", "Amount", "Date" '
            f'FROM "Transactions"{where} ORDER BY "Date", "Id"',
            *args,
        )
        return [
            TransactionSlice(
                account_id=record["AccountId"],
                transfer_account_id=record["TransferAccountId"],
                category_id=record["CategoryId"],
                type=TransactionType(record["Type"]),
                amount=record["Amount"],
                date=record["Date"],
            )
            for record in records
        ]

    async def add(self, transaction: Transaction) -> None:
        await self.add_many([transaction])

    async def add_many(self, transactions: list[Transaction]) -> None:
        """Chunked so one oversized import cannot build an unbounded statement."""
        if not transactions:
            return
        for start in range(0, len(transactions), _CHUNK):
            chunk = transactions[start : start + _CHUNK]
            await self._conn.executemany(
                'INSERT INTO "Transactions" ("Id", "UserId", "AccountId", "CategoryId", '
                '"Type", "Amount", "Date", "Description", "Notes", "TagsRaw", '
                '"TransferAccountId") '
                "VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
                [
                    (
                        item.id,
                        item.user_id,
                        item.account_id,
                        item.category_id,
                        int(item.type),
                        item.amount,
                        item.date,
                        item.description,
                        item.notes,
                        join_tags(item.tags),
                        item.transfer_account_id,
                    )
                    for item in chunk
                ],
            )

    async def update(self, transaction: Transaction) -> None:
        await self._conn.execute(
            'UPDATE "Transactions" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5, '
            '"Amount" = $6, "Date" = $7, "Description" = $8, "Notes" = $9, "TagsRaw" = $10, '
            '"TransferAccountId" = $11 WHERE "Id" = $1 AND "UserId" = $2',
            transaction.id,
            transaction.user_id,
            transaction.account_id,
            transaction.category_id,
            int(transaction.type),
            transaction.amount,
            transaction.date,
            transaction.description,
            transaction.notes,
            join_tags(transaction.tags),
            transaction.transfer_account_id,
        )

    async def delete(self, user_id: uuid.UUID, transaction_id: uuid.UUID) -> bool:
        status = await self._conn.execute(
            'DELETE FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2',
            transaction_id,
            user_id,
        )
        return status != "DELETE 0"
