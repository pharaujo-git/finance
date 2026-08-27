"""Integration harness: a throwaway Postgres schema per test session.

Mirrors backend-go/internal/pgtest. The schema is built from the real baseline
migration in db/migrations, so the tests run against the shape that actually
ships rather than one this file invents. Without TEST_DATABASE_URL every test
that needs Postgres skips, and `pytest` stays green with no Docker.
"""

from __future__ import annotations

import os
import re
import uuid
from collections.abc import AsyncIterator
from pathlib import Path

import asyncpg
import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient

from app.core.config import Settings
from app.db import normalize_dsn
from app.main import create_app

# Deliberately not DATABASE_URL: pointing the suite at a developer's real
# database by accident would be expensive.
DATABASE_URL_VARIABLE = "TEST_DATABASE_URL"

MIGRATIONS = Path(__file__).resolve().parents[2] / "db" / "migrations"

DEMO_EMAIL = "harness@test.dev"
DEMO_PASSWORD = "Passw0rd!123"


def _migration_up(path: Path) -> str:
    """The `-- migrate:up` half of a dbmate migration."""
    text = path.read_text()
    body = re.split(r"^--\s*migrate:down\s*$", text, flags=re.MULTILINE)[0]
    return re.sub(r"^--\s*migrate:up\s*$", "", body, flags=re.MULTILINE)


@pytest.fixture(scope="session")
def database_url() -> str:
    url = (os.environ.get(DATABASE_URL_VARIABLE) or "").strip()
    if not url:
        pytest.skip(f"{DATABASE_URL_VARIABLE} is not set; skipping the Postgres tests")
    return normalize_dsn(url)


@pytest_asyncio.fixture
async def pool(database_url: str) -> AsyncIterator[asyncpg.Pool]:
    """A pool pinned to a schema of its own, dropped when the test ends."""
    schema = "pytest_" + uuid.uuid4().hex[:20]

    admin = await asyncpg.connect(database_url)
    try:
        await admin.execute(f'CREATE SCHEMA "{schema}"')
    finally:
        await admin.close()

    created = await asyncpg.create_pool(
        database_url,
        min_size=1,
        max_size=5,
        server_settings={"search_path": schema},
    )
    assert created is not None

    async with created.acquire() as conn:
        for name in ("0001_baseline.sql", "0002_seed_default_categories.sql"):
            await conn.execute(_migration_up(MIGRATIONS / name))

    try:
        yield created
    finally:
        await created.close()
        cleanup = await asyncpg.connect(database_url)
        try:
            await cleanup.execute(f'DROP SCHEMA "{schema}" CASCADE')
        finally:
            await cleanup.close()


@pytest_asyncio.fixture
async def client(pool: asyncpg.Pool) -> AsyncIterator[AsyncClient]:
    """The API, driven in-process against the throwaway schema."""
    settings = Settings(
        database_url="unused: the pool is supplied",
        jwt_secret="finance-tracker-local-development-signing-key-please-override",
        port=8082,
        allowed_origins=["http://localhost:5173"],
    )
    app = create_app(settings, pool=pool)

    # The app's lifespan has to run for the token service to exist.
    async with (
        AsyncClient(transport=ASGITransport(app=app), base_url="http://testserver") as http,
        app.router.lifespan_context(app),
    ):
        yield http


@pytest_asyncio.fixture
async def token(client: AsyncClient) -> str:
    """A registered user's bearer token."""
    response = await client.post(
        "/api/auth/register",
        json={"email": DEMO_EMAIL, "password": DEMO_PASSWORD, "name": "Harness"},
    )
    assert response.status_code == 200, response.text
    return str(response.json()["token"])


@pytest_asyncio.fixture
async def auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


@pytest_asyncio.fixture
async def account(client: AsyncClient, auth: dict[str, str]) -> dict:
    response = await client.post(
        "/api/accounts",
        headers=auth,
        json={"name": "Checking", "type": "checking", "initialBalance": 1000, "currency": "USD"},
    )
    assert response.status_code == 200, response.text
    return dict(response.json())


@pytest_asyncio.fixture
async def category(client: AsyncClient, auth: dict[str, str]) -> dict:
    response = await client.post(
        "/api/categories",
        headers=auth,
        json={"name": "Food", "type": "expense", "icon": "utensils", "color": "#ef4444"},
    )
    assert response.status_code == 200, response.text
    return dict(response.json())
