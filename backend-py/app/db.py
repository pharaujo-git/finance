"""Postgres connection pool.

The schema is owned by the backend-neutral dbmate migrations in db/migrations,
so nothing here issues DDL. Table and column names are EF Core's quoted
PascalCase, which is why every query quotes its identifiers.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from decimal import Decimal
from typing import Any

import asyncpg


def normalize_dsn(database_url: str) -> str:
    """asyncpg rejects the postgresql+driver:// and sslmode aliases some hosts emit."""
    dsn = database_url.strip()
    if dsn.startswith("postgres://"):
        dsn = "postgresql://" + dsn[len("postgres://") :]
    return dsn


async def _init_connection(conn: asyncpg.Connection) -> None:
    """numeric comes back as Decimal by default; make that explicit and keep
    JSON-facing money exact rather than routed through float."""
    await conn.set_type_codec(
        "numeric",
        encoder=str,
        decoder=Decimal,
        schema="pg_catalog",
        format="text",
    )


async def create_pool(database_url: str, **kwargs: Any) -> asyncpg.Pool:
    """Opens the pool used for the lifetime of the process."""
    pool = await asyncpg.create_pool(
        normalize_dsn(database_url),
        min_size=1,
        max_size=10,
        init=_init_connection,
        **kwargs,
    )
    if pool is None:  # pragma: no cover - asyncpg only returns None on misuse
        raise RuntimeError("persistence: could not open the connection pool")
    return pool


@asynccontextmanager
async def acquire(pool: asyncpg.Pool) -> AsyncIterator[asyncpg.Connection]:
    """Borrows a connection for one unit of work."""
    async with pool.acquire() as connection:
        yield connection
